// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	"net/http"
	"crypto/tls"
	"github.com/bitfield/script"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var pwChars = []rune(`abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789`)

func randomPassword(count int) string {
	runes := make([]rune, count)

	for i := 0; i < count; i++ {
		runes[i] = pwChars[rand.Intn(len(pwChars))]
	}
	return string(runes)
}

const (
	harborPasswordLength   = 100
	keycloakPasswordLength = 14
)

var (
	harborPassword   = randomPassword(harborPasswordLength)
	keycloakPassword = randomPassword(keycloakPasswordLength)
)

const giteaPasswordLength = 100

var (
	adminGiteaUsername  	= "gitea_admin"
	adminGiteaPassword   	= randomPassword(giteaPasswordLength)
	argoGiteaUsername     	= "argocd"
	argoGiteaPassword    	= randomPassword(giteaPasswordLength)
	appGiteaUsername     	= "apporch"
	appGiteaPassword     	= randomPassword(giteaPasswordLength)
	clusterGiteaUsername 	= "clusterorch"
	clusterGiteaPassword 	= randomPassword(giteaPasswordLength)
	deployRepoClusterUrl	= "https://gitea-http.gitea.svc.cluster.local/argocd/edge-manageability-framework.git"
	deployRepoPath 			= "argocd/edge-manageability-framework"
	deployRepoName 			= "edge-manageability-framework"	
	deployGiteaRepoDir 		= ".deploy/gitea"	
)

func (Deploy) all(targetEnv string) error {
	// validate the password, so we don't waste time later on in life
	_, err := GetDefaultOrchPassword()
	if err != nil {
		return err
	}

	if err := (Deploy{}).kind(targetEnv); err != nil {
		return err
	}

	// TBD: commit deployment and configs to gitea and deploy using argo referencing to gitea

	// TBD: Restore after walkthrough and implementation of deploy through Gitea workflow

	if err := (Deploy{}).orchLocal(targetEnv); err != nil {
		return err
	}
	return nil
}

func (Deploy) kind(targetEnv string) error { //nolint:gocyclo
	// TBD: Validate the targetEnv as kind appropriate, not a specified preset name

	targetEnvType, err := (Config{}).getTargetEnvType(targetEnv)
	if err != nil || targetEnvType != "kind" {
		return fmt.Errorf("wrong environment specified for kind deployment: %s is a %s orchestrator definition", targetEnv, targetEnvType)
	}

	if err := checkEnv(targetEnv); err != nil {
		return err
	}
	if err := AsdfPlugins(); err != nil {
		return err
	}
	if err := copyPolicy("bootstrap/audit-policy.yaml", "/tmp/policies"); err != nil {
		return err
	}

	// TODO: Before destroying environment Check if the tls-orch secret exists, if it does persist it
	targetAutoCertEnabled, _ := (Config{}).isAutoCertEnabled(targetEnv)
	if autoCert && targetAutoCertEnabled {
		existingCert := gatewayTLSSecretValid()
		if existingCert {
			fmt.Println("Gateway TLS secret exists, persisting secret")
			if err := saveGatewayTLSSecret("aws"); err != nil {
				return err
			}
		}
	}

	if err := kindCluster(kindOrchClusterName, targetEnv); err != nil {
		return err
	}

	// If cache registry URL is set, load the cert into the kind cluster
	cacheRegistry, _ := (Config{}).getDockerCache(targetEnv)
	if cacheRegistry != "" {
		if err := (loadKindRegistryCacheCerts("kind-control-plane")); err != nil {
			return fmt.Errorf("error loading registry cache CA certificates into kind cluster: %w", err)
		}
	}

	if err := createNamespaces(); err != nil {
		return fmt.Errorf("error creating namespaces: %w", err)
	}

	// TBD: Extend support for gernerally configurable token based release service authentication. This is currently not supported.
	if err := localSecret(targetEnv, false); err != nil {
		return fmt.Errorf("error creating local secrets: %w", err)
	}

	// Check if the tls-orch secret exists on the filesystem
	// if it does, then read it and create the tls-orch secret with it
	if autoCert && targetAutoCertEnabled {
		fmt.Println("Restoring existing tls-orch secret")
		if err := restoreGatewayTLSSecret("aws"); err != nil {
			fmt.Printf("Error restoring secret: %+v\n", err)
		}
	}

	if err := deployMetalLB(); err != nil {
		return fmt.Errorf("error deploying metallb: %w", err)
	}

	// TBD: This should not be a preset name based decision. This should be a cluster configuration setting.
	targetMailpitEnabled, _ := (Config{}).isMailpitEnabled(targetEnv)
	if targetMailpitEnabled {
		if err := deployMailpit(); err != nil {
			return err
		}
	}

	if err := (Deploy{}).generateInfraCerts(); err != nil {
		return err
	}

	if err := (Deploy{}).Gitea(targetEnv); err != nil {
		return err
	}

	if err := (Deploy{}).Argocd(targetEnv); err != nil {
		return err
	}

	if err := (Router{}).Stop(); err != nil {
		return err
	}
	if err := (Router{}).Start(); err != nil {
		return err
	}

	// Wait for Argo CD deployment to finish and the load balancer to be updated
	for i := 0; i < argoRetryCount; i++ {
		var err error
		if err = (Argo{}).login(); err == nil {
			break
		}
		if i == argoRetryCount-1 {
			fmt.Printf("Max retry reached\n")
			return err
		}
		fmt.Printf("Login failed, retrying in %d seconds... (Attempt %d)\n", argoRetryInterval, i+1)
		time.Sleep(argoRetryInterval * time.Second)
	}

	if err := (Argo{}).repoAdd(); err != nil {
		return err
	}

	if err := (Argo{}).dockerHubChartOrgAdd(); err != nil {
		return err
	}
	fmt.Println("kind cluster ready: 😊")
	return nil
}

func (Deploy) addKyvernoPolicy() error {
	requireRoRootFs := `
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-ro-rootfs
  annotations:
    policies.kyverno.io/title: Require Read-Only Root Filesystem
    policies.kyverno.io/category: Best Practices, EKS Best Practices, PSP Migration
    policies.kyverno.io/severity: medium
    policies.kyverno.io/subject: Pod
    policies.kyverno.io/minversion: 1.6.0
    policies.kyverno.io/description: >-
      A read-only root file system helps to enforce an immutable infrastructure strategy;
      the container only needs to write on the mounted volume that persists the state.
      An immutable root filesystem can also prevent malicious binaries from writing to the
      host system. This policy validates that containers define a securityContext
      with readOnlyRootFilesystem: true.
spec:
  validationFailureAction: audit
  background: true
  rules:
  - name: validate-readOnlyRootFilesystem
    match:
      any:
      - resources:
          kinds:
          - Pod
    validate:
      message: "Root filesystem must be read-only."
      pattern:
        spec:
          containers:
          - securityContext:
              readOnlyRootFilesystem: true
`
	out, err := script.Echo(requireRoRootFs).Exec("kubectl apply -f - ").String()
	fmt.Println(out)
	return err
}

// Install Orch CA to trusted CA list.
func (Deploy) orchCA() error {
	return addCATrustStore("orch-ca.crt")
}

// called with "apply" or "delete" argument for kubectl command
func (Deploy) victoriaMetrics(cmd string) error {
	metricsTemplate := template.Must(template.New("victoria-metrics").
		Parse(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: victoriametrics-deployment
  namespace: orch-sre
spec:
  replicas: 1
  selector:
    matchLabels:
      app: victoriametrics
  template:
    metadata:
      labels:
        app: victoriametrics
    spec:
      containers:
        - name: victoriametrics
          image: victoriametrics/victoria-metrics:latest
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 8428
          args:
            - '-httpAuth.username=sre'
            - '-httpAuth.password={{ .SrePassword }}'
            - '-maxLabelsPerTimeseries=40'
---
apiVersion: v1
kind: Service
metadata:
  name: sre-exporter-destination
  namespace: orch-sre
spec:
  selector:
    app: victoriametrics
  ports:
    - protocol: TCP
      port: 8428
      targetPort: 8428
  type: ClusterIP
---
`))
	if cmd != "apply" && cmd != "delete" {
		return fmt.Errorf("invalid command: %s. Use 'apply' or 'delete'", cmd)
	}
	// set basic-auth-password for sre-exporter to be the same as the default orchestrator password
	pass, err := GetDefaultOrchPassword()
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}

	if err := metricsTemplate.Execute(
		buf,
		struct {
			SrePassword string
		}{
			SrePassword: pass,
		},
	); err != nil {
		return fmt.Errorf("error in executing template: %w", err)
	}
	out, err := script.Echo(buf.String()).Exec(fmt.Sprintf("kubectl %s -f -", cmd)).String()
	fmt.Println(out)
	return err
}

func (Deploy) LEOrchestratorCABundle() error {
	return addCATrustStore("orchestrator-ca-bundle.crt")
}

func checkEnv(targetEnv string) error {
	if os.Getenv("GIT_USER") == "" {
		return fmt.Errorf("must set environment variable GIT_USER")
	}
	if os.Getenv("GIT_TOKEN") == "" {
		return fmt.Errorf("must set environment variable GIT_TOKEN")
	}

	targetConfig := getTargetConfig(targetEnv)
	if _, err := os.Stat(targetConfig); os.IsNotExist(err) {
		return fmt.Errorf("invalid cluster config: %s", targetConfig)
	}
	return nil
}

func kubectlCreateAndApply(createArgs ...string) error {
	createCmdBase := []string{"kubectl", "create"}

	// Append the --dry-run=client -o yaml flags to the create command
	createCmd := append(createCmdBase, createArgs[0:]...)
	dryrunCmd := append(createCmd, "--dry-run=client", "-o", "yaml")
	dryrunExec := exec.Command(dryrunCmd[0], dryrunCmd[1:]...)

	// Capture the dry-run output as the resource to apply
	var applyYaml bytes.Buffer
	dryrunExec.Stdout = &applyYaml
	if err := dryrunExec.Run(); err != nil {
		return fmt.Errorf("exec %s error %w: %s", strings.Join(dryrunCmd, " "), err, applyYaml.String())
	}

	// Execute kubectl apply using YAML generated by the dry-run as input
	applyExec := exec.Command("kubectl", "apply", "-f", "-")
	applyExec.Stdin = strings.NewReader(applyYaml.String())

	var applyOut bytes.Buffer
	applyExec.Stdout = &applyOut
	if err := applyExec.Run(); err != nil {
		return fmt.Errorf("apply failed with error %w: %s", err, applyOut.String())
	}

	return nil
}

// Secret Manager is not available for environments other than EKS clusters and Coder
func secretsManagerAvailable(targetEnv string) bool {
	// Config will contain the following:
	// argo:
	//   enabled:
	//     aws-sm-proxy: true

	if targetEnv == "" {
		return false
	}

	clusterFilePath := fmt.Sprintf("orch-configs/clusters/%s.yaml", targetEnv)
	clusterValues, err := parseClusterValues(clusterFilePath)
	if err != nil {
		return false
	}

	argoConfig, ok := clusterValues["argo"].(map[string]interface{})
	if !ok {
		return false
	}

	autoCertConfig, ok := argoConfig["enabled"].(map[string]interface{})
	if !ok {
		return false
	}

	enabled, ok := autoCertConfig["aws-sm-proxy"].(bool)
	if !ok {
		return false
	}

	return enabled
}

func createLocalSreSecrets() error {
	// basic-auth-username for sre-exporter
	if err := kubectlCreateAndApply("secret", "generic", "-n", "orch-sre", "basic-auth-username",
		"--from-literal=username=sre"); err != nil {
		return err
	}
	// set basic-auth-password for sre-exporter to be the same as the default orchestrator password
	pass, err := GetDefaultOrchPassword()
	if err != nil {
		return err
	}
	// basic-auth-password for sre-exporter
	if err := kubectlCreateAndApply("secret", "generic", "-n", "orch-sre", "basic-auth-password",
		fmt.Sprintf("--from-literal=password=%s", pass)); err != nil {
		return err
	}
	// destination-secret-url for sre-exporter
	if err := kubectlCreateAndApply("secret", "generic", "-n", "orch-sre", "destination-secret-url",
		"--from-literal=url=http://sre-exporter-destination.orch-sre.svc.cluster.local:8428/api/v1/write"); err != nil {
		return err
	}
	return nil
}

// FIXME Revisit this code when we have Secrets Manager
func localSecret(targetEnv string, createRSToken bool) error {
	if err := kubectlCreateAndApply("namespace", "orch-harbor"); err != nil {
		return err
	}

	// creating harbor-admin-password for orch-harbor such that it doesn't include "username:"
	if err := kubectlCreateAndApply("secret", "generic", "-n", "orch-harbor", "harbor-admin-password",
		"--from-literal=HARBOR_ADMIN_PASSWORD="+harborPassword); err != nil {
		return err
	}

	// harbor-admin-credential secret to facilate harbor access through curl commands or using username/password
	if err := kubectlCreateAndApply("secret", "generic", "-n", "orch-harbor", "harbor-admin-credential",
		"--from-literal=credential=admin:"+harborPassword); err != nil {
		return err
	}

	// creating platform-keycloak secret that contains the randomly generated keycloak admin password
	if err := kubectlCreateAndApply("secret", "generic", "-n", "orch-platform", "platform-keycloak",
		"--from-literal=admin-password="+keycloakPassword); err != nil {
		return err
	}

	// TBD: Extend support for gernerally configurable token based release service authentication. This is currently not supported.
	// if createRSToken {
	// 	// for environments w/o Secret Manager we have to create respective secrets.
	// 	// azure-ad-creds for release service
	// 	if _, err := script.Exec("./tools/create-azuread-creds-secret.sh").Stdout(); err != nil {
	// 		return err
	// 	}
	// }

	if !secretsManagerAvailable(targetEnv) {
		if err := createLocalSreSecrets(); err != nil {
			return err
		}
	}
	return nil
}

func copyPolicy(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	_, err = os.Open(dst)
	if os.IsNotExist(err) {
		err := os.MkdirAll(dst, 0o777)
		if err != nil {
			return err
		}
	}

	file, err := os.Create(dst + "/audit-policy.yaml")
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, source)
	if err != nil {
		return err
	}

	return nil
}

func kindCluster(name string, targetEnv string) error {
	mg.Deps(Undeploy{}.Kind)

	apiServerAddress := "127.0.0.1"
	if _, ok := os.LookupEnv("EXTERNAL_K8S_API"); ok {
		primary, err := getPrimaryIP()
		if err != nil {
			return err
		}
		apiServerAddress = primary.String()
	}

	// Create Docker config file with optional token from an environment variable.
	// If the token is not provided, kind will pull docker images without authentication.
	dockerToken := os.Getenv("DOCKERHUB_TOKEN")
	dockerUsername := os.Getenv("DOCKERHUB_USERNAME")
	dockerDir := os.Getenv("HOME") + "/docker"

	if dockerToken != "" {
		auth := fmt.Sprintf("%s:%s", dockerUsername, dockerToken)
		encodedToken := base64.StdEncoding.EncodeToString([]byte(auth))
		dockerConfig := fmt.Sprintf(`{
  "auths": {
    "https://index.docker.io/v1/": {
      "auth": "%s"
    }
  }
}`, encodedToken)

		if err := os.MkdirAll(dockerDir, 0o755); err != nil {
			return fmt.Errorf("error creating %s directory: %w", dockerDir, err)
		}

		if err := os.WriteFile(dockerDir+"/config.json", []byte(dockerConfig), 0o644); err != nil {
			return fmt.Errorf("error writing Docker config file: %w", err)
		}
	}

	cacheRegistry, _ := (Config{}).getDockerCache(targetEnv)
	cacheRegistryURL := ""
	if cacheRegistry != "" {
		cacheRegistryURL = fmt.Sprintf("https://%s", cacheRegistry)
		(Gen{}).RegistryCacheCert(targetEnv) 
	}

	//nolint: lll
	kindTemplate := template.Must(template.New("kind-cluster").
		Parse(`kind create cluster --name {{ .Name }} --image kindest/node:v1.30.3 --config - <<EOF
    kind: Cluster
    apiVersion: kind.x-k8s.io/v1alpha4
    networking:
      apiServerAddress: {{ .APIServerAddress }}
    nodes:
    - role: control-plane
      extraMounts:
      - hostPath: audit-policy.yaml
        containerPath: /tmp/policies/audit-policy.yaml
        readOnly: true
      - hostPath: /var/run/docker.sock
        containerPath: /var/run/docker.sock
    {{- if .DockerToken }}
      - hostPath: {{ .DockerDir }}/config.json
        containerPath: /var/lib/kubelet/config.json
        readOnly: true
    {{- end }}
      kubeadmConfigPatches:
      - |
        kind: ClusterConfiguration
        apiServer:
        # enable auditing flags on the API server
          extraArgs:
            audit-log-path: /var/log/kubernetes/policies/kube-apiserver-audit.log
            audit-policy-file: /etc/kubernetes/policies/audit-policy.yaml
            # mount new files / directories on the control plane
          extraVolumes:
          - name: audit-policies
            hostPath: /tmp/policies
            mountPath: /etc/kubernetes/policies
            readOnly: true
            pathType: "DirectoryOrCreate"
          - name: audit-logs
            hostPath: "/var/log/kubernetes/policies"
            mountPath: "/var/log/kubernetes/policies"
            readOnly: false
            pathType: DirectoryOrCreate
        ---
        apiVersion: kubelet.config.k8s.io/v1beta1
        kind: KubeletConfiguration
        maxPods: 250
        serializeImagePulls: false
    containerdConfigPatches:
      - |-
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        {{- if ne .CacheRegistryURL "" }}
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."*"]
              endpoint = ["{{ .CacheRegistryURL }}"]
          {{- if ne .CacheRegistryHost "" }}
            [plugins."io.containerd.grpc.v1.cri".registry.configs]
              [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .CacheRegistryHost }}".tls]
                ca_file = "/usr/local/share/ca-certificates/registry-cache-ca.crt"
          {{- end }}
        {{- end }}
EOF`))

	buf := &bytes.Buffer{}

	if err := kindTemplate.Execute(
		buf,
		struct {
			Name              string
			APIServerAddress  string
			ClusterDomain     string
			DockerToken       string
			DockerDir         string
			CacheRegistryURL  string
			CacheRegistryHost string
		}{
			Name:              name,
			APIServerAddress:  apiServerAddress,
			ClusterDomain:     "kind.internal",
			DockerToken:       dockerToken,
			DockerDir:         dockerDir,
			CacheRegistryURL:  cacheRegistryURL,
			CacheRegistryHost: cacheRegistry,
		},
	); err != nil {
		return fmt.Errorf("error in executing template: %w", err)
	}

	return sh.RunV("sh", "-c", buf.String())
}

const (
	nsCreateRetries    = 5
	nsCreateRetryDelay = 50
)

// Create namespaces without erroring out if they already exist
func createNamespaces() error {
	if autoCert && coderEnv {
		// Add orch-gateway namespace
		// Used for reusing certificates in coder environments
		argoNamespaces = append(argoNamespaces, "orch-gateway")
	}

	// Create Namespaces without Istio injection label
	for _, namespace := range argoNamespaces {
		fmt.Printf("Creating namespace %s\n", namespace)
		for attempt := 1; attempt <= nsCreateRetries; attempt++ {
			if err := kubectlCreateAndApply("ns", namespace); err != nil {
				if attempt == nsCreateRetries {
					return fmt.Errorf("error creating namespace %s after %d attempts: %w", namespace, nsCreateRetries, err)
				}
				time.Sleep(nsCreateRetryDelay * time.Millisecond)
				continue
			}
			break
		}
	}

	return nil
}

func deployMetalLB() error {
	cmd := "helm repo add metallb https://metallb.github.io/metallb --force-update"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}
	cmd = "helm upgrade --install metallb metallb/metallb --version 0.13.11 -f bootstrap/metallb.yaml --wait -n metallb-system --create-namespace"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}
	if err := metalSetup(); err != nil {
		return err
	}
	return nil
}

func deployMailpit() error {
	cmd := "kubectl create namespace mailpit-dev"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}
	cmd = "kubectl apply -f e2e-tests/mailpit/mail_catcher.yaml -n mailpit-dev"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}

	cmd = "kubectl apply -f e2e-tests/mailpit/smtp_secret.yaml -n orch-infra"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}
	return nil
}

// MetalSetup sets up MetalLB which uses Docker network IPs for k8s Load Balancer services.
func metalSetup() error {
	data, err := script.NewPipe().Exec(fmt.Sprintf("docker network inspect %s", kindOrchClusterName)).String()
	if err != nil {
		return fmt.Errorf("docker network inspect %s: %w", data, err)
	}
	ipamConfigList, err := script.Echo(data).JQ(".[] | .IPAM.Config").String()
	if err != nil {
		return fmt.Errorf("jq network: %w", err)
	}
	type ipamConfig struct {
		Subnet  string `json:"Subnet"`
		Gateway string `json:"Gateway"`
	}
	var ipamConfigs []ipamConfig
	err = json.Unmarshal([]byte(ipamConfigList), &ipamConfigs)
	if err != nil {
		return fmt.Errorf("unmarshal ipamConfig: %w", err)
	}

	// Search for first IPv4 subnet
	var ipnet *net.IPNet
	for _, item := range ipamConfigs {
		_, ipnet, err = net.ParseCIDR(strings.TrimSpace(item.Subnet))
		if err != nil {
			continue
		}
		if ipnet.IP.To4() != nil {
			break
		}
	}
	if ipnet == nil {
		return fmt.Errorf("unable to find IPv4 subnet from %s network", kindOrchClusterName)
	}
	// convert IPNet struct mask and address to uint32
	mask := binary.BigEndian.Uint32(ipnet.Mask)
	start := binary.BigEndian.Uint32(ipnet.IP)
	// find the final address
	finish := (start & mask) | (mask ^ 0xffffffff)

	rangeStart := make(net.IP, 4)
	binary.BigEndian.PutUint32(rangeStart, finish-20) // use the last 20 ips

	rangeEnd := make(net.IP, 4)
	binary.BigEndian.PutUint32(rangeEnd, finish)

	// validate that range start and end are inside of the cidr
	if !ipnet.Contains(rangeStart) {
		return fmt.Errorf("network %v does not contain range start %v", ipnet, rangeStart)
	}
	if !ipnet.Contains(rangeEnd) {
		return fmt.Errorf("network %v does not contain range end %v", ipnet, rangeEnd)
	}
	fmt.Printf("%v\n", rangeStart)
	fmt.Printf("%v\n", rangeEnd)

	metalTemplate := template.Must(template.New("metal-LB").
		Parse(`apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
    name: example
    namespace: metallb-system
spec:
    addresses:
    - {{ .IPStart }}-{{ .IPEnd }}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
    name: empty
    namespace: metallb-system`))

	buf := &bytes.Buffer{}

	if err := metalTemplate.Execute(
		buf,
		struct {
			IPStart string
			IPEnd   string
		}{
			IPStart: rangeStart.String(),
			IPEnd:   rangeEnd.String(),
		},
	); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}
	fmt.Println(buf.String())
	out, err := script.Echo(buf.String()).Exec("kubectl apply -f -").String()
	if err != nil {
		return fmt.Errorf("docker network inspect: %w %s", err, out)
	}
	fmt.Println(out)
	return nil
}

// loadKindRegistryCacheCerts loads the Configured Docker registry cache's x509 CA certificate into the kind nodes system
// trust store to allow the container runtime to pull images from Docker Hub through the registry cache over TLS.
func loadKindRegistryCacheCerts(cluster string) error {
	// This CA file is rendered as part of kind config generation if a cache registry URL is present in the cluster
	// definition. This function should not be called if there is no cache registry URL.
	certFile := filepath.Join("mage", "registry-cache-ca.crt")

	if _, err := os.Stat(certFile); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("registry cache CA certificate not found: %s", certFile)
	}

	clusterName := "name=" + cluster
	c, builder := exec.Command(
		"docker",
		"ps",
		"-q",
		"-f",
		clusterName), new(strings.Builder)
	c.Stdout = builder
	err := c.Run()
	print(builder.String())
	if err != nil {
		return fmt.Errorf("error searching for kind-control-plane: %w", err)
	}
	if builder.String() == "" {
		return fmt.Errorf("error finding kind-control-plane cluster, is KinD cluster created?")
	}

	cpDst := cluster + ":/usr/local/share/ca-certificates/registry-cache-ca.crt"
	if err := sh.Run(
		"docker",
		"cp",
		certFile,
		cpDst,
	); err != nil {
		return fmt.Errorf("error copying certificates into kind node container: %w", err)
	}

	if err := sh.Run(
		"docker",
		"exec",
		cluster,
		"update-ca-certificates",
	); err != nil {
		return fmt.Errorf("error executing update CA certificates command within kind node container: %w", err)
	}

	if err := sh.Run(
		"docker",
		"exec",
		cluster,
		"systemctl",
		"restart",
		"containerd",
	); err != nil {
		return fmt.Errorf("error executing containerd restart to apply CA certificates within kind node container: %w", err) //nolint:lll
	}

	return nil
}

func joinNamedParams(valueName string, values []string) string {
	var sb strings.Builder

	argPrefix := ""
	if len(valueName) == 1 {
		argPrefix = "-"
	} else if len(valueName) > 1 {
		argPrefix = "--"
	}

	for _, v := range values {
		sb.WriteString(argPrefix)
		sb.WriteString(valueName)
		sb.WriteString(" ")
		sb.WriteString(v)
		sb.WriteString(" ")
	}

	return strings.TrimSpace(sb.String())
}

// Generate TLS cert for Gitea and ArgoCD. Must be executed before deploying Gitea and ArgoCD
// since tls-orch will not be created until later stage.
func (Deploy) generateInfraCerts() error {
	// Process Subject Alrternative Names (SAN)
	// The cert is signed for both *.kind.internal and *.serviceDomain.
	commonName := "*.kind.internal"
	san := fmt.Sprintf("subjectAltName=DNS:%s,DNS:%s", commonName, fmt.Sprintf("*.%s", serviceDomain))

	// Generate infra TLS cert
	cmd := fmt.Sprintf("openssl req -x509 -nodes -days 365 -newkey rsa:4096 -keyout infra-tls.key -out infra-tls.crt "+
		"-subj '/CN=%s' -addext '%s'", commonName, san)
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}

	// Export TLS cert for Gitea as K8s secret
	cmd = "kubectl -n gitea create secret tls gitea-tls-certs --cert=infra-tls.crt --key=infra-tls.key"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}

	// Export TLS cert for ArgoCD as K8s secret
	cmd = "kubectl -n argocd create secret tls argocd-server-tls --cert=infra-tls.crt --key=infra-tls.key"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}

	return nil
}

func (Deploy) gitea(bootstrapValues []string, targetEnv string) error {
	// Deploy Gitea
	giteaRegistry := "oci://registry-1.docker.io/giteacharts/gitea"

	bootstrapParam := joinNamedParams("values", bootstrapValues)
	cmd := fmt.Sprintf("helm -n gitea upgrade --install gitea %s --version %s "+
		bootstrapParam+" --set gitea.admin.username='%s' --set gitea.admin.password='%s' --create-namespace --wait",
		giteaRegistry, giteaVersion, adminGiteaUsername, adminGiteaPassword)
	fmt.Printf("Deploying Gitea with cmd: %s\n", cmd)
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}
	// Store admin credential in K8s Secret
	// This is not consumed during normal situlations
	fmt.Println("Creating admin-gitea-credential secret")
	if err := createOrUpdateGiteaSecret("admin-gitea-credential", adminGiteaUsername, adminGiteaPassword); err != nil {
		return err
	}
	// Create argocd account in Gitea
	fmt.Println("Creating argocd account in Gitea")
	if err := createOrUpdateGiteaAccount(argoGiteaUsername, argoGiteaPassword); err != nil {
		fmt.Println("Error creating argocd account in Gitea")
		return err
	}
	// Store argocd gitea credential in K8s Secret
	fmt.Println("Creating argocd-gitea-credential secret")
	if err := createOrUpdateGiteaSecret("argocd-gitea-credential", argoGiteaUsername, argoGiteaPassword); err != nil {
		fmt.Println("Error creating argocd-gitea-credential secret")
		return err
	}
	// Create apporch account in Gitea
	fmt.Println("Creating apporch account in Gitea")
	if err := createOrUpdateGiteaAccount(appGiteaUsername, appGiteaPassword); err != nil {
		fmt.Println("Error creating apporch account in Gitea")
		return err
	}
	// Store app orch gitea credential in K8s Secret
	// This will later be connsumed by adm-secret to create Vault secret ma_git_service
	fmt.Println("Creating app-gitea-credential secret")
	if err := createOrUpdateGiteaSecret("app-gitea-credential", appGiteaUsername, appGiteaPassword); err != nil {
		fmt.Println("Error creating app-gitea-credential secret")
		return err
	}
	// Create clusterorch acccount in Gitea
	fmt.Println("Creating clusterorch account in Gitea")
	if err := createOrUpdateGiteaAccount(clusterGiteaUsername, clusterGiteaPassword); err != nil {
		fmt.Println("Error creating clusterorch account in Gitea")
		return err
	}
	// Store cluster orch gitea credential in K8s Secret
	// This will later be connsumed by adm-secret to create Vault secret mc_git_service
	fmt.Println("Creating cluster-gitea-credential secret")
	if err := createOrUpdateGiteaSecret("cluster-gitea-credential", clusterGiteaUsername, clusterGiteaPassword); err != nil {
		fmt.Println("Error creating cluster-gitea-credential secret")
		return err
	}

	// Create the Gitea argocd edge-managability-framework repo
	fmt.Println("Creating edge-managability-framework repo in Gitea")
	if err := createOrUpdateGiteaRepo(argoGiteaUsername, argoGiteaPassword, deployRepoName); err != nil {
		fmt.Println("Error creating edge-managability-framework repo in Gitea")
		return err
	}

	return nil
}

// Create or update a Gitea secret
func createOrUpdateGiteaSecret(secretName string, username string, password string) error {
	cmd := fmt.Sprintf("kubectl -n orch-platform create secret generic %s "+
		"--from-literal=username='%s' --from-literal=password='%s' --dry-run=client -o yaml", secretName, username, password)
	secret, err := script.Exec(cmd).String()
	if err != nil {
		return err
	}

	cmd = "kubectl apply -f -"
	if _, err := script.Echo(secret).Exec(cmd).String(); err != nil {
		return err
	}
	return nil
}

// Create or update a Gitea account
func createOrUpdateGiteaAccount(username string, password string) error {
	cmd := "kubectl get pods -n gitea -l app.kubernetes.io/name=gitea -o jsonpath='{.items[0].metadata.name}'"
	out, err := script.Exec(cmd).String()
	if err != nil {
		return err
	}
	kubectlPrefix := fmt.Sprintf("kubectl exec -n gitea -c gitea %s --", strings.TrimSpace(out))

	cmd = fmt.Sprintf("%s gitea admin user list", kubectlPrefix)
	match, err := script.Exec(cmd).Match(username).CountLines()
	if err != nil {
		return err
	}

	// Create user if not exists
	if match == 0 {
		fmt.Println("User does not exist, creating")
		cmd = fmt.Sprintf("%s gitea admin user create --username %s --password %s --email %s", kubectlPrefix, username, password, username+"@local.domain")
		if _, err := script.Exec(cmd).String(); err != nil {
			return err
		}
	}
	// Ensure password is update-to-date when updating the password
	cmd = fmt.Sprintf("%s gitea admin user change-password --username %s --password %s --must-change-password=false", kubectlPrefix, username, password)
	if _, err := script.Exec(cmd).String(); err != nil {
		return err
	}
	return nil
}

func (Deploy) getArgoGiteaCredentials() (string, string, error) {
	// Load the username from the Kubernetes secret argocd-gitea-credential in orch-platform namespace
	cmd := "kubectl get secret argocd-gitea-credential -n orch-platform -o jsonpath='{.data.username}'"
	encodedUsername, err := script.Exec(cmd).String()
	if err != nil {
		return "", "", fmt.Errorf("error retrieving username from Kubernetes secret: %w", err)
	}
	argoGiteaUsername := strings.TrimSpace(string(encodedUsername))

	// Load the password from the Kubernetes secret argocd-gitea-credential in orch-platform namespace
	cmd = "kubectl get secret argocd-gitea-credential -n orch-platform -o jsonpath='{.data.password}'"
	encodedPassword, err := script.Exec(cmd).String()
	if err != nil {
		return "", "", fmt.Errorf("error retrieving password from Kubernetes secret: %w", err)
	}
	argoGiteaPassword := strings.TrimSpace(string(encodedPassword))

	// Decode the base64 encoded username and password
	decodedUsername, err := base64.StdEncoding.DecodeString(argoGiteaUsername)
	if err != nil {
		return "", "", fmt.Errorf("error decoding username: %w", err)
	}
	decodedPassword, err := base64.StdEncoding.DecodeString(argoGiteaPassword)
	if err != nil {
		return "", "", fmt.Errorf("error decoding password: %w", err)
	}

	return string(decodedUsername), string(decodedPassword), nil
}

func (Deploy) startGiteaPortForward() (*exec.Cmd, error) {
	// Get the service port for gitea-http in the gitea namespace
	cmd := "kubectl get svc gitea-http -n gitea -o jsonpath='{.spec.ports[0].port}'"
	servicePort, err := script.Exec(cmd).String()
	fmt.Printf("Service port: %s\n", servicePort)
	if err != nil {
		return nil, fmt.Errorf("error retrieving gitea-http service port: %w", err)
	}
	servicePort = strings.TrimSpace(servicePort)

	// Start kubectl port-forward in the background
	fmt.Printf("Starting port-forward for Gitea on port 9654 to service port %s\n", servicePort)
	portForwardCmd := exec.Command("kubectl", "port-forward", "-n", "gitea", "svc/gitea-http", "9654:"+servicePort)
	portForwardCmd.Stdout = os.Stdout
	portForwardCmd.Stderr = os.Stderr
	if err := portForwardCmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting port-forward: %w", err)
	}

	// Wait for the port forwarding to be established
	portForwardRetries := 10
	portForwardDelay := 2 * time.Second
	for i := 0; i < portForwardRetries; i++ {
		time.Sleep(portForwardDelay)
		// Create an HTTP client that skips server certificate validation
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
		resp, err := client.Get("https://localhost:9654/api/v1/version")
		if err != nil {
			fmt.Printf("Error connecting to Gitea: %v\n", err)
		} else {
			fmt.Printf("Gitea response: %s\n", resp.Status)
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Printf("Error reading response body: %v\n", err)
			} else {
				fmt.Printf("Response body: %s\n", string(body))
			}
			defer resp.Body.Close()
		}
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if i == portForwardRetries-1 {
			portForwardCmd.Process.Kill()
			return portForwardCmd, fmt.Errorf("timed out attempting to establish port forwarding for Gitea: %w", err)
		}
	}	

	fmt.Printf("Port-forward started with PID: %d\n", portForwardCmd.Process.Pid)

	return portForwardCmd, nil
}

func (Deploy) stopGiteaPortForward(portForwardCmd *exec.Cmd) error {
	if err := portForwardCmd.Process.Kill(); err != nil {
		return fmt.Errorf("error stopping port-forward: %v", err)
	}
	return nil
}

func createOrUpdateGiteaRepo(username string, password string, repo string) error {
	// Get the Gitea credentials from the Kubernetes secret, the randomly generated password constants are not
	// safe as this commit/update may be part of a separate mage run than the initial deployment
	gitUsername, gitPassword, err := (Deploy{}).getArgoGiteaCredentials()
	if err != nil {
		return fmt.Errorf("error getting Gitea credentials: %w", err)
	}
	
	portForwardCmd, err := (Deploy{}).startGiteaPortForward()
	if err != nil {	
		return fmt.Errorf("error starting Gitea port-forward: %w", err)
	}
	defer func() {
		if err := (Deploy{}).stopGiteaPortForward(portForwardCmd); err != nil {
			fmt.Printf("error stopping Gitea port-forward: %v\n", err)
		}
	}()


	url := "https://localhost:9654/api/v1/user/repos"
	payload := fmt.Sprintf(`{"name": "%s"}`, repo)
	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %w", err)
	}
	req.SetBasicAuth(gitUsername, gitPassword)
	req.Header.Set("Content-Type", "application/json")

	// Create an HTTP client that skips server certificate validation
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create repository, status: %s, response: %s", resp.Status, string(body))
	}
	fmt.Printf("%s repository created successfully\n", repo)

	return nil
}

func (Deploy) updateDeployRepo(targetEnv, gitRepoPath, repoName, localClonePath string) error {
	// Get the current working directory so we can return to it when this function exits
	originalDir, err := os.Getwd()
	fmt.Printf("updateDeployRepo inital working directory: %s\n", originalDir)
	if err != nil {
		return fmt.Errorf("error getting current working directory: %w", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			fmt.Printf("error changing back to original directory: %v\n", err)
		}
	}()

	portForwardCmd, err := (Deploy{}).startGiteaPortForward()
	if err != nil {	
		return fmt.Errorf("error starting Gitea port-forward: %w", err)
	}
	defer func() {
		if err := (Deploy{}).stopGiteaPortForward(portForwardCmd); err != nil {
			fmt.Printf("error stopping Gitea port-forward: %v\n", err)
		}
	}()

	// Get the Gitea credentials from the Kubernetes secret, the randomly generated password constants are not
	// safe as this commit/update may be part of a separate mage run than the initial deployment
	gitUsername, gitPassword, err := (Deploy{}).getArgoGiteaCredentials()
	if err != nil {
		return fmt.Errorf("error getting Gitea credentials: %w", err)
	}
	
	// Set GIT_SSL_NO_VERIFY=true for git commmands that we are running through the port forward tunnel
	os.Setenv("GIT_SSL_NO_VERIFY", "true")

	// Ensure the localClonePath path exists and doesn't have a copy of the repo already cloned
	if _, err := os.Stat(localClonePath); os.IsNotExist(err) {
		if err := os.MkdirAll(localClonePath, 0o755); err != nil {
			return fmt.Errorf("error creating directory %s: %w", localClonePath, err)
		}
	}
	if err := os.Chdir(localClonePath); err != nil {
		return fmt.Errorf("error changing to directory %s: %w", localClonePath, err)
	}
	if _, err := os.Stat(deployRepoName); err == nil {
		if err := os.RemoveAll(deployRepoName); err != nil {
			return fmt.Errorf("error removing directory %s: %w", deployRepoName, err)
		}
	}

	// Clone the repository using the port-forwarded address
	cmd := fmt.Sprintf("git clone https://localhost:9654/%s %s", gitRepoPath, repoName)
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return fmt.Errorf("error cloning repository: %w", err)
	}

	// Copy files and directories to the newly cloned deployRepoPath
	filesToCopy := []string{"VERSION", "argocd", "orch-configs/profiles"}
	filesToCopy = append(filesToCopy, fmt.Sprintf("orch-configs/clusters/%s.yaml", targetEnv))

	for _, file := range filesToCopy {
		src := filepath.Join("../..", file)
		dst := filepath.Join(repoName, file)

		// Check if the source exists
		if _, err := os.Stat(src); os.IsNotExist(err) {
			return fmt.Errorf("source file or directory does not exist: %s", src)
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("error creating directory %s: %w", filepath.Dir(dst), err)
		}

		// Copy the file or directory
		cmd := fmt.Sprintf("cp -r %s %s", src, dst)
		if _, err := script.Exec(cmd).Stdout(); err != nil {
			return fmt.Errorf("error copying %s to %s: %w", src, dst, err)
		}
	}

	// Navigate to the deployRepoPath
	if err := os.Chdir(repoName); err != nil {
		return fmt.Errorf("error changing to directory %s: %w", repoName, err)
	}

	// Add all changes to git
	cmd = "git add ."
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return fmt.Errorf("error adding changes to git: %w", err)
	}

	// Get the VERSION file content
	version, err := os.ReadFile("VERSION")
	if err != nil {
		return fmt.Errorf("error reading VERSION file: %w", err)
	}
	version = bytes.TrimSpace(version)

	// Commit the changes with the appropriate message
	cmd = fmt.Sprintf("git commit -m 'update deployment to version: %s'", version)
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return fmt.Errorf("error committing changes: %w", err)
	}

	// Push the changes to the gitea repository
	cmd = fmt.Sprintf("git push https://%s:%s@localhost:9654/%s", gitUsername, gitPassword, gitRepoPath)
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return fmt.Errorf("error pushing changes to remote repository: %w", err)
	}

	// Add any additional logic for updating the repository if needed
	fmt.Printf("Repository %s updated successfully at %s\n", deployRepoName, deployRepoPath)
	return nil
}

// Deploy ArgoCD using helm chart
func (Deploy) argocd(bootstrapValues []string, targetEnv string) error {
	// TBD: Fix the cert to be config based rather than server query based. Current handling is a generalization of
	//      pre-OSS logic that queries a well known, trusted cache server for the cert rather than configuring a known
	//      good cert. This is not a secure practice and should be fixed.
	registryCertName := "blank" // this variable will be ignored by a helm chart if useIntelRegistry is false
	registryCertPem := []byte("blank")
	dockerCache, _ := (Config{}).getDockerCache(targetEnv)
	if dockerCache == "" {
		registryCertServer := strings.ReplaceAll(dockerCache, ".", "\\.")
		registryCertName = fmt.Sprintf("configs.tls.certificates.%s", registryCertServer)
		var err error
		registryCertPem, err = os.ReadFile(filepath.Join("mage", "registry-cache-ca.crt"))
		if err != nil {
			return fmt.Errorf("read registry certificate file: %w", err)
		}
	}
	cmd := "helm repo add argo-helm https://argoproj.github.io/argo-helm --force-update"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}

	// FIXME Workaround for ArgoCD not applying CA file when pulling from OCI registry. Remove this once the issue is fixed
	// Ref: https://github.com/argoproj/argo-cd/issues/13726, https://github.com/argoproj/argo-cd/issues/14877
	if err := kubectlCreateAndApply("ns", "argocd"); err != nil {
		return err
	}
	registryCertParam := fmt.Sprintf("--from-literal=registry-certs.crt=\"%s\"", registryCertPem)
	if err := kubectlCreateAndApply("configmap", "registry-certs", "-n", "argocd", registryCertParam); err != nil {
		return err
	}

	argocdAdminPassword, err := GetDefaultOrchPassword()
	if err != nil {
		return fmt.Errorf("could not get default password: %w", err)
	}
	hashPass, err := hashArgoCDPassword(argocdAdminPassword)
	if err != nil {
		return fmt.Errorf("could not hash password: %w", err)
	}

	// Generate TLS cert for Argo CD server
	bootstrapParam := joinNamedParams("values", bootstrapValues)
	cmd = fmt.Sprintf("helm -n argocd upgrade --install argocd argo-helm/argo-cd --version %s "+
		bootstrapParam+" --create-namespace --set '%s=%s' --set 'configs.cm.users.session.duration=24h' "+
		"--set 'configs.secret.argocdServerAdminPassword=%s' --wait",
		argoVersion, registryCertName, registryCertPem, hashPass)
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}
	return nil
}

func hashArgoCDPassword(password string) (string, error) {
	cmd := fmt.Sprintf("argocd account bcrypt --password %s", password)
	data, err := script.Exec(cmd).String()
	if err != nil {
		return "", err
	}
	return data, nil
}

func getConfigsDir() string {
	orchConfigsDir := "./orch-configs"
	return orchConfigsDir
}

func getTargetConfig(targetEnv string) string {
	orchConfigsDir := getConfigsDir()
	return fmt.Sprintf("%s/clusters/%s.yaml", orchConfigsDir, targetEnv)
}

func getDeployDir() string {
	edgeManageabilityFrameworkDir := os.Getenv("EDGE_MANAGEABILITY_FRAMEWORK_DIR")
	if edgeManageabilityFrameworkDir == "" {
		edgeManageabilityFrameworkDir = "."
	}
	return edgeManageabilityFrameworkDir
}

func getDeployRevision() string {
	deployRevision := os.Getenv("EDGE_MANAGEABILITY_FRAMEWORK_REV")
	if deployRevision == "" {
		deployDir := getDeployDir()
		if _, err := os.Stat(deployDir); os.IsNotExist(err) {
			fmt.Println("failed to locate deploy (.) repo, using cluster default deploy revision")
			return ""
		} else {
			cmd := fmt.Sprintf("bash -c 'cd %s; git rev-parse --short HEAD'", deployDir)
			out, err := script.Exec(cmd).String()
			if err != nil {
				fmt.Println("failed to determine deployRevision: %w", err)
				fmt.Println("  using cluster default configs revision")
				return ""
			}
			deployRevision = strings.TrimSpace(out)
		}
	}
	return deployRevision
}

func getDeployTag() (string, error) {
	var deployTag string
	deployDir := getDeployDir()
	if _, err := os.Stat(deployDir); os.IsNotExist(err) {
		return "", fmt.Errorf("failed to locate deploy repo: %w", err)
	}
	if versionBuf, err := os.ReadFile(filepath.Join(deployDir, "VERSION")); err != nil {
		return "", fmt.Errorf("failed to read version file: %w", err)
	} else {
		deployTag = strings.TrimSpace(string(versionBuf))
	}
	if strings.Contains(deployTag, "-dev") {
		deployRevision := getDeployRevision()
		if len(deployRevision) == 0 {
			return "", fmt.Errorf("failed to get edge-manageability-framework revision")
		}
		deployTag = deployTag + "-" + deployRevision
	}

	deployTag = "v" + deployTag
	return deployTag, nil
}

func getDeployRevisionParam() string {
	deployRevision := getDeployRevision()
	if deployRevision == "" {
		return ""
	}
	return fmt.Sprintf("--set-string argo.deployRepoRevision=%s ", deployRevision)
}

func getConfigsRevision() string {
	configsRevision := os.Getenv("ORCH_CONFIG_REV")
	if configsRevision == "" {
		configsDir := getConfigsDir()
		if _, err := os.Stat(configsDir); os.IsNotExist(err) {
			fmt.Println("failed to locate config repo, using cluster default configs revision")
			return ""
		} else {
			cmd := fmt.Sprintf("bash -c 'cd %s; git rev-parse --short HEAD'", configsDir)
			out, err := script.Exec(cmd).String()
			if err != nil {
				fmt.Println("failed to determine configsRevision: %w", err)
				fmt.Println("  using cluster default configs revision")
				return ""
			}
			configsRevision = strings.TrimSpace(out)
		}
	}
	return configsRevision
}

func getConfigsRevisionParam() string {
	configsRevision := getConfigsRevision()
	if configsRevision == "" {
		return ""
	}
	return fmt.Sprintf("--set-string argo.configsRepoRevision=%s ", configsRevision)
}

func getOrchestratorVersion() (string, error) {
	version, err := getVersionFromFile()
	if err != nil {
		return "", err
	}
	imageTags, err := getVersionTags(version)
	if err != nil {
		return "", err
	}
	if len(imageTags) < 1 {
		return "", fmt.Errorf("cannot get version tag from version %s", version)
	}
	return imageTags[0], nil
}

func getOrchestratorVersionParam() (string, error) {
	version, err := getOrchestratorVersion()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("--set-string argo.orchestratorVersion=%s ", version), nil
}

// Root app that starts the deployment of children apps.
func (d Deploy) orch(targetEnv string) error {
	targetConfig := getTargetConfig(targetEnv)

	// Clone and update the Gitea deployment repo
	if err := (Deploy{}).updateDeployRepo(targetEnv, deployRepoPath, deployRepoName, deployGiteaRepoDir); err != nil {
		return fmt.Errorf("error updating deployment repo content: %w", err)
	}

	cmd := fmt.Sprintf("helm upgrade --install root-app argocd/root-app -f %s -n %s --create-namespace", targetConfig, targetEnv)
	_, err := script.Exec(cmd).Stdout()
	return err
}

func (d Deploy) orchLocal(targetEnv string) error {
	var subDomain string
	// deployRevision := getDeployRevisionParam()
	// configsRevision := getConfigsRevisionParam()
	// orchVersion, err := getOrchestratorVersionParam()
	// if err != nil {
	// 	return err
	// }

	targetConfig := getTargetConfig(targetEnv)

	// Clone and update the Gitea deployment repo
	if err := (Deploy{}).updateDeployRepo(targetEnv, deployRepoPath, deployRepoName, deployGiteaRepoDir); err != nil {
		return fmt.Errorf("error updating deployment repo content: %w", err)
	}

	cmd := fmt.Sprintf("helm upgrade --install root-app argocd/root-app -f %s -n %s --create-namespace", targetConfig, targetEnv)
	_, err := script.Exec(cmd).Stdout()

	// We are now taking current code and pushing to main in gitea. There is no longer a need for the revision params and overwrite
	// cmd := fmt.Sprintf("helm upgrade --install root-app argocd/root-app -f %s  -n %s --create-namespace %s %s %s"+
	// 	"--set root.useLocalValues=true", targetConfig, targetEnv, deployRevision, configsRevision, orchVersion)

	// only for coder deployments
	targetAutoCertEnabled, _ := (Config{}).isAutoCertEnabled(targetEnv)
	if autoCert && targetAutoCertEnabled {
		// retrieve the subdomain name
		subDomain = os.Getenv("ORCH_DOMAIN")
		if subDomain == "" {
			return fmt.Errorf("error retrieving the orchestrator domain from autocert aws lookup")
		}

		// override the clusterDomain and self-signed-cert values
		// setting the clusterDomain to the domain name generated with the cluster creation in the format orch-<ip>.espdqa.infra-host.com
		// setting self-signed-cert generation to false as the cert will be issued with lets encrypt
		cmd = cmd + " " + fmt.Sprintf("--set argo.autoCert.enabled=true --set argo.self-signed-cert.generateOrchCert=false --set argo.clusterDomain=%s", subDomain)

		// Get AWS account ID
		awsAccountID, err := script.Exec("aws sts get-caller-identity --query Account --output text").String()
		if err != nil {
			return fmt.Errorf("error retrieving the AWS account ID: %w", err)
		}
		cmd = cmd + " " + fmt.Sprintf("--set argo.aws.account=%s", strings.Trim(awsAccountID, "\n"))

		// Get AWS region of this VM
		az, err := script.Exec("curl -s http://169.254.169.254/latest/meta-data/placement/availability-zone").String()
		if err != nil || az == "" {
			return fmt.Errorf("error retrieving the AWS AZ: %w", err)
		}
		region := az[:len(az)-1]
		cmd = cmd + " " + fmt.Sprintf("--set argo.aws.region=%s", region)
	}
	fmt.Printf("exec: %s\n", cmd)
	_, err = script.Exec(cmd).Stdout()

	if autoCert && (strings.HasPrefix(targetEnv, "dev") || strings.HasPrefix(targetEnv, "ext")) {
		fmt.Printf("Orchestrator will be available at domain : %s\n", subDomain)
	}
	return err
}
