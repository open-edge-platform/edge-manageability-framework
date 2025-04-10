// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onboardingmanager

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	infra_api "github.com/open-edge-platform/infra-core/api/pkg/api/v0"
	pb_om "github.com/open-edge-platform/infra-onboarding/onboarding-manager/pkg/api/onboardingmgr/v1"
	tinkv1alpha1 "github.com/tinkerbell/tink/api/v1alpha1"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	k8s_rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MyOSID struct {
	OsID string `json:"osResourceID,omitempty"`
}

type ProviderConfig struct {
	DefaultOs     string `json:"defaultOs"`
	AutoProvision bool   `json:"autoProvision"`
}

func GrpcInfraOnboardNewNode(host, token, mac, uuid string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		host,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(
			credentials.NewClientTLSFromCert(nil, ""),
		),
		grpc.WithPerRPCCredentials(
			oauth.TokenSource{
				TokenSource: oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: token},
				),
			},
		),
	)
	if err != nil {
		return fmt.Errorf("could not dial server %s: %w", host, err)
	}
	defer conn.Close()

	client := pb_om.NewInteractiveOnboardingServiceClient(conn)
	nodeData := &pb_om.NodeData{
		Hwdata: []*pb_om.HwData{
			{
				MacId:     mac,
				SutIp:     "192.168.10.1",
				Uuid:      uuid,
				Serialnum: "98333",
			},
		},
	}

	nodeRequest := &pb_om.CreateNodesRequest{
		Payload: []*pb_om.NodeData{nodeData},
	}

	_, err = client.CreateNodes(ctx, nodeRequest)
	if err != nil {
		return fmt.Errorf("could not CreateNodes to server %s: %w", host, err)
	}
	return nil
}

func GrpcInfraOnboardStreamNode(host, mac, uuid, token string) (pb_om.OnboardNodeStreamResponse_NodeState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var opts []grpc.DialOption

	// Add transport credentials
	opts = append(opts, grpc.WithTransportCredentials(
		credentials.NewClientTLSFromCert(nil, ""), // Use host's root CA set to establish trust
	))

	// Conditionally add per-RPC credentials if token is provided
	if token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(
			oauth.TokenSource{
				TokenSource: oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: token}, // Send the access token as part of the HTTP Authorization header
				),
			},
		))
	}

	conn, err := grpc.DialContext(
		ctx,
		host,
		opts...,
	)
	if err != nil {
		return pb_om.OnboardNodeStreamResponse_NODE_STATE_UNSPECIFIED, fmt.Errorf("could not dial server %s: %w", host, err)
	}
	defer conn.Close()

	client := pb_om.NewNonInteractiveOnboardingServiceClient(conn)

	// Establish a stream with the server
	stream, err := client.OnboardNodeStream(ctx)
	if err != nil {
		return pb_om.OnboardNodeStreamResponse_NODE_STATE_UNSPECIFIED, fmt.Errorf("could not create stream: %w", err)
	}
	defer func() {
		if err := stream.CloseSend(); err != nil {
			fmt.Printf("error closing stream: %v\n", err)
		}
	}()

	// Construct the OnboardStreamRequest message
	request := &pb_om.OnboardNodeStreamRequest{
		Uuid:      uuid,
		MacId:     mac,
		Serialnum: "SN123456783",  // TODO: Replace with the actual serial number
		HostIp:    "192.168.10.1", // TODO: Replace with the actual host IP
	}

	// Send the request over the stream
	if err := stream.Send(request); err != nil {
		return pb_om.OnboardNodeStreamResponse_NODE_STATE_UNSPECIFIED, fmt.Errorf("could not send data to server: %w", err)
	}

	// Receive response over the stream
	resp, err := stream.Recv()
	if err != nil {
		return pb_om.OnboardNodeStreamResponse_NODE_STATE_UNSPECIFIED, fmt.Errorf("error receiving response from server: %w", err)
	}

	// Handle the response based on the status
	switch resp.GetNodeState() {
	case pb_om.OnboardNodeStreamResponse_NODE_STATE_REGISTERED:
		fmt.Printf("Node successfully registered\n")
		return pb_om.OnboardNodeStreamResponse_NODE_STATE_REGISTERED, nil
	case pb_om.OnboardNodeStreamResponse_NODE_STATE_ONBOARDED:
		fmt.Printf("Node successfully onboarded and received Client Id: %v Client Secret: %v from OM\n",
			resp.GetClientId(), resp.GetClientSecret())
		return pb_om.OnboardNodeStreamResponse_NODE_STATE_ONBOARDED, nil
	case pb_om.OnboardNodeStreamResponse_NODE_STATE_UNSPECIFIED:
		return pb_om.OnboardNodeStreamResponse_NODE_STATE_UNSPECIFIED, fmt.Errorf("onboarding failed")
	default:
		return pb_om.OnboardNodeStreamResponse_NODE_STATE_UNSPECIFIED, fmt.Errorf("unknown status")
	}
}

func HttpInfraOnboardNewRegisterHost(
	url, token string,
	client *http.Client,
	hostUuid uuid.UUID,
	autoOnboard bool,
) (*infra_api.Host, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	name := "host-test"

	hostRegisterInfo := &infra_api.HostRegisterInfo{
		Name:        &name,
		Uuid:        &hostUuid,
		AutoOnboard: &autoOnboard,
	}

	data, err := json.Marshal(hostRegisterInfo)
	if err != nil {
		return nil, err
	}

	fmt.Printf("HostRegisterInfo %s\n", data)

	var registeredHost infra_api.Host

	responseHooker := func(res *http.Response) error {
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}

		err = json.Unmarshal(b, &registeredHost)
		if err != nil {
			return err
		}
		return nil
	}

	fmt.Printf("Sending POST request to %s with token %s\n", url, token)
	if err := httpPost(ctx, client, url, token, data, responseHooker); err != nil {
		fmt.Printf("HTTP POST request failed: %v\n", err)
		return nil, err
	}

	return &registeredHost, nil
}

func HttpInfraOnboardGetHostID(ctx context.Context, url string, token string, client *http.Client, uuid string) (string, error) {
	rCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	hostID := ""
	responseHooker := func(res *http.Response) error {
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		ps := &infra_api.HostsList{}
		err = json.Unmarshal(b, &ps)
		if err != nil {
			return err
		}
		if ps.Hosts == nil || len(*ps.Hosts) == 0 {
			return fmt.Errorf("empty host result for uuid %s", uuid)
		}
		fmt.Printf("HostResource %#v\n", ps)
		hostID = *(*ps.Hosts)[0].ResourceId
		return nil
	}
	if err := httpGet(rCtx, client, fmt.Sprintf("%s?uuid=%s", url, uuid), token, responseHooker); err != nil {
		return hostID, err
	}

	return hostID, nil
}

func HttpInfraOnboardGetHostIDAndInstanceID(ctx context.Context, url string, token string, client *http.Client, uuid string) (string, string, error) {
	rCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	hostID := ""
	instanceID := ""
	responseHooker := func(res *http.Response) error {
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		ps := &infra_api.HostsList{}
		err = json.Unmarshal(b, &ps)
		if err != nil {
			return err
		}
		if ps.Hosts == nil || len(*ps.Hosts) == 0 {
			return fmt.Errorf("empty host result for uuid %s", uuid)
		}
		fmt.Printf("HostResource %#v\n", ps)
		host := (*ps.Hosts)[0]
		hostID = *host.ResourceId
		if host.Instance != nil && host.Instance.InstanceID != nil {
			instanceID = *host.Instance.InstanceID
		} else {
			return fmt.Errorf("instance not yet created for uuid %s", uuid)
		}
		return nil
	}

	if err := httpGet(rCtx, client, fmt.Sprintf("%s?uuid=%s", url, uuid), token, responseHooker); err != nil {
		return hostID, instanceID, err
	}

	return hostID, instanceID, nil
}

func HttpInfraOnboardDelResource(ctx context.Context, url string, token string, client *http.Client, resourceID string) error {
	rCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	fmt.Printf("Delete resource %s\n", resourceID)
	if err := httpDelete(rCtx, client, url, token, resourceID, nil); err != nil {
		return err
	}

	return nil
}

func HttpInfraOnboardNewInstance(instanceUrl, token, hostID, osID string, client *http.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	instKind := infra_api.INSTANCEKINDMETAL
	instanceName := "test-instance"
	instance := infra_api.Instance{
		HostID: &hostID,
		OsID:   &osID,
		Kind:   &instKind,
		Name:   &instanceName,
	}

	data, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("failed to marshal instance data: %w", err)
	}

	responseHooker := func(res *http.Response) error {
		if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to create instance, status: %s", res.Status)
		}
		return nil
	}

	if err := httpPost(ctx, client, instanceUrl, token, data, responseHooker); err != nil {
		return fmt.Errorf("HTTP POST request failed: %w", err)
	}

	return nil
}

func HttpInfraOnboardGetOSID(ctx context.Context, url string, token string, client *http.Client) (string, error) {
	rCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	osID := ""
	responseHooker := func(res *http.Response) error {
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		os := &infra_api.OperatingSystemResourceList{}
		err = json.Unmarshal(b, &os)
		if err != nil {
			return err
		}
		if os.OperatingSystemResources == nil || len(*os.OperatingSystemResources) == 0 {
			return fmt.Errorf("empty os resources")
		}
		for _, osr := range *os.OperatingSystemResources {
			if *osr.ProfileName == "ubuntu-22.04-lts-generic" {
				osID = *osr.ResourceId
				fmt.Printf("Found OS: %s\n", osID)
				break
			}
		}
		if osID == "" {
			return fmt.Errorf("ubuntu-22.04-lts-generic profile not found")
		}
		return nil
	}
	if err := httpGet(rCtx, client, url, token, responseHooker); err != nil {
		return osID, err
	}

	return osID, nil
}

func CheckWorkflowCreationInfraOnboard(ns string, uuid string, cli client.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	pollTimeDuration := 30 * time.Second

	check := func() (bool, error) {
		wfList := &tinkv1alpha1.WorkflowList{}
		if err := cli.List(ctx, wfList, &client.ListOptions{}); err != nil {
			return false, err
		}
		for _, wf := range wfList.Items {
			fmt.Printf("wf %s %s\n", wf.Namespace, wf.Name)
			if ns == wf.Namespace && strings.Contains(wf.Name, uuid) {
				return true, nil
			}
		}
		return false, nil
	}

	if err := wait.PollUntilContextTimeout(ctx, pollTimeDuration, time.Hour, false, func(_ context.Context) (bool, error) {
		success, statusErr := check()
		if statusErr != nil {
			return false, statusErr
		}
		fmt.Printf("Workflow check: %v %v\n", success, statusErr)
		return success, nil
	}); err != nil {
		return fmt.Errorf("workflow not found: %w", err)
	}

	return nil
}

func NewK8SClient() (client.Client, error) {
	var (
		config *k8s_rest.Config
		err    error
	)
	config, err = k8s_rest.InClusterConfig()
	if err != nil {
		if !errors.Is(err, k8s_rest.ErrNotInCluster) {
			return nil, err
		}
		config, err = outClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	if schemeErr := tinkv1alpha1.AddToScheme(scheme.Scheme); schemeErr != nil {
		return nil, schemeErr
	}

	kubeClient, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
}

func outClusterConfig() (*k8s_rest.Config, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	conf, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

// TODO: use api client
func httpGet(ctx context.Context, client *http.Client, url, token string, responseHooker func(*http.Response) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP GET to %s failed, status: %s", url, resp.Status)
		return err
	}

	if responseHooker != nil {
		if err := responseHooker(resp); err != nil {
			return err
		}
	}

	return nil
}

func httpPost(ctx context.Context, client *http.Client, url, token string, data []byte, responseHooker func(*http.Response) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP POST to %s failed, status: %s", url, resp.Status)
		return err
	}

	if responseHooker != nil {
		if err := responseHooker(resp); err != nil {
			return err
		}
	}

	return nil
}

// TODO: use api client
func httpDelete(ctx context.Context, client *http.Client, url, token string, resourceID string, responseHooker func(*http.Response) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/%s", url, resourceID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP Delete to %s failed, status: %s", fmt.Sprintf("%s/%s", url, resourceID), resp.Status)
		return err
	}

	if responseHooker != nil {
		if err := responseHooker(resp); err != nil {
			return err
		}
	}

	return nil
}

const (
	local     = 0b10
	multicast = 0b1
)

func Mac() string {
	buf := make([]byte, 6)
	_, err := rand.Read(buf)
	if err != nil {
		return "ab:cd:ef:12:34:56"
	}

	buf[0] = buf[0]&^multicast | local
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
}
