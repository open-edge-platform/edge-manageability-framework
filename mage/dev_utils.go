// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/bitfield/script"
	"github.com/google/uuid"
	"github.com/open-edge-platform/edge-manageability-framework/internal/retry"

	onboarding_manager "github.com/open-edge-platform/edge-manageability-framework/e2e-tests/orchestrator/onboarding_manager"
)

// Deploys the ENiC (indicates the number of instances, optionally set env variables: ORCH_FQDN, ORCH_IP, ORCH_USER, ORCH_PASS, ORCH_ORG, ORCH_PROJECT).
func (DevUtils) DeployEnic(replicas int, targetEnv string) error {
	deployRevision := giteaDeployRevisionParam()
	namespace := "utils"
	orchestratorIp, err := getPrimaryIP()
	if err != nil {
		return err
	}

	orchFQDN := serviceDomain
	if orchFQDNEnv := os.Getenv("ORCH_FQDN"); orchFQDNEnv != "" {
		orchFQDN = orchFQDNEnv
	}

	orchIP := orchestratorIp.String()
	if orchIPEnv := os.Getenv("ORCH_IP"); orchIPEnv != "" {
		orchIP = orchIPEnv
	}

	orchPass, err := GetDefaultOrchPassword()
	if err != nil {
		return err
	}
	if orchPassEnv := os.Getenv("ORCH_PASS"); orchPassEnv != "" {
		orchPass = orchPassEnv
	}

	orchOrg := defaultOrg
	if orchOrgEnv := os.Getenv("ORCH_ORG"); orchOrgEnv != "" {
		orchOrg = orchOrgEnv
	}

	orchProject := defaultProject
	if orchProjectEnv := os.Getenv("ORCH_PROJECT"); orchProjectEnv != "" {
		orchProject = orchProjectEnv
	}

	orchUser := fmt.Sprintf("%s-%s", orchProject, "onboarding-user")
	if orchUserEnv := os.Getenv("ORCH_USER"); orchUserEnv != "" {
		orchUser = orchUserEnv
	}

	targetConfig := getTargetConfig(targetEnv)

	cmd := fmt.Sprintf("helm upgrade --install root-app argocd-internal/root-app -f %s -n %s --create-namespace %s "+
		"--set root.useLocalValues=true --set argo.enic.replicas=%d "+
		"--set argo.clusterDomain=%s --set argo.enic.orchestratorIp=%s "+
		"--set argo.enic.orchestratorUser=%s --set argo.enic.orchestratorPass=%s "+
		"--set argo.enic.orchestratorOrg=%s --set argo.enic.orchestratorProject=%s",
		targetConfig, namespace, deployRevision,
		replicas,
		orchFQDN, orchIP,
		orchUser, orchPass,
		orchOrg, orchProject,
	)

	// Pushd to the deployment directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	deploymentDir := filepath.Join(deployGiteaRepoDir, deployRepoName)
	if err := os.Chdir(deploymentDir); err != nil {
		return fmt.Errorf("failed to change directory to %s: %w", deploymentDir, err)
	}
	defer func() {
		if err := os.Chdir(currentDir); err != nil {
			log.Printf("failed to change back to original directory: %v", err)
		}
	}()

	fmt.Printf("exec: %s\n", cmd)
	_, err = script.Exec(cmd).Stdout()
	return err
}

func isEnicArgoAppReady() (bool, error) {
	bytes, err := exec.Command("kubectl", "get", "application", "-n", "utils", "enic", "-o", "json").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("get ENiC argocd application: %w: %s", err, strings.TrimSpace(string(bytes)))
	}

	var app argoCDApp
	if err := json.Unmarshal(bytes, &app); err != nil {
		return false, fmt.Errorf("unmarshal application: %w", err)
	}

	if app.Status.Sync.Status == "Synced" && app.Status.Health.Status == "Healthy" {
		return true, nil
	}
	return false, nil
}

func getEnicUUIDInt(pod string) (uuid.UUID, error) {
	var enicUUID uuid.UUID
	var errUUID error

	cmd := fmt.Sprintf("kubectl exec -it -n %s %s -c edge-node -- bash -c 'dmidecode -s system-uuid'", enicNs, pod)
	ctx, cancel := context.WithTimeout(context.Background(), waitForReadyMin*time.Minute)
	defer cancel()
	fn := func() error {
		out, err := exec.Command("bash", "-c", cmd).Output()
		outParsed := strings.Trim(string(out), "\n")
		enicUUID, errUUID = uuid.Parse(outParsed)
		if err != nil || errUUID != nil {
			return fmt.Errorf("enic UUID is not ready")
		} else {
			return nil
		}
	}

	if err := retry.UntilItSucceeds(ctx, fn, time.Duration(waitForNextSec)*time.Second); err != nil {
		return uuid.UUID{}, fmt.Errorf("enic UUID retrieve error: %w 😲", err)
	}

	return enicUUID, nil
}

func getEnicSNInt(pod string) (string, error) {
	var serialNumbers string

	cmd := fmt.Sprintf("kubectl exec -it -n %s %s -c edge-node -- bash -c 'dmidecode -s system-serial-number'", enicNs, pod)
	ctx, cancel := context.WithTimeout(context.Background(), waitForReadyMin*time.Minute)
	defer cancel()
	fn := func() error {
		out, err := exec.Command("bash", "-c", cmd).Output()
		if err != nil {
			return fmt.Errorf("get ENiC serial number: %w", err)
		}
		serialNumbers = strings.TrimSpace(string(out))
		return nil
	}

	if err := retry.UntilItSucceeds(ctx, fn, time.Duration(waitForNextSec)*time.Second); err != nil {
		return "", fmt.Errorf("failed to get ENiC serial number after multiple attempts: %w", err)
	}

	return serialNumbers, nil
}

// RegisterEnic Registers ENiC with orchestrator, usage: mage devUtils:registerEnic enic-0
func (DevUtils) RegisterEnic(podName string) error {
	fmt.Printf("Registering ENiC...\n")

	enicUUID, err := getEnicUUIDInt(podName)
	if err != nil {
		return fmt.Errorf("error getting ENiC UUID: %w", err)
	}

	cli, err := GetClient()
	if err != nil {
		return fmt.Errorf("error creating HTTP client: %w", err)
	}

	orchPass, err := GetDefaultOrchPassword()
	if err != nil {
		return err
	}
	if orchPassEnv := os.Getenv("ORCH_PASS"); orchPassEnv != "" {
		orchPass = orchPassEnv
	}

	orchProject := defaultProject
	if orchProjectEnv := os.Getenv("ORCH_PROJECT"); orchProjectEnv != "" {
		orchProject = orchProjectEnv
	}

	orchUser := fmt.Sprintf("%s-%s", orchProject, "api-user")
	if orchUserEnv := os.Getenv("ORCH_USER"); orchUserEnv != "" {
		orchUser = orchUserEnv
	}

	orchFQDN := serviceDomain
	if orchFQDNEnv := os.Getenv("ORCH_FQDN"); orchFQDNEnv != "" {
		orchFQDN = orchFQDNEnv
	}

	apiToken, err := GetApiToken(cli, orchUser, orchPass)
	if err != nil {
		return fmt.Errorf("error getting API Token: %w", err)
	}

	apiBaseURLTemplate := "https://api.%s/v1/projects/%s"
	baseProjAPIUrl := fmt.Sprintf(apiBaseURLTemplate, orchFQDN, orchProject)
	hostRegUrl := baseProjAPIUrl + "/compute/hosts/register"

	fmt.Printf("Registering ENiC on %s (project: %s) with user %s and password %s\n", orchFQDN, orchProject, orchUser, orchPass)
	_, err = onboarding_manager.HttpInfraOnboardNewRegisterHost(hostRegUrl, *apiToken, cli, enicUUID, true)
	if err != nil {
		return fmt.Errorf("error registering ENiC: %w", err)
	}

	fmt.Printf("Registered ENiC ...\n")
	return nil
}

// ProvisionEnic Provisions ENiC with orchestrator, usage: mage devUtils:provisionEnic enic-0
func (DevUtils) ProvisionEnic(podName string) error {
	fmt.Printf("Provisioning ENiC...\n")

	enicUUID, err := getEnicUUIDInt(podName)
	if err != nil {
		return fmt.Errorf("error getting ENiC UUID: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), waitForReadyMin*time.Minute)
	defer cancel()

	cli, err := GetClient()
	if err != nil {
		return fmt.Errorf("error creating HTTP client: %w", err)
	}

	orchPass, err := GetDefaultOrchPassword()
	if err != nil {
		return err
	}
	if orchPassEnv := os.Getenv("ORCH_PASS"); orchPassEnv != "" {
		orchPass = orchPassEnv
	}

	orchProject := defaultProject
	if orchProjectEnv := os.Getenv("ORCH_PROJECT"); orchProjectEnv != "" {
		orchProject = orchProjectEnv
	}

	orchUser := fmt.Sprintf("%s-%s", orchProject, "api-user")
	if orchUserEnv := os.Getenv("ORCH_USER"); orchUserEnv != "" {
		orchUser = orchUserEnv
	}

	orchFQDN := serviceDomain
	if orchFQDNEnv := os.Getenv("ORCH_FQDN"); orchFQDNEnv != "" {
		orchFQDN = orchFQDNEnv
	}

	apiToken, err := GetApiToken(cli, orchUser, orchPass)
	if err != nil {
		return fmt.Errorf("error getting API Token: %w", err)
	}

	apiBaseURLTemplate := "https://api.%s/v1/projects/%s"
	baseProjAPIUrl := fmt.Sprintf(apiBaseURLTemplate, orchFQDN, orchProject)
	hostUrl := baseProjAPIUrl + "/compute/hosts"
	instanceUrl := baseProjAPIUrl + "/compute/instances"
	osUrl := baseProjAPIUrl + "/compute/os"

	hostID, err := onboarding_manager.HttpInfraOnboardGetHostID(ctx, hostUrl, *apiToken, cli, enicUUID.String())
	if err != nil {
		return fmt.Errorf("error getting ENiC resourceID: %w", err)
	}

	osID, err := onboarding_manager.HttpInfraOnboardGetOSID(ctx, osUrl, *apiToken, cli)
	if err != nil {
		return fmt.Errorf("error getting Ubuntu resourceID: %w", err)
	}

	err = onboarding_manager.HttpInfraOnboardNewInstance(instanceUrl, *apiToken, hostID, osID, cli)
	if err != nil {
		return fmt.Errorf("error provisioning ENiC: %w", err)
	}

	fmt.Printf("Provisioned ENiC ...\n")
	return nil
}

func getEnicReplicaCount() (int, error) {
	var replicas int

	fn := func() (*int, error) {
		// get the replica count from the ENiC replica set
		cmd := fmt.Sprintf("kubectl -n %s get statefulsets %s -o jsonpath='{.spec.replicas}'", enicNs, "enic")

		out, err := exec.Command("bash", "-c", cmd).Output()
		if err != nil {
			fmt.Printf("\rFailed to get ENiC replica count")
			return nil, fmt.Errorf("get ENiC replica count: %w", err)
		}
		// convert out to integer
		count, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			return nil, fmt.Errorf("failed to convert replicas to int: %w", err)
		}
		return &count, nil
	}

	for {
		count, err := fn()
		if err != nil {
			fmt.Println(fmt.Errorf("error while checking ENiC App: %w", err))
		}

		if count != nil {
			replicas = *count
			break
		}
		fmt.Println("Can't get ENiC replicas count, will check again 10 seconds 🟡")
		time.Sleep(10 * time.Second)
	}

	fmt.Printf("ENiC replicas count: %d\n", replicas)
	return replicas, nil
}

// GetEnicSerialNumber retrieves the ENiC serial number.
func (DevUtils) GetEnicSerialNumber() error {
	replicas, err := getEnicReplicaCount()
	if err != nil {
		return fmt.Errorf("error getting ENiC replica count: %w", err)
	}

	fmt.Println("ENiC Serial Numbers:")
	// iterate through the replicas to get the serial number
	for i := 0; i < replicas; i++ {
		pod := fmt.Sprintf("%s-%d", "enic", i)
		serialNumber, err := getEnicSNInt(pod)
		if err != nil {
			return fmt.Errorf("error getting ENiC SN: %w", err)
		}

		fmt.Printf("\t- %s: %s\n", pod, serialNumber)
	}

	return nil
}

// GetEnicUUID retrieves the ENiC UUID.
func (DevUtils) GetEnicUUID() error {
	replicas, err := getEnicReplicaCount()
	if err != nil {
		return fmt.Errorf("error getting ENiC replica count: %w", err)
	}

	fmt.Println("ENiC UUIDs:")
	// iterate through the replicas to get the serial number
	for i := 0; i < replicas; i++ {
		pod := fmt.Sprintf("%s-%d", "enic", i)
		uuid, err := getEnicUUIDInt(pod)
		if err != nil {
			return fmt.Errorf("error getting ENiC UUID: %w", err)
		}
		fmt.Printf("\t- %s: %s\n", pod, uuid)
	}

	return nil
}

// WaitForEnic waits for the ENiC pod to be in a running state.
func (DevUtils) WaitForEnic() error {
	ctx, cancel := context.WithTimeout(context.Background(), waitForReadyMin*time.Minute)
	defer cancel()
	counter := 0

	for {
		ready, err := isEnicArgoAppReady()
		if err != nil {
			fmt.Println(fmt.Errorf("error while checking ENiC App: %w", err))
		}

		if ready {
			break
		}
		fmt.Println("ENiC not synced or healthy, will check again 10 seconds 🟡")
		time.Sleep(10 * time.Second)
	}

	// Add another check for enic readiness, sometimes enic argo will be synced and healthy but no enic pod
	cmd := fmt.Sprintf("kubectl -n %s get pod/%s -o jsonpath='{.status.phase}'", enicNs, enicPodName)

	fmt.Printf("Waiting %v minutes for ENiC pod to start...\n", waitForReadyMin)
	fn := func() error {
		out, err := exec.Command("bash", "-c", cmd).Output()

		enicPodStatus := string(out)
		if enicPodStatus == "" {
			enicPodStatus = "Pending"
		}

		if err != nil || enicPodStatus != "Running" {
			fmt.Printf("\rENiC pod Status: %s (%vs)", enicPodStatus, counter*waitForNextSec)
			counter++
			return fmt.Errorf("enic pod is not ready")
		} else {
			fmt.Printf("\nENiC pod Status: %s (%vs)\n", enicPodStatus, counter*waitForNextSec)
			return nil
		}
	}

	if err := retry.UntilItSucceeds(ctx, fn, time.Duration(waitForNextSec)*time.Second); err != nil {
		return fmt.Errorf("enic pod setup error: %w 😲", err)
	}

	return nil
}

// WaitForEnicNodeAgent waits until node agent in ENiC reports INSTANCE_STATUS_RUNNING.
func (DevUtils) WaitForEnicNodeAgent() error {
	for {
		ready, err := isEnicArgoAppReady()
		if err != nil {
			fmt.Println(fmt.Errorf("error while checking ENiC App: %w", err))
		}

		if ready {
			break
		}
		fmt.Println("ENiC not synced or healthy, will check again 10 seconds 🟡")
		time.Sleep(10 * time.Second)
	}

	// Add another check for enic readiness, sometimes enic argo will be synced and healthy but no enic pod
	ctx, cancel := context.WithTimeout(context.Background(), waitForReadyMin*time.Minute)
	defer cancel()
	counter := 0

	cmd := fmt.Sprintf("kubectl -n %s exec -it $(kubectl -n %s get pods -l app=%s --no-headers | awk '{print $1}') -c %s -- journalctl -u node-agent -n 2",
		enicNs, enicNs, enicNs, enicContainerName)

	fmt.Printf("Waiting %v minutes for Node Agent in ENiC to be in Running Status ...\n", waitForReadyMin)
	fn := func() error {
		out, err := exec.Command("bash", "-c", cmd).Output()

		enicNodeAgentStatus := string(out)
		if enicNodeAgentStatus == "" {
			enicNodeAgentStatus = "Error"
		}

		if err != nil || !strings.Contains(enicNodeAgentStatus, "INSTANCE_STATUS_RUNNING") {
			fmt.Printf("\rNode Agent Status: %s (%vs)", enicNodeAgentStatus, counter*waitForNextSec)
			counter++
			return fmt.Errorf("ENiC Node Agent is not in Running Status")
		} else {
			fmt.Printf("\nNode Agent Status: %s (%vs)\n", enicNodeAgentStatus, counter*waitForNextSec)
			return nil
		}
	}

	if err := retry.UntilItSucceeds(ctx, fn, time.Duration(waitForNextSec)*time.Second); err != nil {
		return fmt.Errorf("ENiC Node Agent Status error: %w 😲", err)
	}

	return nil
}

const (
	defaultUser        = "all-groups-example-user"
	adminUser          = "admin"
	defaultServicePort = 443
)

// NOTE do we need to parametrize this for different envs?
var serviceDomainWithPort = fmt.Sprintf("%s:%d", serviceDomain, defaultServicePort)

func GetClient() (*http.Client, error) {
	caPool := x509.NewCertPool()
	// path is from the root Magefile
	pemBytes, err := script.File("./orch-ca.crt").Bytes()
	if err != nil {
		return nil, fmt.Errorf("load Orchestrator CA certificate. Did you deploy Orchestrator?: %w", err)
	}
	if len(pemBytes) == 0 {
		return nil, errors.New("./orch-ca.crt must not be empty")
	}
	caPool.AppendCertsFromPEM(pemBytes)
	cli := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				RootCAs:    caPool,
			},
		},
	}
	return cli, nil
}

func GetApiToken(client *http.Client, username string, password string) (*string, error) {
	v := url.Values{}

	v.Set("username", username)
	v.Add("password", password)
	v.Add("grant_type", "password")
	v.Add("client_id", "system-client")
	v.Add("scope", "openid")

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://keycloak."+serviceDomainWithPort+"/realms/master/protocol/openid-connect/token",
		strings.NewReader(v.Encode()),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		bodyString := string(bodyBytes)
		return nil, fmt.Errorf("cannot login: [%d] %s", resp.StatusCode, bodyString)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	err = json.NewDecoder(resp.Body).Decode(&tokenResp)
	if err != nil {
		return nil, err
	}
	if len(tokenResp.AccessToken) == 0 {
		return nil, errors.New("empty access token")
	}
	return &tokenResp.AccessToken, nil
}

// Deprecated: use tenantUtils.GetProjectID
func RuntimeProjUid(projName string) (string, error) {
	kubeCmd := fmt.Sprintf("kubectl get runtimeproject -o json -l nexus/display_name=%s", projName)

	data, err := script.Exec(kubeCmd).String()
	if err != nil {
		return "", fmt.Errorf("runtimeProjUid error: %w (%s)", err, data)
	}
	uid, err := script.Echo(data).JQ(`.items | .[0] | .metadata.uid`).Replace(`"`, "").String()
	if err != nil {
		return "", fmt.Errorf("runtimeProjUid cannot parse json: %w (%s)", err, data)
	}
	if uid == "" {
		return "", fmt.Errorf("cannot find UID in: %s", data)
	}

	uid = strings.TrimSpace(uid)

	return uid, nil
}

func GetUserID(cli *http.Client, username, token string) (string, error) {
	req, err := http.NewRequestWithContext(
		context.TODO(), // TODO: Allow the user to pass a proper context.
		http.MethodGet,
		"https://keycloak."+serviceDomainWithPort+"/admin/realms/master/users?username="+username,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := cli.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var userInfo []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(bodyBytes, &userInfo); err != nil {
		return "", fmt.Errorf("parse JSON: %w", err)
	}

	if len(userInfo) == 0 {
		return "", fmt.Errorf("empty list of users")
	}

	// Assuming the first user is the one we are looking for.
	return userInfo[0].ID, nil
}

// ManageRoles Assigns roles to a user. Requires an admin token to make the changes in Keycloak.
// Deprecated: assign Groups instead of roles, see tenantUtils.addUserToGroups
func ManageRoles(cli *http.Client, token, action, username, orgUUID, projUUID string) error {
	var roles []string
	var roleID string

	log.Printf("Managing(%s) roles to user %s in org %s and project %s\n", action, username, orgUUID, projUUID)

	// TODO consider assigning the group directly (instead of individual roles)
	if orgUUID == "" && projUUID == "" {
		// if ProjectID and orgUUID is empty we are assigning the org-manager-group roles
		roleList := []string{"org-read-role", "org-write-role", "org-update-role", "org-delete-role"}
		roles = append(roles, roleList...)
	} else if projUUID == "" {
		// if ProjectID is empty we are assigning the project-manager-group roles
		roleList := []string{"project-read-role", "project-write-role", "project-update-role", "project-delete-role"}
		for _, roleName := range roleList {
			prefixedRole := fmt.Sprintf("%s_%s", orgUUID, roleName)
			roles = append(roles, prefixedRole)
		}
	} else {
		// if the project ID is specified we assign:
		// - member role: which contains both Org and Project ID and is only used by tenant manager to allow access to a specific project
		roles = append(roles, fmt.Sprintf("%s_%s_%s", orgUUID, projUUID, "m"))
		// - all the roles that the services require
		// TODO consider assigning/removing the group(s) directly (instead of individual roles)
		roleList := []string{
			"en-agent-rw",
			"im-rw",
			"cl-rw",
			"cl-tpl-rw",
			"alrt-rw",
			"ao-rw",
			"cat-rw",
			"tc-r",
		}
		for _, roleName := range roleList {
			prefixedRole := fmt.Sprintf("%s_%s", projUUID, roleName)
			roles = append(roles, prefixedRole)
		}
	}

	log.Printf("Updating(%s) roles %v to user %s and role ID %s", action, roles, username, roleID)

	userId, err := GetUserID(http.DefaultClient, username, token)
	if err != nil {
		return fmt.Errorf("failed to get user ID: %w", err)
	}

	pollIntervalInSecs := time.Duration.Seconds(1)
	retryCount := 120 // wait for 2 mins
	for i := 0; i < retryCount; i++ {
		var errs []error
		for _, roleName := range roles {
			if err := manageRole(context.TODO(), cli, token, action, userId, roleName); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) == 0 {
			return nil
		}
		fmt.Printf("Not all roles are assigned, failed with an error: %v, Retrying...", errs)
		time.Sleep(time.Duration(pollIntervalInSecs))
	}
	return fmt.Errorf("time out waiting for roles to be assigned")
}

func ManageTenancyUserAndRoles(ctx context.Context, cli *http.Client, orgId, projId, action, tenancyUser string, removeUser bool) error {
	client, token, err := KeycloakLogin(ctx)
	if err != nil {
		return err
	}

	params := gocloak.GetUsersParams{
		Username: &tenancyUser,
	}

	users, err := client.GetUsers(ctx, token.AccessToken, KeycloakRealm, params)
	if err != nil {
		fmt.Printf("error getting user %s: %v", tenancyUser, err)
		return err
	}
	userExists := false
	userId := ""
	for _, user := range users {
		if *user.Username == tenancyUser {
			userId = *user.ID
			userExists = true
			break
		}
	}
	if removeUser {
		if !userExists {
			fmt.Println("no user found to delete. Skipping delete...")
			return nil
		}
		err = deleteTenancyUser(ctx, client, token, userId)
		if err != nil {
			fmt.Printf("error deleting user %s: %v", tenancyUser, err)
			return err
		}
		return nil
	}
	if !userExists {
		userId, err = createTenancyUser(ctx, client, token, tenancyUser)
		if err != nil {
			return err
		}
	}
	if orgId == "" {
		roleList := []string{"org-read-role", "org-write-role", "org-update-role", "org-delete-role"}
		for _, role := range roleList {
			err := manageRole(ctx, cli, token.AccessToken, action, userId, role)
			if err != nil {
				return err
			}
		}
		fmt.Printf("Action (%s) taken for SI Admin Roles for the user %s", action, tenancyUser)
	} else if projId == "" {
		var roles []string
		roleList := []string{"project-read-role", "project-write-role", "project-update-role", "project-delete-role"}
		for _, roleName := range roleList {
			prefixedRole := fmt.Sprintf("%s_%s", orgId, roleName)
			roles = append(roles, prefixedRole)
		}
		for _, role := range roles {
			err := manageRole(ctx, cli, token.AccessToken, action, userId, role)
			if err != nil {
				return err
			}
		}
		fmt.Printf("Action (%s) taken for Org Admin Roles for the user %s", action, tenancyUser)
	} else {
		role := fmt.Sprintf("%s_%s_m", orgId, projId)
		var roles []string
		roles = append(roles, role)
		roleList := []string{
			"en-agent-rw",
			"im-rw",
			"cl-rw",
			"cl-tpl-rw",
			"alrt-rw",
			"ao-rw",
			"cat-rw",
			"tc-r",
		}
		for _, roleName := range roleList {
			prefixedRole := fmt.Sprintf("%s_%s", projId, roleName)
			roles = append(roles, prefixedRole)
		}

		for _, role := range roles {
			err := manageRole(ctx, cli, token.AccessToken, action, userId, role)
			if err != nil {
				return err
			}
		}
		fmt.Printf("Action (%s) taken for Member Roles for the user %s", action, tenancyUser)
	}
	return nil
}

func GetRoleFromKeycloak(ctx context.Context, cli *http.Client, token, roleName string) (string, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://keycloak."+serviceDomainWithPort+"/admin/realms/master/roles",
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := cli.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	var availableRoles []struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(bodyBytes, &availableRoles); err != nil {
		return "", fmt.Errorf("error parsing JSON: %w", err)
	}

	var roleID string
	for _, role := range availableRoles {
		if role.Name == roleName {
			roleID = role.ID
			break
		}
	}
	return roleID, nil
}

func manageRole(ctx context.Context, cli *http.Client, token, action, userID, roleName string) error {
	roleID, err := GetRoleFromKeycloak(ctx, cli, token, roleName)
	if err != nil {
		return fmt.Errorf("error getting role %s: %w", roleName, err)
	}

	if roleID == "" {
		return fmt.Errorf("role not found: %s", roleName)
	}

	payload := []map[string]string{
		{"id": roleID, "name": roleName},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, // TODO: Allow the user to pass a proper context.
		action,
		fmt.Sprintf("https://keycloak."+serviceDomainWithPort+"/admin/realms/master/users/%s/role-mappings/realm", userID),
		bytes.NewReader(jsonData),
	)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := cli.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to update role %s: %+v", roleName, resp)
	}
	fmt.Println("Role updated successfully:", roleName)
	return nil
}

func (DevUtils) CreateDefaultUser(ctx context.Context) error {
	client, token, err := KeycloakLogin(ctx)
	if err != nil {
		return err
	}

	// Create an array of gocloak.User
	users := []gocloak.User{
		{
			Username: gocloak.StringP("service-admin-example-user"),
			RealmRoles: &[]string{
				"default-roles-master",
			},
			Groups: &[]string{
				"service-admin-group",
			},
			FirstName: gocloak.StringP("Service"),
			LastName:  gocloak.StringP("Admin"),
		},
		{
			Username: gocloak.StringP("edge-manager-example-user"),
			RealmRoles: &[]string{
				"default-roles-master",
			},
			Groups: &[]string{
				"edge-manager-group",
			},
			FirstName: gocloak.StringP("Edge"),
			LastName:  gocloak.StringP("Manager"),
		},
		{
			Username: gocloak.StringP("edge-operator-example-user"),
			RealmRoles: &[]string{
				"default-roles-master",
			},
			Groups: &[]string{
				"edge-operator-group",
			},
			FirstName: gocloak.StringP("Edge"),
			LastName:  gocloak.StringP("Operator"),
		},
		{
			Username: gocloak.StringP("host-manager-example-user"),
			RealmRoles: &[]string{
				"default-roles-master",
			},
			Groups: &[]string{
				"host-manager-group",
			},
			FirstName: gocloak.StringP("Host"),
			LastName:  gocloak.StringP("Manager"),
		},
		{
			Username: gocloak.StringP("iam-admin-example-user"),
			RealmRoles: &[]string{
				"default-roles-master",
			},
			Groups: &[]string{
				"iam-admin-group",
			},
			FirstName: gocloak.StringP("IAM"),
			LastName:  gocloak.StringP("Admin"),
		},
		{
			Username: gocloak.StringP("sre-example-user"),
			RealmRoles: &[]string{
				"default-roles-master",
			},
			Groups: &[]string{
				"sre-group",
			},
			FirstName: gocloak.StringP("SRE"),
			LastName:  gocloak.StringP("Admin"),
		},
		{
			Username: gocloak.StringP("no-groups-example-user"),
			RealmRoles: &[]string{
				"default-roles-master",
			},
			FirstName: gocloak.StringP("No Groups"),
			LastName:  gocloak.StringP("Example User"),
		},
		{
			Username: gocloak.StringP("all-groups-example-user"),
			RealmRoles: &[]string{
				"default-roles-master",
				"en-agent-rw",
			},
			Groups: &[]string{
				"service-admin-group",
				"edge-manager-group",
				"edge-operator-group",
				"host-manager-group",
				"iam-admin-group",
				"sre-group",
			},
			FirstName: gocloak.StringP("All"),
			LastName:  gocloak.StringP("Groups"),
		},
	}

	fmt.Printf("Creating (%d) Example Users\n", len(users))

	for _, user := range users {
		// create example user
		userId, err := CreateDefaultKeyCloakUser(ctx, client, token, &user, "example")
		if err != nil {
			fmt.Printf("failed to create user: %s, %s\n", *user.Username, err.Error())
			continue
		}

		// // add user to group
		if user.Groups != nil {
			userGroup := *user.Groups
			err = addUserToGroups(ctx, client, token, KeycloakRealm, userGroup, userId)
			if err != nil {
				return fmt.Errorf("error adding org roles to user %s. Error: %w", defaultUser, err)
			}
		}

		// // add realm role to user
		var roles []gocloak.Role
		for _, role := range *user.RealmRoles {
			realmRole, err := getRealmRole(ctx, client, token, "master", role)
			if err != nil {
				return err
			}
			roles = append(roles, *realmRole)
		}

		err = client.AddRealmRoleToUser(ctx, token.AccessToken, "master", userId, roles)
		if err != nil {
			return err
		}
		fmt.Printf("added member roles to the user %s\n", userId)
		time.Sleep(1 * time.Second)
	}

	return nil
}
