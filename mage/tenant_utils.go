// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/bitfield/script"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	KeycloakRealm  = "master"
	defaultOrg     = "sample-org"
	defaultProject = "sample-project"
)

// CreateDefaultMtSetup creates one Org, one Project, one Project admin, CO and Edge Infrastructure Manager users in the Project.
func (TenantUtils) CreateDefaultMtSetup(ctx context.Context) error {
	fmt.Println("Creating default organization...")
	err := TenantUtils{}.CreateOrg(ctx, defaultOrg)
	if err != nil {
		return fmt.Errorf("failed to create org: %w", err)
	}
	fmt.Println("Default organization created successfully.")

	projectAdminUser := fmt.Sprintf("%s-admin", defaultOrg)
	fmt.Printf("Creating project admin user '%s' in organization '%s'...\n", projectAdminUser, defaultOrg)
	err = TenantUtils{}.CreateProjectAdminInOrg(ctx, defaultOrg, projectAdminUser)
	if err != nil {
		return fmt.Errorf("failed to create project admin in org: %w", err)
	}
	fmt.Println("Project admin user created successfully.")

	fmt.Printf("Creating default project '%s' in organization '%s'...\n", defaultProject, defaultOrg)
	err = TenantUtils{}.CreateProjectInOrg(ctx, defaultOrg, defaultProject)
	if err != nil {
		return fmt.Errorf("failed to create project in org: %w", err)
	}
	fmt.Println("Default project created successfully.")

	fmt.Printf("Creating edge infra users in project '%s'...\n", defaultProject)
	err = TenantUtils{}.CreateEdgeInfraUsers(ctx, defaultOrg, defaultProject, defaultProject)
	if err != nil {
		return fmt.Errorf("failed to create edge infra users: %w", err)
	}
	fmt.Println("Edge infra users created successfully.")

	fmt.Printf("Creating cluster orchestration users in project '%s'...\n", defaultProject)
	err = TenantUtils{}.CreateClusterOrchUsers(ctx, defaultOrg, defaultProject, defaultProject)
	if err != nil {
		return fmt.Errorf("failed to create cluster orch users: %w", err)
	}
	fmt.Println("Cluster orchestration users created successfully.")

	return nil
}

// ─── Tenancy-Manager REST helpers ────────────────────────────────────────────

// tmHTTPClient skips TLS certificate verification for dev/kind environments.
var tmHTTPClient = &http.Client{ //nolint:gosec
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	},
}

// tmEnsurePortForward checks whether port 18080 is already reachable on
// localhost; if not it starts a kubectl port-forward to the tenancy-manager
// service. Returns the local endpoint URL and a cleanup func to stop the
// port-forward (no-op if one was already running).
func tmEnsurePortForward() (string, func()) {
	port := 18080
	addr := fmt.Sprintf("localhost:%d", port)
	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		conn.Close()
		return fmt.Sprintf("http://localhost:%d", port), func() {}
	}
	cmd := exec.Command("kubectl", "-n", "orch-iam", "port-forward",
		"svc/tenancy-manager", fmt.Sprintf("%d:8080", port))
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		fmt.Printf("warning: failed to start port-forward to tenancy-manager: %v\n", err)
		return fmt.Sprintf("http://localhost:%d", port), func() {}
	}
	// Poll until the port accepts connections (up to 4 s).
	for i := 0; i < 20; i++ {
		time.Sleep(200 * time.Millisecond)
		if conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
			conn.Close()
			break
		}
	}
	return fmt.Sprintf("http://localhost:%d", port), func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}

// tmGetAdminToken obtains a Keycloak JWT for the Keycloak admin user via
// system-client. Always reads the password from the platform-keycloak k8s
// secret so that a stale ORCH_DEFAULT_PASSWORD env var is never used.
func tmGetAdminToken() (string, error) {
	out, err := exec.Command(
		"kubectl", "get", "secret", "platform-keycloak",
		"-n", "orch-platform",
		"-o", "jsonpath={.data.admin-password}",
	).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to read KC admin password from secret: %w\noutput: %s", err, out)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return "", fmt.Errorf("failed to decode KC admin password: %w", err)
	}
	adminPass := strings.TrimSpace(string(decoded))
	if adminPass == "" {
		return "", fmt.Errorf("empty KC admin password from platform-keycloak secret")
	}
	keycloakBase := "https://keycloak." + serviceDomainWithPort
	formData := url.Values{
		"client_id":  {"system-client"},
		"username":   {"admin"},
		"password":   {adminPass},
		"grant_type": {"password"},
		"scope":      {"openid"},
	}
	resp, err := tmHTTPClient.PostForm(
		keycloakBase+"/realms/master/protocol/openid-connect/token",
		formData,
	)
	if err != nil {
		return "", fmt.Errorf("KC token request failed: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode KC token response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("KC token error: %s: %s", result.Error, result.ErrorDesc)
	}
	return result.AccessToken, nil
}

// tmRequest sends an authenticated HTTP request to the tenancy-manager REST API.
func tmRequest(method, endpoint, path, token string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("json marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, endpoint+path, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, err
}

// tmOrgStatusDetail holds the status fields returned by the TM REST API for orgs.
type tmOrgStatusDetail struct {
	StatusIndicator string `json:"statusIndicator"`
	UID             string `json:"uID"`
	Message         string `json:"message"`
}

// tmProjectStatusDetail holds the status fields returned by the TM REST API for projects.
type tmProjectStatusDetail struct {
	StatusIndicator string `json:"statusIndicator"`
	UID             string `json:"uID"`
	Message         string `json:"message"`
}

// tmOrgResp is used to decode TM REST API responses for orgs.
type tmOrgResp struct {
	Name   string `json:"name"`
	Status struct {
		OrgStatus tmOrgStatusDetail `json:"orgStatus"`
	} `json:"status"`
}

// tmProjectResp is used to decode TM REST API responses for projects.
type tmProjectResp struct {
	Name   string `json:"name"`
	Status struct {
		ProjectStatus tmProjectStatusDetail `json:"projectStatus"`
	} `json:"status"`
}

// ─── Org operations ──────────────────────────────────────────────────────────

// CreateOrg creates an Org via the tenancy-manager REST API and waits until
// all controllers report IDLE: mage tenantUtils:createOrg <org-name>
func (TenantUtils) CreateOrg(ctx context.Context, org string) error {
	endpoint, cleanup := tmEnsurePortForward()
	defer cleanup()

	token, err := tmGetAdminToken()
	if err != nil {
		return fmt.Errorf("failed to get admin token: %w", err)
	}

	// Check if org already exists.
	orgUID, err := tmGetOrgUID(endpoint, token, org)
	if err != nil {
		return fmt.Errorf("failed to check existing org: %w", err)
	}
	if orgUID != "" {
		fmt.Printf("Org (%s) already present with UID (%s)\n", org, orgUID)
		return nil
	}

	fmt.Printf("Creating Org (%s)\n", org)
	body := map[string]interface{}{
		"spec": map[string]string{"description": org},
	}
	respBody, code, err := tmRequest(http.MethodPut, endpoint, "/v1/orgs/"+org, token, body)
	if err != nil {
		return fmt.Errorf("PUT /v1/orgs/%s failed: %w", org, err)
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return fmt.Errorf("PUT /v1/orgs/%s: HTTP %d: %s", org, code, string(respBody))
	}

	orgUID, err = waitUntilOrgCreation(ctx, endpoint, token, org)
	if err != nil {
		return fmt.Errorf("wait for org %s to go active failed: %w", org, err)
	}
	fmt.Printf("\nOrg (%s) has UID: %s\n", org, orgUID)
	return nil
}

// tmGetOrgUID returns the UID of an org by name, or "" if not found.
func tmGetOrgUID(endpoint, token, orgName string) (string, error) {
	respBody, code, err := tmRequest(http.MethodGet, endpoint, "/v1/orgs/"+orgName, token, nil)
	if err != nil {
		return "", err
	}
	if code == http.StatusNotFound {
		return "", nil
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("GET /v1/orgs/%s: HTTP %d: %s", orgName, code, string(respBody))
	}
	var resp tmOrgResp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("decode org response: %w", err)
	}
	return resp.Status.OrgStatus.UID, nil
}

// getOrgId returns the UID of an org by name, or "" if not found.
// Uses the tenancy-manager REST API.
func getOrgId(_ context.Context, orgName string) (string, error) {
	endpoint, cleanup := tmEnsurePortForward()
	defer cleanup()
	token, err := tmGetAdminToken()
	if err != nil {
		return "", fmt.Errorf("failed to get admin token: %w", err)
	}
	return tmGetOrgUID(endpoint, token, orgName)
}

// waitUntilOrgCreation polls the tenancy-manager until the org status is IDLE.
// Returns the org UID.
func waitUntilOrgCreation(ctx context.Context, endpoint, token, orgName string) (string, error) {
	fmt.Println("\nwaiting until org creation is completed")
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			respBody, code, err := tmRequest(http.MethodGet, endpoint, "/v1/orgs/"+orgName, token, nil)
			if err != nil || code != http.StatusOK {
				fmt.Printf("  polling org %s: HTTP %d\n", orgName, code)
				continue
			}
			var resp tmOrgResp
			if err := json.Unmarshal(respBody, &resp); err != nil {
				continue
			}
			s := resp.Status.OrgStatus
			fmt.Printf("  org %s status: %s (%s)\n", orgName, s.StatusIndicator, s.Message)
			if strings.Contains(s.StatusIndicator, "IDLE") {
				return s.UID, nil
			}
		case <-timeout:
			return "", fmt.Errorf("org %s creation timed out", orgName)
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// DeleteOrg deletes an Org via the tenancy-manager REST API.
func (TenantUtils) DeleteOrg(ctx context.Context, org string) error {
	endpoint, cleanup := tmEnsurePortForward()
	defer cleanup()
	token, err := tmGetAdminToken()
	if err != nil {
		return fmt.Errorf("failed to get admin token: %w", err)
	}

	orgUID, err := tmGetOrgUID(endpoint, token, org)
	if err != nil {
		return fmt.Errorf("failed to check org: %w", err)
	}
	if orgUID == "" {
		fmt.Printf("Org (%s) not present to delete. Skipping delete\n", org)
		return nil
	}

	respBody, code, err := tmRequest(http.MethodDelete, endpoint, "/v1/orgs/"+org, token, nil)
	if err != nil {
		return fmt.Errorf("DELETE /v1/orgs/%s: %w", org, err)
	}
	if code != http.StatusOK && code != http.StatusNoContent && code != http.StatusAccepted {
		return fmt.Errorf("DELETE /v1/orgs/%s: HTTP %d: %s", org, code, string(respBody))
	}

	// Poll until the org is gone.
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_, code, _ := tmRequest(http.MethodGet, endpoint, "/v1/orgs/"+org, token, nil)
			if code == http.StatusNotFound {
				fmt.Println("org deleted successfully")
				return nil
			}
		case <-timeout:
			return fmt.Errorf("org %s deletion timed out", org)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// DeleteProject deletes a Project via the tenancy-manager REST API.
func (TenantUtils) DeleteProject(ctx context.Context, org, project string) error {
	endpoint, cleanup := tmEnsurePortForward()
	defer cleanup()
	token, err := tmGetAdminToken()
	if err != nil {
		return fmt.Errorf("failed to get admin token: %w", err)
	}

	projectUID, err := tmGetProjectUID(endpoint, token, org, project)
	if err != nil {
		return fmt.Errorf("failed to check project: %w", err)
	}
	if projectUID == "" {
		fmt.Printf("Project (%s) not present to delete. Skipping delete\n", project)
		return nil
	}
	fmt.Printf("Deleting Project (%s)\n", project)

	path := fmt.Sprintf("/v1/projects/%s?org=%s", project, org)
	respBody, code, err := tmRequest(http.MethodDelete, endpoint, path, token, nil)
	if err != nil {
		return fmt.Errorf("DELETE /v1/projects/%s: %w", project, err)
	}
	if code != http.StatusOK && code != http.StatusNoContent && code != http.StatusAccepted {
		return fmt.Errorf("DELETE /v1/projects/%s: HTTP %d: %s", project, code, string(respBody))
	}

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			uid, _ := tmGetProjectUID(endpoint, token, org, project)
			if uid == "" {
				return nil
			}
		case <-timeout:
			return fmt.Errorf("project %s deletion timed out", project)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// GetOrg gets the Org status from the tenancy-manager REST API:
// mage tenantUtils:getOrg <org-name>
func (TenantUtils) GetOrg(ctx context.Context, orgName string) error {
	endpoint, cleanup := tmEnsurePortForward()
	defer cleanup()
	token, err := tmGetAdminToken()
	if err != nil {
		return fmt.Errorf("failed to get admin token: %w", err)
	}

	respBody, code, err := tmRequest(http.MethodGet, endpoint, "/v1/orgs/"+orgName, token, nil)
	if err != nil {
		return fmt.Errorf("GET /v1/orgs/%s: %w", orgName, err)
	}
	if code == http.StatusNotFound {
		fmt.Printf("org %s does not exist.\n", orgName)
		return nil
	}
	if code != http.StatusOK {
		return fmt.Errorf("GET /v1/orgs/%s: HTTP %d: %s", orgName, code, string(respBody))
	}
	var resp tmOrgResp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("decode org response: %w", err)
	}
	s := resp.Status.OrgStatus
	if strings.HasPrefix(s.Message, "Waiting for watchers") && strings.HasSuffix(s.Message, "to be deleted") {
		return fmt.Errorf("\norg (%s) is waiting for watchers to be deleted: %s", orgName, s.Message)
	}
	fmt.Printf("\nOrg status: %s, message: %s, UID: %s\n", s.StatusIndicator, s.Message, s.UID)
	return nil
}

// CreateProjectAdminInOrg creates a Project Admin in a given Org: mage tenantUtils:createProjectAdminInOrg <org-name> <project-admin-user>
func (TenantUtils) CreateProjectAdminInOrg(ctx context.Context, orgName string, projectAdminUser string) error {
	// TODO - Add a check to determine if ktc is running

	client, token, err := KeycloakLogin(ctx)
	if err != nil {
		return err
	}

	userId, orgId, err := createKeycloakUser(ctx, client, token, projectAdminUser, orgName)
	if err != nil {
		return err
	}

	groups := []string{orgId + "_Project-Manager-Group"}

	err = addUserToGroups(ctx, client, token, KeycloakRealm, groups, userId)
	if err != nil {
		fmt.Printf("error adding org roles to user %s", projectAdminUser)
		return err
	}

	return nil
}

func addUserToGroups(ctx context.Context, client *gocloak.GoCloak, token *gocloak.JWT, realm string, groupsList []string, userId string) error {
	for _, groupName := range groupsList {
		group, err := getGroup(ctx, client, token, realm, groupName)
		if err != nil {
			fmt.Printf("error fetching group %s", groupName)
			return err
		}
		err = client.AddUserToGroup(ctx, token.AccessToken, realm, userId, *group.ID)
		if err != nil {
			fmt.Printf("error adding org roles to the user %s", userId)
			return err
		}
		fmt.Printf("added user %s to group %s\n", userId, groupName)
	}

	return nil
}

func getGroup(ctx context.Context, client *gocloak.GoCloak, token *gocloak.JWT, realm string, groupName string) (*gocloak.Group, error) {
	group, err := client.GetGroupByPath(ctx, token.AccessToken, realm, groupName)
	if err != nil {
		fmt.Printf("Failed to retrieve group %s in realm %s: %v", groupName, realm, err)
		return nil, err
	}
	return group, nil
}

// Prints the default Orchestrator password.
func OrchPassword() error {
	pass, err := GetDefaultOrchPassword()
	if err != nil {
		return err
	}

	fmt.Printf("Default Orch Password: %s\n", pass)

	return nil
}

func GetDefaultOrchPassword() (string, error) {
	pass := os.Getenv("ORCH_DEFAULT_PASSWORD")
	if pass == "" {
		fmt.Println("Environment variable ORCH_DEFAULT_PASSWORD is empty, attempting to retrieve password using kubectl command...")

		output, err := exec.Command(
			"kubectl",
			"get",
			"secret",
			"platform-keycloak",
			"-n", "orch-platform",
			"-o", "jsonpath={.data.admin-password}",
		).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to get password from kubectl command: %w\noutput: %s", err, string(output))
		}
		encodedPass := string(output)

		fmt.Println("Decoding base64 password... ", encodedPass)

		decodedPass, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedPass))
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 password: %w", err)
		}

		pass = strings.TrimSpace(string(decodedPass))
		if pass == "" {
			return "", fmt.Errorf("empty password retrieved from kubectl command")
		}

		fmt.Println("Password successfully retrieved using kubectl command")
	}

	// Keycloak is configured to require: "passwordPolicy": "length(14) and digits(1) and specialChars(1) and upperCase(1) and lowerCase(1)",
	// and this variable is used in the MT setup to create the users so it must respect that policy.

	// check password length
	if len(pass) < 14 {
		return "", fmt.Errorf("password length less than 14 characters")
	}

	// check for at least one digit
	if !strings.ContainsAny(pass, "0123456789") {
		return "", fmt.Errorf("password does not contain a digit")
	}

	// check for at least one special character
	if !strings.ContainsAny(pass, "!@#$%%^&*()_+-=[]{}|;:,.<>?") {
		return "", fmt.Errorf("password does not contain a special character")
	}

	// check for at least one upper case letter
	if strings.ToLower(pass) == pass {
		return "", fmt.Errorf("password does not contain an upper case letter")
	}

	// check for at least one lower case letter
	if strings.ToUpper(pass) == pass {
		return "", fmt.Errorf("password does not contain a lower case letter")
	}

	return pass, nil
}

func GetKeycloakSecret() (string, error) {
	kubecmd := fmt.Sprintf("kubectl get secret -n %s platform-keycloak -o jsonpath='{.data.admin-password}' ", "orch-platform")
	pass, err := script.Exec(kubecmd).String()
	if err != nil {
		return "", err
	}

	// Decode the Base64 string
	encodedPass, err := base64.StdEncoding.DecodeString(pass)
	if err != nil {
		return "", fmt.Errorf("error decoding Base64 string: %w", err)
	}

	// Convert the decoded bytes to a string
	adminPass := string(encodedPass)

	if adminPass == "" {
		return "", fmt.Errorf("password string empty")
	}

	return adminPass, nil
}

func KeycloakLogin(ctx context.Context) (*gocloak.GoCloak, *gocloak.JWT, error) {
	keycloakURL := "https://keycloak." + serviceDomainWithPort

	// retrieve admin user and password from keycloak secret
	adminPass, err := GetKeycloakSecret()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get keycloak admin password")
	}

	client := gocloak.NewClient(keycloakURL)

	jwtToken, err := client.LoginAdmin(ctx, adminUser, adminPass, KeycloakRealm)
	if err != nil {
		fmt.Printf("%v", err)
		return nil, nil, fmt.Errorf("failed to login to keycloak %s", keycloakURL)
	}
	return client, jwtToken, nil
}

// ─── Project operations ───────────────────────────────────────────────────────

// CreateProjectInOrg creates a Project via the tenancy-manager REST API and
// waits until all controllers report IDLE:
// mage tenantUtils:createProjectInOrg <org-name> <project-name>
func (TenantUtils) CreateProjectInOrg(ctx context.Context, orgName string, projectName string) error {
	endpoint, cleanup := tmEnsurePortForward()
	defer cleanup()
	token, err := tmGetAdminToken()
	if err != nil {
		return fmt.Errorf("failed to get admin token: %w", err)
	}

	// Check if project already exists.
	projectUID, err := tmGetProjectUID(endpoint, token, orgName, projectName)
	if err != nil {
		return fmt.Errorf("failed to check existing project: %w", err)
	}
	if projectUID != "" {
		fmt.Printf("Project (%s) already present with UID (%s)\n", projectName, projectUID)
		return nil
	}

	fmt.Printf("Creating Project (%s)\n", projectName)
	body := map[string]interface{}{
		"spec": map[string]string{"description": projectName},
	}
	path := fmt.Sprintf("/v1/projects/%s?org=%s", projectName, orgName)
	respBody, code, err := tmRequest(http.MethodPut, endpoint, path, token, body)
	if err != nil {
		return fmt.Errorf("PUT /v1/projects/%s: %w", projectName, err)
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return fmt.Errorf("PUT /v1/projects/%s: HTTP %d: %s", projectName, code, string(respBody))
	}

	projectUID, err = waitUntilProjectCreation(ctx, endpoint, token, orgName, projectName)
	if err != nil {
		return fmt.Errorf("wait for project %s to go active failed: %w", projectName, err)
	}
	fmt.Printf("\nProject (%s) has UID: %s\n", projectName, projectUID)
	return nil
}

// tmGetProjectUID returns the UID of a project by name within the given org,
// or "" if not found.
func tmGetProjectUID(endpoint, token, orgName, projectName string) (string, error) {
	path := fmt.Sprintf("/v1/projects/%s?org=%s", projectName, orgName)
	respBody, code, err := tmRequest(http.MethodGet, endpoint, path, token, nil)
	if err != nil {
		return "", err
	}
	if code == http.StatusNotFound {
		return "", nil
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("GET /v1/projects/%s: HTTP %d: %s", projectName, code, string(respBody))
	}
	var resp tmProjectResp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("decode project response: %w", err)
	}
	return resp.Status.ProjectStatus.UID, nil
}

// waitUntilProjectCreation polls the tenancy-manager until the project status
// is IDLE. Returns the project UID.
func waitUntilProjectCreation(ctx context.Context, endpoint, token, orgName, projectName string) (string, error) {
	fmt.Println("\nwaiting until project creation is completed")
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	path := fmt.Sprintf("/v1/projects/%s?org=%s", projectName, orgName)
	for {
		select {
		case <-ticker.C:
			respBody, code, err := tmRequest(http.MethodGet, endpoint, path, token, nil)
			if err != nil || code != http.StatusOK {
				fmt.Printf("  polling project %s: HTTP %d\n", projectName, code)
				continue
			}
			var resp tmProjectResp
			if err := json.Unmarshal(respBody, &resp); err != nil {
				continue
			}
			s := resp.Status.ProjectStatus
			fmt.Printf("  project %s status: %s (%s)\n", projectName, s.StatusIndicator, s.Message)
			if strings.Contains(s.StatusIndicator, "IDLE") {
				return s.UID, nil
			}
		case <-timeout:
			return "", fmt.Errorf("project %s creation timed out", projectName)
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// WaitUntilProjectReady waits for a project to reach IDLE status:
// mage tenantUtils:waitUntilProjectReady <org-name> <project-name>
func (TenantUtils) WaitUntilProjectReady(ctx context.Context, orgName, projectName string) (string, error) {
	endpoint, cleanup := tmEnsurePortForward()
	defer cleanup()
	token, err := tmGetAdminToken()
	if err != nil {
		return "", fmt.Errorf("failed to get admin token: %w", err)
	}
	return waitUntilProjectCreation(ctx, endpoint, token, orgName, projectName)
}

// WaitUntilProjectWatchersReady waits for a project to reach IDLE status
// (all controller watchers processed):
// mage tenantUtils:waitUntilProjectWatchersReady <org-name> <project-name>
func (TenantUtils) WaitUntilProjectWatchersReady(ctx context.Context, orgName, projectName string) (string, error) {
	endpoint, cleanup := tmEnsurePortForward()
	defer cleanup()
	token, err := tmGetAdminToken()
	if err != nil {
		return "", fmt.Errorf("failed to get admin token: %w", err)
	}
	uid, err := waitUntilProjectCreation(ctx, endpoint, token, orgName, projectName)
	if err != nil {
		return "", err
	}
	fmt.Println("all watchers ready and in idle state")
	return uid, nil
}

// GetProject gets the Project status from the tenancy-manager REST API:
// mage tenantUtils:getProject <org-name> <project-name>
func (TenantUtils) GetProject(ctx context.Context, orgName, projectName string) error {
	endpoint, cleanup := tmEnsurePortForward()
	defer cleanup()
	token, err := tmGetAdminToken()
	if err != nil {
		return fmt.Errorf("failed to get admin token: %w", err)
	}

	path := fmt.Sprintf("/v1/projects/%s?org=%s", projectName, orgName)
	respBody, code, err := tmRequest(http.MethodGet, endpoint, path, token, nil)
	if err != nil {
		return fmt.Errorf("GET /v1/projects/%s: %w", projectName, err)
	}
	if code == http.StatusNotFound {
		fmt.Printf("project %s does not exist.\n", projectName)
		return nil
	}
	if code != http.StatusOK {
		return fmt.Errorf("GET /v1/projects/%s: HTTP %d: %s", projectName, code, string(respBody))
	}
	var resp tmProjectResp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("decode project response: %w", err)
	}
	s := resp.Status.ProjectStatus
	if strings.Contains(s.Message, "Waiting for watchers") {
		return fmt.Errorf("\nproject (%s) status message is set to %s", projectName, s.Message)
	}
	fmt.Printf("\nproject status: %s, message: %s, UID: %s\n", s.StatusIndicator, s.Message, s.UID)
	return nil
}

// GetProjectId gets the UID of a Project by name:
// mage tenantUtils:getProjectId <org-name> <project-name>
func GetProjectId(ctx context.Context, orgName, projectName string) (string, error) {
	endpoint, cleanup := tmEnsurePortForward()
	defer cleanup()
	token, err := tmGetAdminToken()
	if err != nil {
		return "", fmt.Errorf("failed to get admin token: %w", err)
	}
	return tmGetProjectUID(endpoint, token, orgName, projectName)
}

// ─── Keycloak user creation helpers ──────────────────────────────────────────

// CreateEdgeInfraUsers creates Edge Infra Manager users in a given Project:
// mage tenantUtils:createEdgeInfraUser <org-name> <project-name> <edge-infra-user-prefix>
func (TenantUtils) CreateEdgeInfraUsers(ctx context.Context, orgName, projectName, edgeInfraUserPrefix string) error {
	// TODO - Add a check to determine if ktc is running

	client, token, err := KeycloakLogin(ctx)
	if err != nil {
		return err
	}

	projectId, err := GetProjectId(ctx, orgName, projectName)
	if projectId == "" || err != nil {
		return fmt.Errorf("error retrieving project %s. Error: %w", projectName, err)
	}

	// Create Edge Infra Manager NB API user
	user := edgeInfraUserPrefix + "-api-user"
	userId, orgId, err := createKeycloakUser(ctx, client, token, user, orgName)
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return fmt.Errorf("error creating Keycloak user %s. Error: %w", user, err)
	}
	if status.Code(err) != codes.AlreadyExists {
		groups := []string{projectId + "_Host-Manager-Group"}

		err = addUserToGroups(ctx, client, token, KeycloakRealm, groups, userId)
		if err != nil {
			return fmt.Errorf("error adding org roles to user %s. Error: %w", user, err)
		}
		err = addProjectMemberRole(ctx, client, token, KeycloakRealm, orgId, projectId, userId)
		if err != nil {
			return fmt.Errorf("error adding member role to user %s. Error: %w", user, err)
		}
		if err = addProjectServiceRoles(ctx, client, token, KeycloakRealm, projectId, userId); err != nil {
			return fmt.Errorf("error adding service roles to user %s. Error: %w", user, err)
		}
	}

	// Create EN agent user
	user = edgeInfraUserPrefix + "-en-svc-account"
	userId, _, err = createKeycloakUser(ctx, client, token, user, orgName)
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return fmt.Errorf("error creating Keycloak user %s. Error: %w", user, err)
	}
	if status.Code(err) != codes.AlreadyExists {
		groups := []string{projectId + "_Edge-Node-M2M-Service-Account"}

		err = addUserToGroups(ctx, client, token, KeycloakRealm, groups, userId)
		if err != nil {
			return fmt.Errorf("error adding org roles to user %s. Error: %w", user, err)
		}
		err = addProjectMemberRole(ctx, client, token, KeycloakRealm, orgId, projectId, userId)
		if err != nil {
			return fmt.Errorf("error adding member role to user %s. Error: %w", user, err)
		}
		if err = addProjectServiceRoles(ctx, client, token, KeycloakRealm, projectId, userId); err != nil {
			return fmt.Errorf("error adding service roles to user %s. Error: %w", user, err)
		}
	}

	// Create EN onboarding user
	user = edgeInfraUserPrefix + "-onboarding-user"
	userId, _, err = createKeycloakUser(ctx, client, token, user, orgName)
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return fmt.Errorf("error creating Keycloak user %s. Error: %w", user, err)
	}
	if status.Code(err) != codes.AlreadyExists {
		groups := []string{projectId + "_Edge-Onboarding-Group"}

		err = addUserToGroups(ctx, client, token, KeycloakRealm, groups, userId)
		if err != nil {
			return fmt.Errorf("error adding org roles to user %s. Error: %w", user, err)
		}
		err = addProjectMemberRole(ctx, client, token, KeycloakRealm, orgId, projectId, userId)
		if err != nil {
			return fmt.Errorf("error adding member role to user %s. Error: %w", user, err)
		}
		if err = addProjectServiceRoles(ctx, client, token, KeycloakRealm, projectId, userId); err != nil {
			return fmt.Errorf("error adding service roles to user %s. Error: %w", user, err)
		}
	}

	// Create Edge Infra Manager NB API user with service-admin which is needed for observability-admin access
	user = edgeInfraUserPrefix + "-service-admin-api-user"
	userId, orgId, err = createKeycloakUser(ctx, client, token, user, orgName)
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return fmt.Errorf("error creating Keycloak user %s. Error: %w", user, err)
	}
	if status.Code(err) != codes.AlreadyExists {
		groups := []string{projectId + "_Host-Manager-Group", "service-admin-group"}

		err = addUserToGroups(ctx, client, token, KeycloakRealm, groups, userId)
		if err != nil {
			return fmt.Errorf("error adding org roles to user %s. Error: %w", user, err)
		}
		err = addProjectMemberRole(ctx, client, token, KeycloakRealm, orgId, projectId, userId)
		if err != nil {
			return fmt.Errorf("error adding member role to user %s. Error: %w", user, err)
		}
		if err = addProjectServiceRoles(ctx, client, token, KeycloakRealm, projectId, userId); err != nil {
			return fmt.Errorf("error adding service roles to user %s. Error: %w", user, err)
		}
	}

	return nil
}

// CreateClusterOrchUsers creates Cluster Orch users in a given Project: mage tenantUtils:createClusterOrchUsers <org-name> <project-name> <co-user-prefix>
func (TenantUtils) CreateClusterOrchUsers(ctx context.Context, orgName, projectName, coUserPrefix string) error {
	// TODO - Add a check to determine if ktc is running

	client, token, err := KeycloakLogin(ctx)
	if err != nil {
		return err
	}

	projectId, err := GetProjectId(ctx, orgName, projectName)
	if projectId == "" || err != nil {
		return fmt.Errorf("error retrieving project %s. Error: %w", projectName, err)
	}

	// Create Edge operator user
	user := coUserPrefix + "-edge-op"
	userId, orgId, err := createKeycloakUser(ctx, client, token, user, orgName)
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return fmt.Errorf("error creating Keycloak user %s. Error: %w", user, err)
	}
	if status.Code(err) != codes.AlreadyExists {
		groups := []string{projectId + "_Edge-Operator-Group"}

		err = addUserToGroups(ctx, client, token, KeycloakRealm, groups, userId)
		if err != nil {
			return fmt.Errorf("error adding org roles to user %s. Error: %w", user, err)
		}
		err = addProjectMemberRole(ctx, client, token, KeycloakRealm, orgId, projectId, userId)
		if err != nil {
			return fmt.Errorf("error adding member role to user %s. Error: %w", user, err)
		}
		if err = addProjectServiceRoles(ctx, client, token, KeycloakRealm, projectId, userId); err != nil {
			return fmt.Errorf("error adding service roles to user %s. Error: %w", user, err)
		}
	}

	// Create Edge Manager user
	user = coUserPrefix + "-edge-mgr"
	userId, _, err = createKeycloakUser(ctx, client, token, user, orgName)
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return fmt.Errorf("error creating Keycloak user %s. Error: %w", user, err)
	}
	if status.Code(err) != codes.AlreadyExists {
		groups := []string{projectId + "_Edge-Manager-Group"}

		err = addUserToGroups(ctx, client, token, KeycloakRealm, groups, userId)
		if err != nil {
			return fmt.Errorf("error adding org roles to user %s. Error: %w", user, err)
		}
		err = addProjectMemberRole(ctx, client, token, KeycloakRealm, orgId, projectId, userId)
		if err != nil {
			return fmt.Errorf("error adding member role to user %s. Error: %w", user, err)
		}
		if err = addProjectServiceRoles(ctx, client, token, KeycloakRealm, projectId, userId); err != nil {
			return fmt.Errorf("error adding service roles to user %s. Error: %w", user, err)
		}
	}

	return nil
}

func CreateDefaultKeyCloakUser(ctx context.Context, client *gocloak.GoCloak, token *gocloak.JWT, user *gocloak.User, orgName string) (string, error) {
	userName := strings.ToLower(*user.Username)
	params := gocloak.GetUsersParams{
		Username: &userName,
	}

	users, err := client.GetUsers(ctx, token.AccessToken, KeycloakRealm, params)
	if err != nil {
		fmt.Printf("error getting user %s: %v", userName, err)
		return "", err
	}
	for _, user = range users {
		if *user.Username == userName {
			return "", status.Errorf(codes.AlreadyExists, "user %s already found in realm %s", userName, KeycloakRealm)
		}
	}

	user = &gocloak.User{
		Username:      &userName,
		Email:         gocloak.StringP(userName + "@" + orgName + ".com"),
		Enabled:       gocloak.BoolP(true),
		EmailVerified: gocloak.BoolP(true),
		FirstName:     user.FirstName,
		LastName:      user.LastName,
	}

	userId, err := client.CreateUser(ctx, token.AccessToken, KeycloakRealm, *user)
	if err != nil {
		fmt.Printf("error creating user %s", userName)
		return "", err
	}

	defaultOrchPass, err := GetDefaultOrchPassword()
	if err != nil {
		fmt.Printf("error getting default orch password %v", err)
		return "", err
	}

	err = client.SetPassword(ctx, token.AccessToken, userId, KeycloakRealm, defaultOrchPass, false)
	if err != nil {
		fmt.Printf("error setting password for user %s", userName)
		return "", err
	}

	fmt.Printf("created user %s\n", userName)
	return userId, nil
}

func createTenancyUser(ctx context.Context, client *gocloak.GoCloak, token *gocloak.JWT, tenancyUser string) (string, error) {
	user := &gocloak.User{
		Username:      &tenancyUser,
		Email:         gocloak.StringP(tenancyUser + "@tenancy-admin.com"),
		Enabled:       gocloak.BoolP(true),
		EmailVerified: gocloak.BoolP(true),
	}

	userId, err := client.CreateUser(ctx, token.AccessToken, KeycloakRealm, *user)
	if err != nil {
		fmt.Printf("error creating user %s", tenancyUser)
		return "", err
	}

	defaultOrchPass, err := GetDefaultOrchPassword()
	if err != nil {
		fmt.Printf("error getting default orch password %v", err)
		return "", err
	}

	err = client.SetPassword(ctx, token.AccessToken, userId, KeycloakRealm, defaultOrchPass, false)
	if err != nil {
		fmt.Printf("error setting password for user %s", tenancyUser)
		return "", err
	}

	fmt.Printf("created user %s\n", tenancyUser)
	return userId, nil
}

func deleteTenancyUser(ctx context.Context, client *gocloak.GoCloak, token *gocloak.JWT, userId string) error {
	err := client.DeleteUser(ctx, token.AccessToken, KeycloakRealm, userId)
	if err != nil {
		fmt.Printf("error deleting user with id  %s", userId)
		return err
	}
	fmt.Printf("deleted user with id - %s\n", userId)
	return nil
}

func createKeycloakUser(ctx context.Context, client *gocloak.GoCloak, token *gocloak.JWT, edgeInfraUser string, orgName string) (string, string, error) {
	var user *gocloak.User
	edgeInfraUser = strings.ToLower(edgeInfraUser)
	params := gocloak.GetUsersParams{
		Username: &edgeInfraUser,
	}

	users, err := client.GetUsers(ctx, token.AccessToken, KeycloakRealm, params)
	if err != nil {
		fmt.Printf("error getting user %s: %v", edgeInfraUser, err)
		return "", "", err
	}
	for _, user = range users {
		if *user.Username == edgeInfraUser {
			return "", "", status.Errorf(codes.AlreadyExists, "user %s already found in realm %s", edgeInfraUser, KeycloakRealm)
		}
	}

	orgId, _ := getOrgId(ctx, orgName)
	if orgId == "" {
		return "", "", err
	}

	user = &gocloak.User{
		Username:      &edgeInfraUser,
		Email:         gocloak.StringP(edgeInfraUser + "@" + orgName + ".com"),
		FirstName:     &edgeInfraUser,
		LastName:      &edgeInfraUser,
		Enabled:       gocloak.BoolP(true),
		EmailVerified: gocloak.BoolP(true),
	}

	userId, err := client.CreateUser(ctx, token.AccessToken, KeycloakRealm, *user)
	if err != nil {
		fmt.Printf("error creating user %s", edgeInfraUser)
		return "", "", err
	}

	defaultOrchPass, err := GetDefaultOrchPassword()
	if err != nil {
		fmt.Printf("error getting default orch password %v", err)
		return "", "", err
	}

	err = client.SetPassword(ctx, token.AccessToken, userId, KeycloakRealm, defaultOrchPass, false)
	if err != nil {
		fmt.Printf("error setting password for user %s", edgeInfraUser)
		return "", "", err
	}

	fmt.Printf("created user %s\n", edgeInfraUser)
	return userId, orgId, nil
}

func addProjectMemberRole(ctx context.Context, client *gocloak.GoCloak, token *gocloak.JWT, realm, orgId, projectId, userId string) error {
	var roles []gocloak.Role
	prefixedRole := fmt.Sprintf("%s_%s_%s", orgId, projectId, "m")
	role, err := getRealmRole(ctx, client, token, realm, prefixedRole)
	if err != nil {
		return err
	}
	roles = append(roles, *role)

	err = client.AddRealmRoleToUser(ctx, token.AccessToken, realm, userId, roles)
	if err != nil {
		return err
	}
	fmt.Printf("added member roles to the user %s\n", userId)
	return nil
}

// addProjectServiceRoles assigns project-scoped service roles to a user so
// that alerting-monitor (OPA: {projectId}_alrt-r) and cluster-manager
// (Activeprojectid header check) authorize requests correctly.
// Roles are created by the Keycloak Tenant Controller when the project is
// provisioned; this function assigns them to each user that needs API access.
// A missing role is treated as a warning (not a fatal error) because some
// roles are optional depending on which components are deployed.
func addProjectServiceRoles(ctx context.Context, client *gocloak.GoCloak, token *gocloak.JWT, realm, projectId, userId string) error {
	serviceRoleSuffixes := []string{
		"alrt-r", "alrt-rw",
		"cl-r", "cl-rw",
		"cl-tpl-r", "cl-tpl-rw",
	}

	var roles []gocloak.Role
	for _, suffix := range serviceRoleSuffixes {
		roleName := fmt.Sprintf("%s_%s", projectId, suffix)
		role, err := getRealmRole(ctx, client, token, realm, roleName)
		if err != nil {
			// Role may not exist if the component is disabled; skip silently.
			fmt.Printf("warning: realm role %q not found, skipping: %v\n", roleName, err)
			continue
		}
		roles = append(roles, *role)
	}

	if len(roles) == 0 {
		fmt.Printf("warning: no project service roles found for project %s — KTC may not have created them yet\n", projectId)
		return nil
	}

	if err := client.AddRealmRoleToUser(ctx, token.AccessToken, realm, userId, roles); err != nil {
		return fmt.Errorf("adding service roles to user %s: %w", userId, err)
	}
	fmt.Printf("added project service roles to user (project: %s)\n", projectId)
	return nil
}

func getRealmRole(ctx context.Context, client *gocloak.GoCloak, token *gocloak.JWT, realm string, role string) (*gocloak.Role, error) {
	realmRole, err := client.GetRealmRole(ctx, token.AccessToken, realm, role)
	if err != nil {
		return nil, err
	}
	return realmRole, nil
}

// getEdgeAndApiUsers retrieves the Edge Infra Manager and API users for a given organization
func getEdgeAndApiUsers(ctx context.Context, orgName string) (string, string, error) {
	var user *gocloak.User
	params := gocloak.GetUsersParams{}
	edgeInfraUser := ""
	apiUser := ""

	client, token, err := KeycloakLogin(ctx)
	if err != nil {
		return edgeInfraUser, apiUser, err
	}

	users, err := client.GetUsers(ctx, token.AccessToken, KeycloakRealm, params)
	if err != nil {
		fmt.Printf("error getting user %v", err)
		return edgeInfraUser, apiUser, err
	}

	// Loop through the users to find Edge Infra Manager and API users
	for _, user = range users {
		// Check if the user is an Edge Infra Manager
		if strings.Contains(*user.Username, "-edge-mgr") {
			usernameSplit := strings.Split(*user.Email, "@")
			if len(usernameSplit) == 0 {
				return edgeInfraUser, apiUser, fmt.Errorf("unable to get user name from user email %s", *user.Email)
			}
			// Ensure the email domain matches the organization name
			if usernameSplit[1] == orgName+".com" {
				edgeInfraUser = *user.Username
			}
		}
		if strings.Contains(*user.Username, "-api-user") && !strings.Contains(*user.Username, "-service-admin") {
			usernameSplit := strings.Split(*user.Email, "@")
			if len(usernameSplit) == 0 {
				return edgeInfraUser, apiUser, fmt.Errorf("unable to get user name from user email %s", *user.Email)
			}
			if usernameSplit[1] == orgName+".com" {
				apiUser = *user.Username
			}
		}
	}

	fmt.Printf("Found Edge Infra User: %s, API User: %s\n", edgeInfraUser, apiUser)
	return edgeInfraUser, apiUser, nil
}
