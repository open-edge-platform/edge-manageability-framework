// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

// CreateOrg creates an Org in the system: mage tenantUtils:createOrg <org-name>
func (TenantUtils) CreateOrg(ctx context.Context, org string) error {
	client, err := newTenancyRESTClient(ctx)
	if err != nil {
		return fmt.Errorf("\nerror creating org (%s). Error: %w", org, err)
	}
	if uid, _ := lookupOrgUID(ctx, client, org); uid != "" {
		fmt.Printf("Org (%s) already present with UID (%s)\n", org, uid)
		return nil
	}
	fmt.Printf("Creating Org (%s)\n", org)
	if err := client.CreateOrg(ctx, org, org); err != nil {
		return fmt.Errorf("\nerror creating org (%s). Error: %w", org, err)
	}
	orgUUID, err := client.waitUntilOrgIdle(ctx, org)
	if err != nil {
		return fmt.Errorf("wait for org %s to go active failed with error %w", org, err)
	}
	fmt.Printf("\nOrg (%s) has UID: %s\n", org, orgUUID)
	return nil
}

// DeleteOrg deletes an Org in the system
func (TenantUtils) DeleteOrg(ctx context.Context, org string) error {
	client, err := newTenancyRESTClient(ctx)
	if err != nil {
		return fmt.Errorf("\nerror deleting org (%s). Error: %w", org, err)
	}
	if uid, _ := lookupOrgUID(ctx, client, org); uid == "" {
		fmt.Printf("Org (%s) not present to delete. Skipping delete \n", org)
		return nil
	}
	if err := client.DeleteOrg(ctx, org); err != nil {
		return fmt.Errorf("\nerror deleting org (%s). Error: %w", org, err)
	}
	if err := client.waitUntilOrgGone(ctx, org); err != nil {
		return err
	}
	fmt.Println("org not found, deleted successfully")
	return nil
}

// DeleteProject deletes a Project in the system
func (TenantUtils) DeleteProject(ctx context.Context, org, project string) error {
	client, err := newTenancyRESTClient(ctx)
	if err != nil {
		return fmt.Errorf("\nerror deleting project (%s). Error: %w", project, err)
	}
	if uid, _ := lookupProjectUID(ctx, client, project); uid == "" {
		fmt.Printf("Project (%s) not present to delete. Skipping delete \n", project)
		return nil
	}
	fmt.Printf("Deleting Project (%s)\n", project)
	if err := client.DeleteProject(ctx, project); err != nil {
		return fmt.Errorf("\nerror deleting project (%s). Error: %w", project, err)
	}
	return client.waitUntilProjectGone(ctx, project)
}

// lookupOrgUID returns the org UID, or "" with errTenancyNotFound when absent.
func lookupOrgUID(ctx context.Context, client *tenancyRESTClient, orgName string) (string, error) {
	org, err := client.GetOrg(ctx, orgName)
	if isTenancyNotFound(err) {
		return "", err
	}
	if err != nil {
		return "", fmt.Errorf("\nerror checking if the org (%s) already present: Error: %w", orgName, err)
	}
	return org.Status.OrgStatus.UID, nil
}

func getOrgId(ctx context.Context, orgName string) (string, error) {
	client, err := newTenancyRESTClient(ctx)
	if err != nil {
		return "", fmt.Errorf("\nerror checking if the org (%s) already present. Error: %w", orgName, err)
	}
	uid, err := lookupOrgUID(ctx, client, orgName)
	if isTenancyNotFound(err) {
		fmt.Printf("org %s does not exist.\n", orgName)
		return "", nil
	}
	return uid, err
}

// GetOrg gets the Org in the system: mage tenantUtils:getOrg <org-name>
func (TenantUtils) GetOrg(ctx context.Context, orgName string) error {
	client, err := newTenancyRESTClient(ctx)
	if err != nil {
		return fmt.Errorf("\nerror retrieving the org (%s). Error: %w", orgName, err)
	}
	org, err := client.GetOrg(ctx, orgName)
	if isTenancyNotFound(err) {
		return err
	}
	if err != nil {
		return fmt.Errorf("\nerror retrieving the org (%s). Error: %w", orgName, err)
	}
	orgStatus := org.Status.OrgStatus
	if strings.HasPrefix(orgStatus.Message, "Waiting for watchers") && strings.HasSuffix(orgStatus.Message, "to be deleted") {
		return fmt.Errorf("\norg (%s) is waiting for watchers to be deleted with status message '%s'", orgName, orgStatus.Message)
	}
	if orgStatus.StatusIndicator != statusIndicationIdle {
		return fmt.Errorf("\norg (%s) status indicator is %s, message: %s", orgName, orgStatus.StatusIndicator, orgStatus.Message)
	}
	fmt.Printf("\nOrg status message - %s, Org status - %s\n", orgStatus.Message, orgStatus.StatusIndicator)
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
	if !strings.ContainsAny(pass, "!@#$%^&*()_+-=[]{}|;:,.<>?") {
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

// CreateProjectInOrg creates a Project in a given Org: mage tenantUtils:createProjectInOrg <org-name> <project-name>
func (TenantUtils) CreateProjectInOrg(ctx context.Context, orgName string, projectName string) error {
	client, err := newTenancyRESTClient(ctx)
	if err != nil {
		return fmt.Errorf("\nerror creating project (%s). Error: %w", projectName, err)
	}
	if _, err := lookupOrgUID(ctx, client, orgName); err != nil {
		if isTenancyNotFound(err) {
			fmt.Printf("Org (%s) not found. Please create an org first\n", orgName)
			return nil
		}
		return fmt.Errorf("\nerror creating project (%s). Error: %w", projectName, err)
	}
	fmt.Printf("Creating Project (%s)\n", projectName)
	if err := client.CreateProject(ctx, projectName, projectName); err != nil {
		return fmt.Errorf("\nerror creating project (%s). Error: %w", projectName, err)
	}
	projectUUID, err := client.waitUntilProjectIdle(ctx, projectName)
	if err != nil {
		return fmt.Errorf("wait for project %s to go active failed with error %w", projectName, err)
	}
	fmt.Printf("\nProject (%s) has UID: %s\n", projectName, projectUUID)
	return nil
}

func (TenantUtils) WaitUntilProjectReady(ctx context.Context, orgName, projectName string) (string, error) {
	client, err := newTenancyRESTClient(ctx)
	if err != nil {
		return "", fmt.Errorf("\nerror creating tenancy REST client (%s). Error: %w", projectName, err)
	}
	return client.waitUntilProjectIdle(ctx, projectName)
}

func (TenantUtils) WaitUntilProjectWatchersReady(ctx context.Context, orgName, projectName string) (string, error) {
	// In the new tenancy-manager REST architecture, the project's STATUS_INDICATION_IDLE
	// is reported only after all controllers have completed their work, so a separate
	// active-watcher poll is no longer required.
	client, err := newTenancyRESTClient(ctx)
	if err != nil {
		return "", fmt.Errorf("\nerror creating tenancy REST client (%s). Error: %w", projectName, err)
	}
	if _, err := client.waitUntilProjectIdle(ctx, projectName); err != nil {
		return "", err
	}
	return "all watchers ready and in idle state", nil
}

// GetProject gets the Project in the system: mage tenantUtils:getProject <org-name> <project-name>
func (TenantUtils) GetProject(ctx context.Context, orgName, projectName string) error {
	client, err := newTenancyRESTClient(ctx)
	if err != nil {
		return fmt.Errorf("\nerror retrieving the project (%s). Error: %w", projectName, err)
	}
	proj, err := client.GetProject(ctx, projectName)
	if isTenancyNotFound(err) {
		return err
	}
	if err != nil {
		return fmt.Errorf("\nerror retrieving the project (%s). Error: %w", projectName, err)
	}
	projectStatus := proj.Status.ProjectStatus
	if strings.Contains(projectStatus.Message, "Waiting for watchers") {
		return fmt.Errorf("\nproject (%s) status message is set to %s", projectName, projectStatus.Message)
	}
	fmt.Printf("\nproject status message - %s, project status - %s\n", projectStatus.Message, projectStatus.StatusIndicator)
	println("projectId: ", projectStatus.UID)
	return nil
}

// lookupProjectUID returns the project UID, or "" with errTenancyNotFound when absent.
func lookupProjectUID(ctx context.Context, client *tenancyRESTClient, projectName string) (string, error) {
	proj, err := client.GetProject(ctx, projectName)
	if isTenancyNotFound(err) {
		return "", err
	}
	if err != nil {
		return "", err
	}
	return proj.Status.ProjectStatus.UID, nil
}

func GetProjectId(ctx context.Context, orgName, projectName string) (string, error) {
	client, err := newTenancyRESTClient(ctx)
	if err != nil {
		return "", fmt.Errorf("\nerror retrieving the project (%s). Error: %w", projectName, err)
	}
	uid, err := lookupProjectUID(ctx, client, projectName)
	if isTenancyNotFound(err) {
		fmt.Printf("project %s does not exist.\n", projectName)
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("\nerror retrieving the project (%s). Error: %w", projectName, err)
	}
	return uid, nil
}

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
