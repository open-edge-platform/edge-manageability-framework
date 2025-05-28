// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"

	util "github.com/open-edge-platform/edge-manageability-framework/mage"
)

func doHarborREST(
	ctx context.Context,
	tls *tls.Config,
	method string,
	endpoint string,
	username string,
	token string,
	expectedStatus int,
	ignoreResponse bool,
) *http.Response {
	c := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tls,
		},
	}
	endpoint = strings.ReplaceAll(endpoint, "oci://", "https://")
	req, err := http.NewRequestWithContext(ctx, method,
		endpoint,
		nil)
	req.SetBasicAuth(username, token)
	Expect(err).ToNot(HaveOccurred())
	resp, err := c.Do(req)
	if !ignoreResponse {
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(expectedStatus), func() string {
			b, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			return fmt.Sprintf("error on %s %s %s",
				method, endpoint, string(b))
		})
	}
	return resp
}

func checkLoginCredentials(ctx context.Context, reg Registry, harborProjectName string) {
	Expect(reg).ToNot(BeNil(), "checking registry")
	Expect(reg.Type).ToNot(BeNil(), "checking registry %s type", reg.Name)
	Expect(reg.RootURL).ToNot(BeNil(), "checking registry %s type", reg.Name)
	Expect(reg.AuthToken).ToNot(BeNil(), "checking Registry %s AuthToken", reg.Name)
	Expect(reg.Cacerts).ToNot(BeNil(), "checking Registry %s CaCert", reg.Name)
	Expect(reg.Username).ToNot(BeNil(), "checking Registry %s Username", reg.Name)

	if reg.Type == "IMAGE" {
		caPool := x509.NewCertPool()
		caPool.AppendCertsFromPEM([]byte(reg.Cacerts))
		tlsConfiguration := &tls.Config{ //nolint: gosec
			RootCAs: caPool,
		}
		baseRegistryURL := strings.Replace(reg.RootURL, "oci://", "https://", 1)
		baseRegistryURL = strings.ToLower(baseRegistryURL)
		baseRegistryURL = strings.TrimSuffix(baseRegistryURL, harborProjectName) + "api/v2.0/"
		resp := doHarborREST(ctx, tlsConfiguration, "GET", baseRegistryURL+"projects?name="+harborProjectName, reg.Username, reg.AuthToken, http.StatusOK, checkRESTResponse)
		b, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()
		Expect(string(b)).To(ContainSubstring(harborProjectName), "expecting project name %s to be found in query", harborProjectName)
	}
}

func validVersion(version string) {
	versionRe, _ := regexp.Compile(`^[0-9]+\.[0-9]+\..*$`)
	Expect(versionRe.MatchString(version)).To(BeTrue(), "invalid version string %s", version)
}

func checkApp(app Application, helmRegistryName string, chartName string, defaultProfileName string,
) {
	Expect(app.HelmRegistryName).To(Equal(helmRegistryName), "checking app %s", app.Name)
	Expect(app.ChartName).To(Equal(chartName), "checking app %s", app.Name)
	Expect(app.DefaultProfileName).To(Equal(defaultProfileName), "checking app %s", app.Name)
	validVersion(app.Version)
	validVersion(app.ChartVersion)
}

func checkDeploymentPackage(dp DeploymentPackage, defaultProfileName string,
	isVisible bool, referenceCount int, dependencyCount int, profilesCount int,
) {
	Expect(dp.DefaultProfileName).To(Equal(defaultProfileName), "checking DP %s", dp.Name)
	Expect(dp.IsVisible).To(Equal(isVisible), "checking DP %s", dp.Name)
	Expect(len(dp.ApplicationReferences) >= referenceCount).To(BeTrue(), "checking DP %s", dp.Name)
	Expect(len(*dp.ApplicationDependencies) >= dependencyCount).To(BeTrue(), "checking DP %s", dp.Name)
	Expect(len(*dp.ApplicationDependencies) >= dependencyCount).To(BeTrue(), "checking DP %s", dp.Name)
	Expect(len(dp.Profiles) >= profilesCount).To(BeTrue(), "checking DP %s", dp.Name)
	validVersion(dp.Version)
}

func checkRegistry(reg Registry, displayName string, description string, regtype string, rootURL string) {
	Expect(reg.DisplayName).To(Equal(displayName), "checking Registry %s", reg.Name)
	Expect(reg.Description).To(Equal(description), "checking Registry %s", reg.Name)
	Expect(reg.Type).To(Equal(regtype), "checking Registry %s", reg.Name)
	Expect(reg.RootURL).To(Equal(rootURL), "checking Registry %s", reg.Name)
}

var _ = Describe("Config Provisioner integration test", Label("orchestrator-integration"), func() {
	var c *http.Client
	var cancel context.CancelFunc
	var ctx context.Context

	testPassword := func() string {
		pass, err := util.GetDefaultOrchPassword()
		if err != nil {
			log.Fatal(err)
		}
		return pass
	}()

	harborProjectDisplayName := "catalog-apps-" + testOrg + "-" + testProject
	harborProjectName := strings.ToLower(harborProjectDisplayName)

	BeforeEach(func() {
		c = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Catalog bootstrap registries test", Ordered, func() {
		When("Checking bootstrap registries", Ordered, func() {
			It("should Get a token from KeyCloak", func() {
				accessToken = getAccessToken(c, testUsername, testPassword)
				Expect(accessToken).To(Not(ContainSubstring(`named cookie not present`)))
			})

			It("should determine that the harbor registries was created", func() {
				harborHelmOCIReg := GetRegistry(ctx, c, accessToken, testProject, "harbor-helm-oci", http.StatusOK, checkRESTResponse)
				checkRegistry(harborHelmOCIReg, "harbor oci helm", "Harbor OCI helm charts registry", "HELM",
					"oci://registry-oci."+serviceDomain+"/"+harborProjectDisplayName)
				checkLoginCredentials(ctx, harborHelmOCIReg, harborProjectName)

				harborDockerOCIReg := GetRegistry(ctx, c, accessToken, testProject, "harbor-docker-oci", http.StatusOK, checkRESTResponse)
				checkRegistry(harborDockerOCIReg, "harbor oci docker", "Harbor OCI docker images registry", "IMAGE",
					"oci://registry-oci."+serviceDomain+"/"+harborProjectName)
				checkLoginCredentials(ctx, harborDockerOCIReg, harborProjectName)

				intelRSHelmReg := GetRegistry(ctx, c, accessToken, testProject, "intel-rs-helm", http.StatusOK, checkRESTResponse)
				checkRegistry(intelRSHelmReg, "intel-rs-helm", "Repo on registry registry-rs.edgeorchestration.intel.com", "HELM",
					"oci://rs-proxy.orch-platform.svc.cluster.local:8443")

				intelRSImagesReg := GetRegistry(ctx, c, accessToken, testProject, "intel-rs-images", http.StatusOK, checkRESTResponse)
				checkRegistry(intelRSImagesReg, "intel-rs-image", "Repo on registry registry-rs.edgeorchestration.intel.com", "IMAGE",
					"oci://registry-rs.edgeorchestration.intel.com")
			})

			It("should determine that the applications were created", func() {
				var apps Applications
				apps = ListApplicationsByName(ctx, c, accessToken, testProject, "intel-device-operator")
				Expect(apps.Applications).ToNot(BeEmpty())
				checkApp(apps.Applications[0],
					"intel-github-io", "intel-device-plugins-operator", "default")

				apps = ListApplicationsByName(ctx, c, accessToken, testProject, "intel-gpu-plugin")
				Expect(apps.Applications).ToNot(BeEmpty())
				checkApp(apps.Applications[0],
					"intel-github-io", "intel-device-plugins-gpu", "exclusive-gpu-alloc")

				apps = ListApplicationsByName(ctx, c, accessToken, testProject, "kubevirt-helper")
				Expect(apps.Applications).ToNot(BeEmpty())
				checkApp(apps.Applications[0],
					"intel-rs-helm", "edge-orch/en/charts/kubevirt-helper", "default")
			})

			It("should determine that the deployment packages were created", func() {
				var dps DeploymentPackages

				dps = ListDeploymentPackagesByName(ctx, c, accessToken, testProject, "intel-gpu")
				Expect(dps.DeploymentPackages).ToNot(BeEmpty())
				checkDeploymentPackage(dps.DeploymentPackages[0], "exclusive-gpu-alloc",
					false, 2, 1, 1)

				dps = ListDeploymentPackagesByName(ctx, c, accessToken, testProject, "virtualization")
				Expect(dps.DeploymentPackages).ToNot(BeEmpty())
				checkDeploymentPackage(dps.DeploymentPackages[0], "default-profile", false,
					3, 0, 2)
			})

			It("should check that deployments are created in ADM", func() {
				var dep Deployment
				dep = getDeploymentByDisplayNameAndProfileName(ctx, c, accessToken, testProject, "base-extensions-restricted", "restricted", http.StatusOK, checkRESTResponse)
				Expect(dep.Name).To(ContainSubstring("deployment"), "checking name of deployment %s", "base-extensions-restricted")
				dep = getDeploymentByDisplayNameAndProfileName(ctx, c, accessToken, testProject, "base-extensions-restricted", "privileged", http.StatusOK, checkRESTResponse)
				Expect(dep.Name).To(ContainSubstring("deployment"), "checking name of deployment %s", "base-extensions-privileged")
				dep = getDeploymentByDisplayNameAndProfileName(ctx, c, accessToken, testProject, "base-extensions-restricted", "baseline", http.StatusOK, checkRESTResponse)
				Expect(dep.Name).To(ContainSubstring("deployment"), "checking name of deployment %s", "base-extensions-baseline")
			})
		})
	})
})

var _ = Describe("Provisioned registries push test", Label("orchestrator-integration"), func() {
	var c *http.Client
	var cancel context.CancelFunc
	var ctx context.Context

	testPassword := func() string {
		pass, err := util.GetDefaultOrchPassword()
		if err != nil {
			log.Fatal(err)
		}
		return pass
	}()

	harborProjectDisplayName := "catalog-apps-" + testOrg + "-" + testProject
	harborProjectName := strings.ToLower(harborProjectDisplayName)

	BeforeEach(func() {
		c = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Provisioned registries push test", Ordered, func() {
		When("Checking provisioned registries", Ordered, func() {
			It("should Get a token from KeyCloak", func() {
				accessToken = getAccessToken(c, testUsername, testPassword)
				Expect(accessToken).To(Not(ContainSubstring(`named cookie not present`)))
			})

			It("should check that a docker image can be pushed", func() {
				var regAuth string

				reg := GetRegistry(ctx, c, accessToken, testProject, "harbor-docker-oci", http.StatusOK, checkRESTResponse)
				authConfig := registry.AuthConfig{
					Username: reg.Username,
					Password: reg.AuthToken,
				}
				encodedJSON, err := json.Marshal(authConfig)
				Expect(err).ToNot(HaveOccurred(), "checking Registry %s", reg)
				regAuth = base64.URLEncoding.EncodeToString(encodedJSON)

				dc, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
				Expect(err).ToNot(HaveOccurred(), "new docker client")
				defer dc.Close()

				tempDir, err := os.MkdirTemp("/tmp", "example")
				Expect(err).ToNot(HaveOccurred(), "creating temp dir")
				defer os.Remove(tempDir)

				// Define the Dockerfile content
				dockerfile := `FROM scratch
				# prevent docker build from complaining about an empty image
				COPY Dockerfile /Dockerfile
				`

				// Create a temporary directory to serve as build context
				dockerfilePath := tempDir + "/Dockerfile"
				err = os.WriteFile(dockerfilePath, []byte(dockerfile), 0o644)
				Expect(err).ToNot(HaveOccurred(), "writing dockerfile")
				defer os.Remove(dockerfilePath)

				buildContext, err := archive.TarWithOptions(tempDir, &archive.TarOptions{})
				Expect(err).ToNot(HaveOccurred(), "creating tar build context")
				defer buildContext.Close()

				imageBuildResponse, err := dc.ImageBuild(
					ctx,
					buildContext,
					dockertypes.ImageBuildOptions{
						Dockerfile: "Dockerfile",
						Tags:       []string{"docker-push-test-image:latest"},
						Remove:     true,
					},
				)
				Expect(err).ToNot(HaveOccurred(), "building docker image")

				// Print the build logs
				_, err = io.Copy(os.Stdout, imageBuildResponse.Body)
				Expect(err).ToNot(HaveOccurred(), "copying docker build response to stdout")

				imageName := "docker-push-test-image"
				imageVer := "latest"

				regDomain := strings.ToLower(reg.RootURL[strings.LastIndex(reg.RootURL, "//")+2:])
				remoteImageName := fmt.Sprintf("%s/%s:%s", strings.TrimRight(regDomain, "/"), imageName, imageVer)

				err = dc.ImageTag(ctx, imageName+":"+imageVer, remoteImageName)
				Expect(err).ToNot(HaveOccurred(), "tagging docker image %s as %s", imageName+":"+imageVer, remoteImageName)

				Expect(regAuth).ToNot(BeEmpty(), "checking docker login credentials")
				push, err := dc.ImagePush(ctx, remoteImageName, image.PushOptions{
					All:          true,
					RegistryAuth: regAuth,
				})
				Expect(err).ToNot(HaveOccurred(), "docker push of %s", remoteImageName)
				defer push.Close()
				buf := new(strings.Builder)

				_, err = io.Copy(buf, push)
				Expect(err).ToNot(HaveOccurred(), "copying docker response to buffer")
				Expect(buf.String()).ToNot(ContainSubstring("unauthorized"),
					"check docker response does not contain 'unauthorized'")
				Expect(buf.String()).ToNot(ContainSubstring("failed to verify certificate"),
					"check docker response is OK with registry CA")
				Expect(buf.String()).To(ContainSubstring(imageVer),
					"check docker response contains %s", imageVer)

				caPool := x509.NewCertPool()
				caPool.AppendCertsFromPEM([]byte(reg.Cacerts))
				tlsConfiguration := &tls.Config{ //nolint: gosec
					RootCAs: caPool,
				}
				baseRegistryURL := strings.TrimSuffix(
					strings.Replace(reg.RootURL, "oci://", "https://", 1), harborProjectName) + "api/v2.0/"
				doHarborREST(ctx, tlsConfiguration, "GET", baseRegistryURL+"projects/"+harborProjectName+
					"/repositories?q=name%3D"+harborProjectName+"%2F"+imageName,
					reg.Username, reg.AuthToken, http.StatusOK, checkRESTResponse)

				// Cleanup
				doHarborREST(ctx, tlsConfiguration, "DELETE", baseRegistryURL+"projects/"+harborProjectName+
					"/repositories/"+imageName,
					reg.Username, reg.AuthToken, http.StatusOK, true)

				remove, err := dc.ImageRemove(ctx, remoteImageName, image.RemoveOptions{
					Force: true,
				})
				Expect(err).ToNot(HaveOccurred(), "removing image %s from docker", remoteImageName)
				Expect(remove).To(HaveLen(2), "checking docker remove reply")
				Expect(remove[0].Untagged).To(Equal(remoteImageName), "checking docker remove reply part 1")
				Expect(remove[1].Untagged).To(ContainSubstring("sha"), "checking docker remove reply part 2")
			})

			It("should check that a docker helm chart can be pushed", func() {
				var harborHelmReg Registry
				var err error
				var orasPath string
				var registryServer string

				harborHelmReg = GetRegistry(ctx, c, accessToken, testProject, "harbor-helm-oci", http.StatusOK, checkRESTResponse)

				orasPath = strings.ToLower(strings.ReplaceAll(harborHelmReg.RootURL, "oci://", ""))
				registryServer = strings.SplitN(orasPath, "/", 2)[0]
				tag := "1.0." + generateRandomDigits(25)

				// Create a sample chart
				tempDir, err := os.MkdirTemp("/tmp", "example")
				Expect(err).ToNot(HaveOccurred(), "creating temp dir")
				defer os.Remove(tempDir)
				chart := `
# SPDX-FileCopyrightText: (C) 2024 Intel Corporation
#
# SPDX-License-Identifier: LicenseRef-Intel
---
apiVersion: v2
`
				chartPath := tempDir + "/Chart.yaml"
				err = os.WriteFile(chartPath, []byte(chart), 0o644)
				Expect(err).ToNot(HaveOccurred(), "writing chart")
				defer os.Remove(chartPath)

				// 0. Create a file store
				fs, err := file.New(tempDir)
				Expect(err).ToNot(HaveOccurred(), "making file")
				defer fs.Close()

				// Add files to the file store
				mediaType := "application/vnd.test.file"
				fileNames := []string{chartPath}
				fileDescriptors := make([]v1.Descriptor, 0, len(fileNames))
				for _, name := range fileNames {
					fileDescriptor, err := fs.Add(ctx, name, mediaType, "")
					Expect(err).ToNot(HaveOccurred(), "making fileDescriptor")
					fileDescriptors = append(fileDescriptors, fileDescriptor)
				}

				// Pack the files and tag the packed manifest
				artifactType := "application/vnd.test.artifact"
				opts := oras.PackManifestOptions{
					Layers: fileDescriptors,
				}
				manifestDescriptor, err := oras.PackManifest(ctx, fs, oras.PackManifestVersion1_1, artifactType, opts)
				Expect(err).ToNot(HaveOccurred(), "making manifestDescriptor")

				err = fs.Tag(ctx, manifestDescriptor, tag)
				Expect(err).ToNot(HaveOccurred(), "tagging")

				// Connect to a remote repository
				repo, err := remote.NewRepository(orasPath + "/push-test")
				Expect(err).ToNot(HaveOccurred(), "making remote repo")
				repo.Client = &auth.Client{
					Client: c,
					Cache:  auth.NewCache(),
					Credential: auth.StaticCredential(registryServer, auth.Credential{
						Username: harborHelmReg.Username,
						Password: harborHelmReg.AuthToken,
					}),
				}

				// Copy from the file store to the remote repository
				_, err = oras.Copy(ctx, fs, tag, repo, tag, oras.DefaultCopyOptions)
				Expect(err).ToNot(HaveOccurred(), "copying to remote repo")

				// Fetch the chart we just made
				fetchFs := memory.New()
				fetchedManifestDescriptor, err := oras.Copy(ctx, repo, tag, fetchFs, tag, oras.DefaultCopyOptions)
				Expect(err).ToNot(HaveOccurred(), "fetching from remote repo")

				// make sure the hashes match
				Expect(fetchedManifestDescriptor.Digest.String()).To(Equal(manifestDescriptor.Digest.String()), "chart contents SHA")
			})
		})
	})
})
