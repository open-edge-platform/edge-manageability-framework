// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/sh"
)

func (Deploy) rke2Cluster() error { //nolint: cyclop
	dockerUser, dockerUserPresent := os.LookupEnv("DOCKER_USERNAME")
	dockerPass, dockerPassPresent := os.LookupEnv("DOCKER_PASSWORD")

	var args []string
	if dockerUserPresent && dockerPassPresent {
		fmt.Println("Using Docker credentials for customizing RKE2 installation")
		args = append(args, "-u", dockerUser, "-p", dockerPass)
	}

	if err := sh.RunV(filepath.Join("rke2", "rke2installerlocal.sh")); err != nil {
		return fmt.Errorf("error running rke2installerlocal.sh: %w", err)
	}

	if err := sh.RunV("/bin/bash", append([]string{filepath.Join("rke2", "customize-rke2.sh")}, args...)...); err != nil {
		return fmt.Errorf("error running customize-rke2.sh: %w", err)
	}

	devEnv, present := os.LookupEnv("INSTALLER_DEPLOY")
	if !present && devEnv == "" {
		if err := (Registry{}.loadRegistryCacheCerts()); err != nil {
			return fmt.Errorf("error loading registry cache CA certificates into rke2 cluster: %w", err)
		}
	}

	// We need to wait for all deployments and pods to be Ready also before deploying OpenEBS
	if err := testDeploymentAndPods(); err != nil {
		return fmt.Errorf("error testing deployments and pods: %w", err)
	}

	// deploy openebs operator
	if err := sh.RunV("kubectl", "apply", "-f", openEbsOperatorK8sTemplate); err != nil {
		return fmt.Errorf("error applying openEbsOperatorK8sTemplate: %w", err)
	}

	// deploy openebs-path operator
	if err := sh.RunV("kubectl", "apply", "-f",
		filepath.Join("rke2", openEbsOperatorK8sTemplateFile)); err != nil {
		return fmt.Errorf("error applying openEbsOperatorK8sTemplateFile: %w", err)
	}

	// create etcd-cert secret
	if err := sh.RunV("kubectl", "create", "secret", "generic", "etcd-certs",
		"--from-file=/var/lib/rancher/rke2/server/tls/etcd/server-client.crt",
		"--from-file=/var/lib/rancher/rke2/server/tls/etcd/server-client.key",
		"--from-file=/var/lib/rancher/rke2/server/tls/etcd/server-ca.crt"); err != nil {
		return fmt.Errorf("error creating etcd-certs secret: %w", err)
	}

	// create cron job that periodically defrags etcd
	if err := sh.RunV("kubectl", "apply", "-f",
		filepath.Join("rke2", "defrag-etcd-job.yaml")); err != nil {
		return fmt.Errorf("error applying defrag-etcd-job.yaml: %w", err)
	}

	// Do a final verification (after installing OpenEBS) of all deployments and pods
	// before declaring cluster is ready
	// We need to wait for all deployments and pods to be Ready also before deploying OpenEBS
	if err := testDeploymentAndPods(); err != nil {
		return fmt.Errorf("error testing deployments and pods after OpenEBS installation: %w", err)
	}

	if err := sh.RunV(filepath.Join("rke2", "customize-rke2.sh")); err != nil {
		return fmt.Errorf("error running customize-rke2.sh after OpenEBS installation: %w", err)
	}

	fmt.Println("RKE2 cluster ready: 😊")
	return nil
}

func (Deploy) rke2ClusterAirGap() error {
	dockerUser, dockerUserPresent := os.LookupEnv("DOCKER_USERNAME")
	dockerPass, dockerPassPresent := os.LookupEnv("DOCKER_PASSWORD")

	if err := sh.RunV(filepath.Join("rke2", "rke2installerlocal.sh"), "-a"); err != nil {
		return fmt.Errorf("error running rke2installerlocal.sh in air-gap mode: %w", err)
	}

	if dockerUserPresent && dockerPassPresent {
		if err := sh.RunV(filepath.Join("rke2", "customize-rke2.sh"),
			"-u", dockerUser,
			"-p", dockerPass,
			"-a"); err != nil {
			return fmt.Errorf("error running customize-rke2.sh with Docker credentials in air-gap mode: %w", err)
		}
	} else {
		if err := sh.RunV(filepath.Join("rke2", "customize-rke2.sh")); err != nil {
			return fmt.Errorf("error running customize-rke2.sh in air-gap mode: %w", err)
		}
	}

	// Verify RKE2 deployments and pods are ready before installing OpenEBS
	if err := testDeploymentAndPods(); err != nil {
		return fmt.Errorf("error testing deployments and pods in air-gap mode: %w", err)
	}

	if err := importImagesToRke2Cluster(); err != nil {
		return fmt.Errorf("error importing images to RKE2 cluster in air-gap mode: %w", err)
	}

	// deploy openebs-path operator
	if err := sh.RunV("kubectl", "apply", "-f",
		filepath.Join(rke2ArtifactDownloadPath, openEbsOperatorK8sTemplateFile)); err != nil {
		return fmt.Errorf("error applying openEbsOperatorK8sTemplateFile in air-gap mode: %w", err)
	}

	// Do a final verification (after installing OpenEBS) of all deployments and pods
	// before declaring cluster is ready
	// We need to wait for all deployments and pods to be Ready also before deploying OpenEBS
	if err := testDeploymentAndPods(); err != nil {
		return fmt.Errorf("error testing deployments and pods after OpenEBS installation in air-gap mode: %w", err)
	}

	if err := sh.RunV(filepath.Join("rke2", "customize-rke2.sh")); err != nil {
		return fmt.Errorf("error running customize-rke2.sh after OpenEBS installation in air-gap mode: %w", err)
	}

	fmt.Println("RKE2 cluster ready: 😊")

	return nil
}

func (d Deploy) downloadCniCalicoTarBall(rke2Version string) error {
	// Download CNI Calico tar ball
	CNILibURL := fmt.Sprintf(rke2CNICalicoURLFmt, url.QueryEscape(rke2Version))
	if err := downloadFile(filepath.Join(rke2ArtifactDownloadPath, rke2CalicoLibPkg), CNILibURL); err != nil {
		return fmt.Errorf("error downloading CNI Calico tarball: %w", err)
	}
	return nil
}

func (d Deploy) downloadRke2TarBall(rke2Version string) error {
	// Download RKE2 Images tar ball
	rke2ImagesURL := fmt.Sprintf(rke2ImagesURLFmt, url.QueryEscape(rke2Version))
	if err := downloadFile(filepath.Join(rke2ArtifactDownloadPath, rke2ImagesPkg), rke2ImagesURL); err != nil {
		return fmt.Errorf("error downloading RKE2 images tarball: %w", err)
	}
	// Download RKE2 libraries tar ball
	rke2LibURL := fmt.Sprintf(rke2LibURLFmt, url.QueryEscape(rke2Version))
	if err := downloadFile(filepath.Join(rke2ArtifactDownloadPath, rke2LibPkg), rke2LibURL); err != nil {
		return fmt.Errorf("error downloading RKE2 libraries tarball: %w", err)
	}
	// Download RKE2 libraries SHA file
	rke2LibSHAURL := fmt.Sprintf(rke2LibSHAURLFmt, url.QueryEscape(rke2Version))
	if err := downloadFile(filepath.Join(rke2ArtifactDownloadPath, rke2LibSHAFile), rke2LibSHAURL); err != nil {
		return fmt.Errorf("error downloading RKE2 libraries SHA file: %w", err)
	}

	return nil
}

func (d Deploy) downloadRKE2CustomArtifacts() error {
	openEbsOperatorK8sTemplateDownloadPath := filepath.Join(rke2ArtifactDownloadPath, openEbsOperatorK8sTemplateFile)
	if err := downloadFile(openEbsOperatorK8sTemplateDownloadPath, openEbsOperatorK8sTemplate); err != nil {
		return fmt.Errorf("error downloading OpenEBS operator K8s template: %w", err)
	}

	if err := downloadImagesForK8sTemplate(openEbsOperatorK8sTemplateDownloadPath); err != nil {
		return fmt.Errorf("error downloading images for OpenEBS operator K8s template: %w", err)
	}

	openEbsHostPathStorageK8sTemplateDownloadPath := filepath.Join(rke2ArtifactDownloadPath, openEbsOperatorK8sTemplateFile)
	if err := downloadFile(openEbsHostPathStorageK8sTemplateDownloadPath, openEbsHostPathStorageK8sTemplate); err != nil {
		return fmt.Errorf("error downloading OpenEBS host path storage K8s template: %w", err)
	}

	if err := downloadImagesForK8sTemplate(openEbsHostPathStorageK8sTemplateDownloadPath); err != nil {
		return fmt.Errorf("error downloading images for OpenEBS host path storage K8s template: %w", err)
	}

	return nil
}

func (d Deploy) downloadRKE2InstallerScript() error {
	installerScriptDownloadPath := filepath.Join(rke2ArtifactDownloadPath, "install.sh")
	if err := downloadFile(installerScriptDownloadPath, "https://get.rke2.io"); err != nil {
		return fmt.Errorf("error downloading RKE2 installer script: %w", err)
	}
	return nil
}

func downloadFile(filepath string, url string) error {
	// Get the data
	// Disable below golangci-lint errors. They are not relevant in this context.
	// 1. G107: Potential HTTP request made with variable url
	// 2. net/http.Get must not be called
	resp, err := http.Get(url) //nolint: gosec, noctx
	if err != nil {
		return fmt.Errorf("error making HTTP GET request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status code %d from %s", resp.StatusCode, url)
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", filepath, err)
	}
	defer out.Close()

	if _, err = io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("error writing to file %s: %w", filepath, err)
	}
	return nil
}

func downloadImagesForK8sTemplate(k8TemplatePath string) error {
	out, err := exec.Command("cat", k8TemplatePath).Output()
	if err != nil {
		return fmt.Errorf("error reading K8s template file %s: %w", k8TemplatePath, err)
	}
	if err := downloadImages(out); err != nil {
		return fmt.Errorf("error downloading images for K8s template %s: %w", k8TemplatePath, err)
	}
	return nil
}

func downloadImages(inputFeed []byte) error {
	images := make(map[string]bool)
	for line := range strings.SplitSeq(string(inputFeed), "\n") {
		if strings.Contains(line, "image:") {
			if len(strings.Split(line, "image:")) == 2 {
				image := strings.TrimSpace(strings.Split(line, "image:")[1])
				images[image] = true
			}
		}
	}

	if err := sh.RunV("mkdir", "-p", rke2CustomImageDownloadPath); err != nil {
		return fmt.Errorf("error creating directory %s: %w", rke2CustomImageDownloadPath, err)
	}

	for image := range images {
		if err := sh.RunV("docker", "pull", image); err != nil {
			return fmt.Errorf("error pulling Docker image %s: %w", image, err)
		}
		i := strings.Split(image, "/")
		if len(i) > 1 {
			l := len(i)
			j := strings.Split(i[l-1], ":")
			if err := sh.RunV("docker", "save", "-o",
				rke2CustomImageDownloadPath+j[0]+".tar", image); err != nil {
				return fmt.Errorf("error saving Docker image %s to tar: %w", image, err)
			}
		}
	}
	return nil
}

func importImagesToRke2Cluster() error {
	files, err := os.ReadDir(rke2CustomImageDownloadPath)
	if err != nil {
		return fmt.Errorf("error reading directory %s: %w", rke2CustomImageDownloadPath, err)
	}
	for _, file := range files {
		fmt.Printf("file name: %v\n", file.Name())
		if err = sh.RunV("sudo", "ctr", "--namespace", "k8s.io", "--address",
			"/run/k3s/containerd/containerd.sock", "images", "import",
			rke2CustomImageDownloadPath+"/"+file.Name()); err != nil {
			return fmt.Errorf("error importing image from file %s: %w", file.Name(), err)
		}
	}
	return nil
}

func testDeploymentAndPods() error {
	if err := (Test{}.deployment()); err != nil {
		return fmt.Errorf("timeout waiting for deployment to be ready: %w", err)
	}

	if err := (Test{}.pods()); err != nil {
		return fmt.Errorf("error waiting for pods to be ready: %w", err)
	}
	return nil
}
