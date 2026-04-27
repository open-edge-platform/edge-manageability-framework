// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// tenancyRESTClient is a thin HTTP client for the new tenancy-manager REST API
// (replaces the removed Nexus CRD-based tenancy-datamodel SDK).
//
// The base URL is derived from serviceDomain. Authentication uses an admin JWT
// obtained via Resource Owner Password Credentials against Keycloak.
type tenancyRESTClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// statusIndicationIdle is the value returned by tenancy-manager status endpoints
// once an org/project has finished provisioning.
const statusIndicationIdle = "STATUS_INDICATION_IDLE"

// errTenancyNotFound is returned by Get* helpers when the API responds 404.
var errTenancyNotFound = errors.New("tenancy resource not found")

// isTenancyNotFound reports whether err comes from a 404 response.
func isTenancyNotFound(err error) bool { return errors.Is(err, errTenancyNotFound) }

// orgStatusBlock mirrors the JSON shape returned by GET /v1/orgs/{name}.
type orgStatusBlock struct {
	Status struct {
		OrgStatus struct {
			StatusIndicator string `json:"statusIndicator"`
			Message         string `json:"message"`
			UID             string `json:"UID"`
		} `json:"orgStatus"`
	} `json:"status"`
}

// projectStatusBlock mirrors the JSON shape returned by GET /v1/projects/{name}.
type projectStatusBlock struct {
	Status struct {
		ProjectStatus struct {
			StatusIndicator string `json:"statusIndicator"`
			Message         string `json:"message"`
			UID             string `json:"UID"`
		} `json:"projectStatus"`
	} `json:"status"`
}

// newTenancyRESTClient builds a client authenticated as the Keycloak realm admin.
// The admin user has the org-* / project-* roles required by tenancy-manager's
// JWT validator in the master realm.
func newTenancyRESTClient(_ context.Context) (*tenancyRESTClient, error) {
	cli, err := GetClient()
	if err != nil {
		return nil, fmt.Errorf("tenancy REST: build http client: %w", err)
	}
	adminPass, err := GetKeycloakSecret()
	if err != nil {
		return nil, fmt.Errorf("tenancy REST: get keycloak admin password: %w", err)
	}
	tokenPtr, err := GetApiToken(cli, adminUser, adminPass)
	if err != nil {
		return nil, fmt.Errorf("tenancy REST: get admin api token: %w", err)
	}
	return &tenancyRESTClient{
		httpClient: cli,
		baseURL:    "https://api." + serviceDomainWithPort,
		token:      *tokenPtr,
	}, nil
}

func (c *tenancyRESTClient) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	url := c.baseURL + path
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func drain(resp *http.Response) {
	if resp == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// CreateOrg PUTs /v1/orgs/{name}. Returns nil on 200 or 409 (already exists).
func (c *tenancyRESTClient) CreateOrg(ctx context.Context, name, description string) error {
	body, _ := json.Marshal(map[string]string{"description": description})
	resp, err := c.do(ctx, http.MethodPut, "/v1/orgs/"+name, body)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
		return nil
	}
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("create org %s: status %d: %s", name, resp.StatusCode, string(b))
}

// GetOrg returns the parsed status block. Returns errTenancyNotFound on 404.
func (c *tenancyRESTClient) GetOrg(ctx context.Context, name string) (*orgStatusBlock, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/orgs/"+name, nil)
	if err != nil {
		return nil, err
	}
	defer drain(resp)
	if resp.StatusCode == http.StatusNotFound {
		return nil, errTenancyNotFound
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get org %s: status %d: %s", name, resp.StatusCode, string(b))
	}
	out := new(orgStatusBlock)
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, fmt.Errorf("get org %s: decode: %w", name, err)
	}
	return out, nil
}

// DeleteOrg DELETEs /v1/orgs/{name}. 404 is treated as success.
func (c *tenancyRESTClient) DeleteOrg(ctx context.Context, name string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/v1/orgs/"+name, nil)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("delete org %s: status %d: %s", name, resp.StatusCode, string(b))
}

// CreateProject PUTs /v1/projects/{name}.
func (c *tenancyRESTClient) CreateProject(ctx context.Context, name, description string) error {
	body, _ := json.Marshal(map[string]string{"description": description})
	resp, err := c.do(ctx, http.MethodPut, "/v1/projects/"+name, body)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
		return nil
	}
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("create project %s: status %d: %s", name, resp.StatusCode, string(b))
}

// GetProject returns the parsed status block.
func (c *tenancyRESTClient) GetProject(ctx context.Context, name string) (*projectStatusBlock, error) {
	resp, err := c.do(ctx, http.MethodGet, "/v1/projects/"+name, nil)
	if err != nil {
		return nil, err
	}
	defer drain(resp)
	if resp.StatusCode == http.StatusNotFound {
		return nil, errTenancyNotFound
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get project %s: status %d: %s", name, resp.StatusCode, string(b))
	}
	out := new(projectStatusBlock)
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, fmt.Errorf("get project %s: decode: %w", name, err)
	}
	return out, nil
}

// DeleteProject DELETEs /v1/projects/{name}. 404 is treated as success.
func (c *tenancyRESTClient) DeleteProject(ctx context.Context, name string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/v1/projects/"+name, nil)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("delete project %s: status %d: %s", name, resp.StatusCode, string(b))
}

// waitUntilOrgIdle polls GET /v1/orgs/{name} until StatusIndicator is IDLE.
// Returns the org UID.
func (c *tenancyRESTClient) waitUntilOrgIdle(ctx context.Context, name string) (string, error) {
	deadline := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		org, err := c.GetOrg(ctx, name)
		if err != nil && !isTenancyNotFound(err) {
			return "", err
		}
		if org != nil && org.Status.OrgStatus.StatusIndicator == statusIndicationIdle && org.Status.OrgStatus.UID != "" {
			return org.Status.OrgStatus.UID, nil
		}
		select {
		case <-ticker.C:
		case <-deadline:
			return "", fmt.Errorf("org %s did not reach IDLE within timeout", name)
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// waitUntilProjectIdle polls GET /v1/projects/{name} until StatusIndicator is IDLE.
// Returns the project UID.
func (c *tenancyRESTClient) waitUntilProjectIdle(ctx context.Context, name string) (string, error) {
	deadline := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		proj, err := c.GetProject(ctx, name)
		if err != nil && !isTenancyNotFound(err) {
			return "", err
		}
		if proj != nil {
			fmt.Printf("project %s status - %s (%s)\n", name, proj.Status.ProjectStatus.StatusIndicator, proj.Status.ProjectStatus.Message)
			if proj.Status.ProjectStatus.StatusIndicator == statusIndicationIdle && proj.Status.ProjectStatus.UID != "" {
				return proj.Status.ProjectStatus.UID, nil
			}
		}
		select {
		case <-ticker.C:
		case <-deadline:
			return "", fmt.Errorf("project %s did not reach IDLE within timeout", name)
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// waitUntilOrgGone polls GET /v1/orgs/{name} until 404.
func (c *tenancyRESTClient) waitUntilOrgGone(ctx context.Context, name string) error {
	deadline := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		_, err := c.GetOrg(ctx, name)
		if isTenancyNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		select {
		case <-ticker.C:
		case <-deadline:
			return fmt.Errorf("org %s deletion timed out", name)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// waitUntilProjectGone polls GET /v1/projects/{name} until 404.
func (c *tenancyRESTClient) waitUntilProjectGone(ctx context.Context, name string) error {
	deadline := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		_, err := c.GetProject(ctx, name)
		if isTenancyNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		select {
		case <-ticker.C:
		case <-deadline:
			return fmt.Errorf("project %s deletion timed out", name)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
