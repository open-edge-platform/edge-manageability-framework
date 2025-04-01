// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	util "github.com/open-edge-platform/edge-manageability-framework/mage"
)

// PrepareOrgAndProject is preparing org and project for o11y testing.
// Username and password for the user with permission for org creation is required.
// Returns: orgUID, projectUID, error.
func PrepareOrgAndProject(ctx context.Context, cli *http.Client, orgName, projName, serviceDomainWithPort, username, password string,
) (orgUID string, projUID string, err error) {
	orgDesc := []byte(`{ "description":
				"Tenancy Alerts Test Organization"
			}`)

	token, err := util.GetApiToken(cli, username, password) //nolint:contextcheck,nolintlint // False-positive
	if err != nil {
		return "", "", err
	}

	resp, err := makeAuthorizedRequest(cli, http.MethodPut, "https://api."+serviceDomainWithPort+"/v1/orgs/"+orgName, *token, orgDesc)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("creating org failed with status %v", resp.StatusCode)
	}

	orgUID, err = waitForOrgToBeActive(ctx, cli, orgName, *token, serviceDomainWithPort)
	if err != nil {
		return "", "", err
	}

	projDesc := []byte(`{ "description": "Tenancy Alerts Test Project" }`)

	err = AddUserToGroup(ctx, username, fmt.Sprintf("%v_Project-Manager-Group", orgUID))
	if err != nil {
		return "", "", err
	}

	token, err = util.GetApiToken(cli, username, password) //nolint:contextcheck,nolintlint // False-positive
	if err != nil {
		return "", "", err
	}

	resp, err = makeAuthorizedRequest(cli, http.MethodPut, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, *token, projDesc)
	if err != nil {
		return "", "", err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("creating project failed with status %v", resp.StatusCode)
	}

	projUID, err = waitForProjectToBeActive(ctx, cli, projName, *token, serviceDomainWithPort)
	if err != nil {
		return "", "", err
	}

	return orgUID, projUID, nil
}

func DeleteOrgAndProject(ctx context.Context, cli *http.Client, orgName, projName, token, serviceDomainWithPort string) error {
	resp, err := makeAuthorizedRequest(cli, http.MethodDelete, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, token, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("project deletion request failed with status %v", resp.StatusCode)
	}

	err = waitForProjectToBeDeleted(ctx, cli, projName, token, serviceDomainWithPort)
	if err != nil {
		return err
	}

	resp, err = makeAuthorizedRequest(cli, http.MethodDelete, "https://api."+serviceDomainWithPort+"/v1/orgs/"+orgName, token, nil)
	if err != nil {
		return fmt.Errorf("failed to delete organization: %s with error: %w", orgName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete org: %s with StatusCode: %d", orgName, resp.StatusCode)
	}

	return waitForOrgToBeDeleted(ctx, cli, orgName, token, serviceDomainWithPort)
}

func makeAuthorizedRequest(cli *http.Client, method, url, token string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	return cli.Do(req)
}

func ParseJSONBody(body io.ReadCloser, target any) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("failed to read body %w", err)
	}

	err = json.Unmarshal(data, target)
	if err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}
	return nil
}

func waitForOrgToBeActive(parentCtx context.Context, cli *http.Client, orgName, token, serviceDomainWithPort string) (string, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return "", errors.New("timeout for waiting for org to be active exceeded")
		default:
			resp, err := makeAuthorizedRequest(cli, http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/orgs/"+orgName, token, nil)
			if err != nil {
				return "", fmt.Errorf("failed to get organization: %s with error: %w", orgName, err)
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return "", fmt.Errorf("failed to get organization: %s with StatusCode: %d", orgName, resp.StatusCode)
			}

			org := new(orgs)
			err = ParseJSONBody(resp.Body, org)
			resp.Body.Close()
			if err != nil {
				return "", fmt.Errorf("failed to parse organization: %s with error: %w", orgName, err)
			}

			if org.Status.OrgStatus.StatusIndicator == "STATUS_INDICATION_IDLE" {
				orgUID := org.Status.OrgStatus.UID
				if orgUID == "" {
					return "", fmt.Errorf("orgUID is empty for organization: %s with status: %v", orgName, org.Status)
				}
				return orgUID, nil
			}

			time.Sleep(20 * time.Second)
		}
	}
}

func waitForProjectToBeActive(parentCtx context.Context, cli *http.Client, projName, token, serviceDomainWithPort string) (string, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return "", errors.New("timeout for waiting for project to be active exceeded")
		default:
			resp, err := makeAuthorizedRequest(cli, http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, token, nil)
			if err != nil {
				return "", fmt.Errorf("failed to get project: %s with error: %w", projName, err)
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return "", fmt.Errorf("failed to get project: %s with StatusCode: %d", projName, resp.StatusCode)
			}

			project := new(projects)
			err = ParseJSONBody(resp.Body, project)
			resp.Body.Close()
			if err != nil {
				return "", fmt.Errorf("failed to parse project: %s with error: %w", projName, err)
			}

			if project.Status.ProjectStatus.StatusIndicator == "STATUS_INDICATION_IDLE" {
				projUID := project.Status.ProjectStatus.UID
				if projUID == "" {
					return "", fmt.Errorf("projUID is empty for project: %s with status: %v", projName, project.Status)
				}
				return projUID, nil
			}

			time.Sleep(20 * time.Second)
		}
	}
}

func waitForProjectToBeDeleted(parentCtx context.Context, cli *http.Client, projName, token, serviceDomainWithPort string) error {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Minute)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return errors.New("timeout for waiting for project to be deleted exceeded")
		default:
			resp, err := makeAuthorizedRequest(cli, http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/projects/"+projName, token, nil)
			if err != nil {
				return fmt.Errorf("failed to get project: %s with error: %w", projName, err)
			}

			if resp.StatusCode == http.StatusNotFound {
				resp.Body.Close()
				return nil
			}
			resp.Body.Close()
			time.Sleep(20 * time.Second)
		}
	}
}

func waitForOrgToBeDeleted(parentCtx context.Context, cli *http.Client, orgName, token, serviceDomainWithPort string) error {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Minute)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return errors.New("timeout for waiting for org to be deleted exceeded")
		default:
			resp, err := makeAuthorizedRequest(cli, http.MethodGet, "https://api."+serviceDomainWithPort+"/v1/orgs/"+orgName, token, nil)
			if err != nil {
				return fmt.Errorf("failed to get organization: %s with error: %w", orgName, err)
			}

			if resp.StatusCode == http.StatusNotFound {
				resp.Body.Close()
				return nil
			}
			resp.Body.Close()
			time.Sleep(20 * time.Second)
		}
	}
}
