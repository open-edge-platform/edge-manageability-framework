// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/gomega"
)

// ConstructCatalogURL constructs a URL for the catalog service give a project and an object
func ConstructCatalogURL(projectName string, objectName string) string {
	if objectName != "" {
		return fmt.Sprintf("%s/%s/projects/%s/catalog/%s", apiBaseURL, catalogApiVersion, projectName, objectName)
	} else {
		return fmt.Sprintf("%s/%s/projects/%s/catalog", apiBaseURL, catalogApiVersion, projectName)
	}
}

// DoCatalogRest executes a REST API request to the catalog service
func DoCatalogREST(
	ctx context.Context,
	c *http.Client,
	method string,
	endpoint string,
	projectName string,
	accessToken string,
	body io.Reader,
	expectedStatus int,
	ignoreResponse bool,
) *http.Response {
	catalogURL := ConstructCatalogURL(projectName, endpoint)
	return doREST(ctx, c, method, catalogURL, accessToken, body, expectedStatus, ignoreResponse)
}

// Catalog Registries

type (

	// Registry is the JSON representation of a registry.
	Registry struct {
		AuthToken   string  `json:"authToken,omitempty"`
		Cacerts     string  `json:"cacerts,omitempty"`
		Description string  `json:"description,omitempty"`
		DisplayName string  `json:"displayName,omitempty"`
		Name        string  `json:"name"`
		RootURL     string  `json:"rootUrl"`
		SecretID    *string `json:"secretId,omitempty"`
		Type        string  `json:"type"`
		Username    string  `json:"username,omitempty"`
	}

	// RegistryGetResponse is the JSON representation of the result of a get of a registry
	RegistryGetResponse struct {
		Registry Registry `json:"registry"`
	}
)

// CreateRegistry uses the REST API to create a registry.
func CreateRegistry(ctx context.Context, c *http.Client, accessToken string, projectName string, registry Registry) {
	registryBody, err := json.Marshal(registry)
	Expect(err).ToNot(HaveOccurred())
	resp := DoCatalogREST(ctx, c, http.MethodPost, "registries", projectName,
		accessToken, bytes.NewReader(registryBody), http.StatusOK, checkRESTResponse)
	defer resp.Body.Close()
}

// GetRegistry uses the REST API to fetch a registry.
func GetRegistry(
	ctx context.Context,
	c *http.Client,
	accessToken string,
	project string,
	name string,
	expectedStatus int,
	ignoreResponse bool,
) Registry {
	resp := DoCatalogREST(
		ctx,
		c,
		http.MethodGet,
		"registries/"+name+"?showSensitiveInfo=true",
		project,
		accessToken,
		nil,
		expectedStatus,
		ignoreResponse,
	)
	defer resp.Body.Close()

	var registryResponse RegistryGetResponse
	err := json.NewDecoder(resp.Body).Decode(&registryResponse)
	Expect(err).ToNot(HaveOccurred())
	return registryResponse.Registry
}

// DeleteRegistry uses the REST API to delete a registry.
func DeleteRegistry(
	ctx context.Context,
	c *http.Client,
	accessToken string,
	project string,
	name string,
	ignoreResponse bool,
) {
	resp := DoCatalogREST(
		ctx,
		c,
		http.MethodDelete,
		"registries/"+name,
		project,
		accessToken,
		nil,
		http.StatusOK,
		ignoreResponse,
	)
	defer resp.Body.Close()
}

// Catalog artifacts

type (

	// ArtifactGetResponse is the JSON representation of the result of a get operation on an artifact.
	ArtifactGetResponse struct {
		Artifact Artifact `json:"artifact"`
	}

	// Artifact is the JSON representation of an artifact.
	Artifact struct {
		Description string `json:"description,omitempty"`
		DisplayName string `json:"displayName,omitempty"`
		Name        string `json:"name"`
		MimeType    string `json:"mimeType"`
		Artifact    []byte `json:"artifact"`
	}
)

// CreateArtifact uses the REST API to create an Artifact.
func CreateArtifact(ctx context.Context, c *http.Client,
	accessToken string,
	project string,
	name string,
	displayName string,
	mimeType string,
	contents []byte,
	expectedStatus int,
) *http.Response {
	// Create an artifact
	newArtifact := Artifact{
		Name:        name,
		DisplayName: displayName,
		MimeType:    mimeType,
		Artifact:    contents,
	}
	artifactBody, err := json.Marshal(newArtifact)
	Expect(err).ToNot(HaveOccurred())
	resp := DoCatalogREST(ctx, c, http.MethodPost, "artifacts", project, accessToken,
		bytes.NewReader(artifactBody), expectedStatus, checkRESTResponse)

	defer resp.Body.Close()

	return resp
}

// GetArtifact uses the REST API to find an artifact
func GetArtifact(
	ctx context.Context,
	c *http.Client,
	accessToken string,
	project string,
	name string,
	expectedStatus int,
	ignoreResponse bool,
) Artifact {
	resp := DoCatalogREST(
		ctx,
		c,
		http.MethodGet,
		"artifacts/"+name,
		project,
		accessToken,
		nil,
		expectedStatus,
		ignoreResponse,
	)
	defer resp.Body.Close()
	var artifactResponse ArtifactGetResponse
	err := json.NewDecoder(resp.Body).Decode(&artifactResponse)
	Expect(err).ToNot(HaveOccurred())
	return artifactResponse.Artifact
}

// DeleteArtifact uses the REST API to delete an artifact.
func DeleteArtifact(
	ctx context.Context,
	c *http.Client,
	accessToken string,
	project string,
	name string,
	ignoreResponse bool,
) {
	resp := DoCatalogREST(
		ctx,
		c,
		http.MethodDelete,
		"artifacts/"+name,
		project,
		accessToken,
		nil,
		http.StatusOK,
		ignoreResponse,
	)
	defer resp.Body.Close()
}

// Catalog applications

type (
	Profile struct {
		ChartValues string `json:"chartValues,omitempty"`
		Description string `json:"description,omitempty"`
		DisplayName string `json:"displayName,omitempty"`
		Name        string `json:"name"`
	}

	// Application is the JSON representation of an application.
	Application struct {
		ChartName          string    `json:"chartName"`
		ChartVersion       string    `json:"chartVersion"`
		DefaultProfileName string    `json:"defaultProfileName,omitempty"`
		Description        string    `json:"description,omitempty"`
		DisplayName        string    `json:"displayName,omitempty"`
		HelmRegistryName   string    `json:"helmRegistryName"`
		ImageRegistryName  string    `json:"imageRegistryName,omitempty"`
		Name               string    `json:"name"`
		Profiles           []Profile `json:"profiles,omitempty"`
		Version            string    `json:"version"`
	}

	// ApplicationGetResponse is the JSON representation of a get of an application.
	ApplicationGetResponse struct {
		Application Application `json:"application"`
	}

	ApplicationDependency struct{}
	ApplicationReference  struct{}
	ArtifactReference     struct{}
	Endpoint              struct {
		AuthType     string `json:"authType"`
		ExternalPath string `json:"externalPath"`
		InternalPath string `json:"internalPath"`
		Scheme       string `json:"scheme"`
		ServiceName  string `json:"serviceName"`
	}

	UIExtension struct{}

	APIExtension struct {
		Description string      `json:"description,omitempty"`
		DisplayName string      `json:"displayName,omitempty"`
		Endpoints   []Endpoint  `json:"endpoints,omitempty"`
		Name        string      `json:"name"`
		UiExtension UIExtension `json:"uiExtension,omitempty"` //nolint: revive,stylecheck
		Version     string      `json:"version"`
	}
	// Applications is the JSON representation of a list of applications.
	Applications struct {
		Applications []Application `json:"applications"`
	}
)

// GetApplication uses the REST API to fetch an application from the catalog
func GetApplication(
	ctx context.Context,
	c *http.Client,
	accessToken string,
	project string,
	name string,
	version string,
	expectedStatus int,
	ignoreResponse bool,
) Application {
	resp := DoCatalogREST(
		ctx,
		c,
		http.MethodGet,
		"applications/"+name+"/versions/"+version,
		project,
		accessToken,
		nil,
		expectedStatus,
		ignoreResponse,
	)
	defer resp.Body.Close()
	var applicationResponse ApplicationGetResponse
	err := json.NewDecoder(resp.Body).Decode(&applicationResponse)
	Expect(err).ToNot(HaveOccurred())
	return applicationResponse.Application
}

// DeleteApplication uses the REST API to remove an application from the catalog
func DeleteApplication(
	ctx context.Context,
	c *http.Client,
	accessToken string,
	project string,
	name string,
	version string,
	ignoreResponse bool,
) {
	resp := DoCatalogREST(
		ctx,
		c,
		http.MethodDelete,
		"applications/"+name+"/versions/"+version,
		project,
		accessToken,
		nil,
		http.StatusOK,
		ignoreResponse,
	)
	defer resp.Body.Close()
}

// ListApplicationsByName uses the REST API to list the applications.
func ListApplicationsByName(ctx context.Context, c *http.Client, accessToken string, project string, appName string) Applications {
	resp := DoCatalogREST(ctx, c, http.MethodGet,
		"applications?pageSize=100&filter=name="+appName,
		project,
		accessToken,
		nil,
		http.StatusOK,
		checkRESTResponse)
	defer resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	var applicationsResp Applications
	err := json.NewDecoder(resp.Body).Decode(&applicationsResp)
	Expect(err).ToNot(HaveOccurred())
	return applicationsResp
}

// Catalog deployment packages

type (
	// DeploymentPackage is the JSON representation of a deployment package.
	DeploymentPackage struct {
		ApplicationDependencies *[]ApplicationDependency `json:"applicationDependencies,omitempty"`
		ApplicationReferences   []ApplicationReference   `json:"applicationReferences"`
		Artifacts               []ArtifactReference      `json:"artifacts"`
		DefaultNamespaces       *map[string]string       `json:"defaultNamespaces,omitempty"`
		DefaultProfileName      string                   `json:"defaultProfileName,omitempty"`
		Description             string                   `json:"description,omitempty"`
		DisplayName             string                   `json:"displayName,omitempty"`
		Extensions              []APIExtension           `json:"extensions"`
		IsDeployed              bool                     `json:"isDeployed,omitempty"`
		IsVisible               bool                     `json:"isVisible,omitempty"`
		Name                    string                   `json:"name"`
		Profiles                []Profile                `json:"profiles,omitempty"`
		Version                 string                   `json:"version"`
	}

	// DeploymentPackages is the JSON representation of a list of deployment packages.
	DeploymentPackages struct {
		DeploymentPackages []DeploymentPackage `json:"DeploymentPackages"`
	}

	// DeploymentPackageGetResponse is the JSON representation of a get of an application.
	DeploymentPackageGetResponse struct {
		DeploymentPackage DeploymentPackage `json:"deploymentPackage"`
	}
)

// ListDeploymentPackagesByName - uses the REST API to list the deployment packages.
func ListDeploymentPackagesByName(ctx context.Context, c *http.Client, accessToken string, project string, name string) DeploymentPackages {
	resp := DoCatalogREST(ctx, c, http.MethodGet, "deployment_packages?pageSize=100&filter=name="+name, project, accessToken, nil, http.StatusOK, checkRESTResponse)
	defer resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	var deploymentPackagesResp DeploymentPackages
	err := json.NewDecoder(resp.Body).Decode(&deploymentPackagesResp)
	Expect(err).ToNot(HaveOccurred())

	return deploymentPackagesResp
}

// GetDeploymentPackage uses the REST API to find a package
func GetDeploymentPackage(
	ctx context.Context,
	c *http.Client,
	accessToken string,
	project string,
	name string,
	version string,
	expectedStatus int,
	ignoreResponse bool,
) DeploymentPackage {
	resp := DoCatalogREST(
		ctx,
		c,
		http.MethodGet,
		"deployment_packages/"+name+"/versions/"+version,
		project,
		accessToken,
		nil,
		expectedStatus,
		ignoreResponse,
	)
	defer resp.Body.Close()
	var deploymentPackageResponse DeploymentPackageGetResponse
	err := json.NewDecoder(resp.Body).Decode(&deploymentPackageResponse)
	Expect(err).ToNot(HaveOccurred())
	return deploymentPackageResponse.DeploymentPackage
}

// DeleteDeploymentPackage uses the REST API to delete a package
func DeleteDeploymentPackage(
	ctx context.Context,
	c *http.Client,
	accessToken string,
	project string,
	name string,
	version string,
	ignoreResponse bool,
) {
	resp := DoCatalogREST(
		ctx,
		c,
		http.MethodDelete,
		"deployment_packages/"+name+"/versions/"+version,
		project,
		accessToken,
		nil,
		http.StatusOK,
		ignoreResponse,
	)
	defer resp.Body.Close()
}
