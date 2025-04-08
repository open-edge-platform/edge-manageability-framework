// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package mage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/bitfield/script"
)

// skipRouter is a package-level variable intialized during startup.
var skipRouter = func() bool {
	// when the environment variable is set (SKIP_ROUTER=1), the router start/stop commands will be skipped
	value := os.Getenv("SKIP_ROUTER")
	return value == "1"
}()

type parameters struct {
	ArgoIP          string
	OrchIP          string
	BootsIP         string
	GiteaIP         string
	ExternalDomain  string
	IsSandbox       bool
	SandboxCertFile string
	SandboxKeyFile  string
	InternalDomain  string
	Hosts           []string
}

func (r Router) start(externalDomain string, sandboxKeyFile string, sandboxCertFile string) error {
	if skipRouter {
		fmt.Println("skipping router start")
		return nil
	}
	argoIP, err := awaitGenericIP("argocd", "argocd-server", 20*time.Second)
	if err != nil {
		return fmt.Errorf("performing argo IP lookup %w", err)
	}
	giteaIP, err := awaitGenericIP("gitea", "gitea-http", 20*time.Second)
	if err != nil {
		return fmt.Errorf("performing argo IP lookup %w", err)
	}
	orchIP, err := awaitGenericIP("orch-gateway", "traefik", 20*time.Second)
	if err != nil {
		fmt.Printf("WARNING: could not find orchestrator IP: %s\n", err)
		fmt.Println("Looks like Orchestrator Traefik isn't ready yet. Please run:")
		fmt.Println("`mage router:stop router:start` after orchestrator is up and running")
		orchIP = "0.0.0.0"
	}

	sandboxKeyFileAbs, err := filepath.Abs(sandboxKeyFile)
	if err != nil {
		return err
	}

	sandboxCertFileAbs, err := filepath.Abs(sandboxCertFile)
	if err != nil {
		return err
	}
	uniquehosts, domainname, err := uniqueHosts()
	if orchIP != "0.0.0.0" && err != nil {
		// skip this error if orchIP is undefined
		return err
	}

	// if auto-cert retrieve domainname from system
	// this is done because the domain would not be known by uniqueHosts until
	// after cert-manager is deployed and running.
	if autoCert {
		// retrieve the subdomain name
		domainname = os.Getenv("ORCH_DOMAIN")
		if domainname == "" {
			return fmt.Errorf("ORCH_DOMAIN is required to enable AUTO_CERT")
		}
	} else if domainname == "" {
		domainname = defaultClusterDomain
	}

	bootsIP, err := awaitGenericIP("orch-boots", "ingress-nginx-controller", 20*time.Second)
	if err != nil {
		fmt.Printf("WARNING: could not find boots IP %v\n", err)
		fmt.Println("Looks like Orchestrators nginx Boots isn't ready yet. Please run:")
		fmt.Println("`mage router:stop router:start` after Orchestrator is up and running")
		bootsIP = "0.0.0.0"
	}

	params := parameters{
		ArgoIP:          argoIP,
		OrchIP:          orchIP,
		BootsIP:         bootsIP,
		GiteaIP:         giteaIP,
		ExternalDomain:  externalDomain,
		IsSandbox:       externalDomain != "",
		SandboxKeyFile:  sandboxKeyFileAbs,
		SandboxCertFile: sandboxCertFileAbs,
		InternalDomain:  domainname,
		Hosts:           uniquehosts,
	}

	err = r.checkExternalDeps(params)
	if err != nil {
		return fmt.Errorf("dependencies not met. %w", err)
	}
	funcMap := template.FuncMap{
		"justHost": func(hostFqdn string) string { return strings.Split(hostFqdn, ".")[0] },
	}

	routerTemplate, err := script.File("./tools/router/traefik.template").String()
	if err != nil {
		return err
	}
	template := template.Must(template.New("router-template").Funcs(funcMap).Parse(routerTemplate))

	buf := &bytes.Buffer{}

	if err := template.Execute(
		buf,
		params,
	); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}
	if _, err := script.Echo(buf.String()).WriteFile("./tools/router/traefik.yml"); err != nil {
		return err
	}

	err = r.generateDockerTemplate(params)
	if err != nil {
		return err
	}
	if _, err := script.Exec("docker-compose -f ./tools/router/docker-compose.yml up -d").Stdout(); err != nil {
		return err
	}
	return nil
}

// Check the cert and key exists and match each other and are valid for the hostname.
func (Router) checkExternalDeps(params parameters) error {
	if !params.IsSandbox {
		return nil
	}
	keyModulus, err := script.Exec(fmt.Sprintf("timeout 3 openssl rsa -modulus -noout -in %s",
		params.SandboxKeyFile)).String()
	if err != nil {
		// Traefik Router cannot accept an encrypted Private key.
		// If key is encrypted it will present a prompt for a pass phrase during the above command.
		return fmt.Errorf("cannot check Key %s. It should not be encrypted. %s %w",
			params.SandboxKeyFile, keyModulus, err)
	}
	crtModulus, err := script.Exec(fmt.Sprintf("openssl x509 -modulus -noout -in %s",
		params.SandboxCertFile)).String()
	if err != nil {
		return fmt.Errorf("cannot check Cert %s. %s %w", params.SandboxCertFile, crtModulus, err)
	}

	if keyModulus != crtModulus {
		return fmt.Errorf("sandbox Cert %s does not match private Key %s",
			params.SandboxCertFile, params.SandboxKeyFile)
	}

	checkHost, err := script.Exec(fmt.Sprintf("openssl x509 -checkhost %s -noout -in %s",
		params.ExternalDomain, params.SandboxCertFile)).String()
	if err != nil {
		return fmt.Errorf("cannot check Cert hostname %s. %s %w", params.SandboxCertFile, checkHost, err)
	}
	// Expecting 'Hostname <hostname> does match certificate'.
	if strings.Contains(checkHost, "NOT") {
		return fmt.Errorf("%s", checkHost)
	}

	oneDay := time.Second * 60 * 24
	checkEnd, err := script.Exec(fmt.Sprintf("openssl x509 -checkend %d -noout -in %s",
		int(oneDay.Seconds()), params.SandboxCertFile)).String()
	if err != nil {
		return fmt.Errorf("cannot check Cert expiry %s. %s %w", params.SandboxCertFile, checkHost, err)
	}
	// Expecting 'certificate will not expire'.
	if !strings.Contains(checkEnd, "not") {
		return fmt.Errorf("%s within %s", checkEnd, oneDay.String())
	}

	return nil
}

func (Router) stop() error {
	if skipRouter {
		fmt.Println("skipping router stop")
		return nil
	}
	// If docker-compose file is not present, then there is nothing to bring down.
	if err := script.IfExists("./tools/router/docker-compose.yml").Error(); err == nil {
		if _, errDown := script.Exec("docker-compose -f ./tools/router/docker-compose.yml down").Stdout(); err != nil {
			return errDown
		}
	}
	return nil
}

func awaitGenericIP(namespace, serviceName string, duration time.Duration) (string, error) {
	timeout := time.After(duration)
	tickDuration := duration / 5
	if tickDuration < 2*time.Second {
		tickDuration = 2 * time.Second
	}
	tick := time.Tick(tickDuration)

	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("timed out waiting for IP of %s:%s", namespace, serviceName)
		case <-tick:
			ip, err := lookupGenericIP(namespace, serviceName)
			if err == nil {
				return ip, nil
			}
			fmt.Printf("Retrying lookup for %s:%s due to error: %v\n", namespace, serviceName, err)
		}
	}
}

func lookupGenericIP(namespace, serviceName string) (string, error) {
	fmt.Printf("looking up %s:%s IP\n", namespace, serviceName)
	cmd := fmt.Sprintf("kubectl -n %s get svc %s -o json", namespace, serviceName)
	data, err := script.Exec(cmd).String()
	if err != nil {
		return "", fmt.Errorf("failed to lookup service details: %w", err)
	}

	var parsedData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &parsedData); err != nil {
		return "", fmt.Errorf("failed to parse service details: %w", err)
	}

	status, ok := parsedData["status"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("status field not found or invalid")
	}
	loadBalancer, ok := status["loadBalancer"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("loadBalancer field not found or invalid")
	}
	ingress, ok := loadBalancer["ingress"].([]interface{})
	if !ok || len(ingress) == 0 {
		return "", fmt.Errorf("ingress field not found or empty")
	}

	firstIngress, ok := ingress[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("ingress[0] is not a valid object")
	}
	ip, ok := firstIngress["ip"].(string)
	if !ok || ip == "" {
		return "", fmt.Errorf("IP field not found or empty in ingress[0]")
	}

	genericIP := strings.TrimSpace(ip)
	fmt.Printf("found GenericIP: %s for %s:%s\n", genericIP, namespace, serviceName)
	return genericIP, nil
}

// Generate docker-compose.yml.
func (Router) generateDockerTemplate(params parameters) error {
	dockerTemplateFile, err := script.File("./tools/router/docker-compose.template").String()
	if err != nil {
		return err
	}
	dockerTemplate := template.Must(template.New("docker-compose-template").Parse(dockerTemplateFile))

	buf := &bytes.Buffer{}

	if err := dockerTemplate.Execute(
		buf,
		params,
	); err != nil {
		return fmt.Errorf("executing docker template: %w", err)
	}
	if _, err := script.Echo(buf.String()).WriteFile("./tools/router/docker-compose.yml"); err != nil {
		return err
	}
	return nil
}
