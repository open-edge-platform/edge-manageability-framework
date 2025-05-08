// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/bitfield/script"
	"github.com/magefile/mage/sh"
)

const edgeManageabilityFrameworkRepo = "edge-manageability-framework"

const giteaSvcDomain = "gitea-http.gitea.svc.cluster.local"

const gitReposEnv = "GIT_REPOS"

const orchInstallerProfileEnv = "ORCH_INSTALLER_PROFILE"

func main() {
	tarFilesLocation := os.Getenv(gitReposEnv)
	if tarFilesLocation == "" {
		log.Fatalf("%v env var is empty", gitReposEnv)
	}

	orchInstallerProfile := os.Getenv(orchInstallerProfileEnv)
	if orchInstallerProfile == "" {
		log.Fatalf("%v env var is empty", orchInstallerProfileEnv)
	}

	log.Printf("Starting installation of orch-installer using %s profile...", orchInstallerProfile)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get $HOME dir - %v", err)
	}

	edgeManageabilityFrameworkFolder, err := os.MkdirTemp(homeDir, edgeManageabilityFrameworkRepo)
	if err != nil {
		log.Panicf("failed to create temp directory - %v", err)
	}
	defer os.RemoveAll(edgeManageabilityFrameworkFolder)

	giteaServiceURL, err := getGiteaServiceURL()
	if err != nil {
		log.Fatalf("failed to get Gitea service URL - %v", err)
	}

	err = pushArtifactRepoToGitea(edgeManageabilityFrameworkFolder, getArtifactPath(tarFilesLocation, edgeManageabilityFrameworkRepo),
		edgeManageabilityFrameworkRepo, giteaServiceURL)
	if err != nil {
		log.Panicf("%v", err)
	}

	err = installRootApp(edgeManageabilityFrameworkFolder, orchInstallerProfile, giteaServiceURL)
	if err != nil {
		log.Panicf("failed to install root-app - %v", err)
	}

	fmt.Printf("Installation of orch-installer is completed.")
}

// Pushes repo from packaged tar file to Gitea repo on the cluster
// Takes 2 arguments:
// 1) Path to local tar file that contains repo
// 2) Name of the Gitea repo that will be created.
func pushArtifactRepoToGitea(untaredPath, artifactPath, repoName, giteaServiceURL string) error {
	_, err := sh.Output("tar", "-xf", artifactPath, "-C", untaredPath)
	if err != nil {
		return fmt.Errorf("failed to untar artifact - %w", err)
	}

	buf := &bytes.Buffer{}
	err = template.Must(template.New("job").Parse(`
apiVersion: batch/v1
kind: Job
metadata:
  name: gitea-init-{{ .repoName }}
  namespace: gitea
  labels:
    managed-by: edge-manageability-framework
spec:
  template:
    spec:
      volumes:
      - name: tea
        hostPath:
          path: /usr/bin/tea
      - name: repo
        hostPath:
          path: {{ .repoDir }}
      - name: tls
        secret:
          secretName: gitea-tls-certs
      containers:
      - name: alpine
        image: alpine/git:2.43.0
        env:
        - name: GITEA_USERNAME
          valueFrom:
            secretKeyRef:
              name: argocd-gitea-credential
              key: username
        - name: GITEA_PASSWORD
          valueFrom:
            secretKeyRef:
              name: argocd-gitea-credential
              key: password
        command:
        - /bin/sh
        - -c
        args:
        - git config --global credential.helper store;
          git config --global user.email $GITEA_USERNAME@orch-installer.com;
          git config --global user.name $GITEA_USERNAME;
          git config --global http.sslCAInfo /usr/local/share/ca-certificates/tls.crt;
          git config --global --add safe.directory /repo;
          echo "https://$GITEA_USERNAME:$GITEA_PASSWORD@{{ .giteaURL }}" > /root/.git-credentials;
          chdir /repo;
          git init;
          git remote add gitea https://{{ .giteaURL }}/$GITEA_USERNAME/{{ .repoName }}.git;
          git checkout -B main;
          git add .;
          git commit --allow-empty -m 'Recreate repo from artifact';
          git push --force gitea main;
        volumeMounts:
        - name: tea
          mountPath: /usr/bin/tea
        - name: repo
          mountPath: /repo
        - name: tls
          mountPath: /usr/local/share/ca-certificates/
      restartPolicy: Never
  backoffLimit: 5`)).
		Execute(buf, map[string]string{
			"repoName": repoName,
			"repoDir":  filepath.Join(untaredPath, repoName),
			"giteaURL": giteaServiceURL,
		})
	if err != nil {
		return fmt.Errorf("failed to template job - %w", err)
	}

	out, err := script.Echo(buf.String()).Exec("kubectl apply -f -").String()
	if err != nil {
		return fmt.Errorf("failed to create job - %w %s", err, out)
	}

	log.Printf("Waiting for %v job to finish...", "gitea-init-"+repoName)

	out, err = script.Exec("kubectl wait --for=condition=complete " +
		"--timeout=300s -n argocd job/gitea-init-" + repoName).String()
	if err != nil {
		return fmt.Errorf("failed to wait for job - %w %s", err, out)
	}

	return nil
}

func getArtifactPath(tarFilesLocation, repoName string) string {
	tarSuffix := ".tgz"

	files, err := os.ReadDir(tarFilesLocation)
	if err != nil {
		log.Panicf("failed to read artifacts directory - %v", err)
	}

	for _, file := range files {
		if strings.Contains(file.Name(), repoName) && strings.HasSuffix(file.Name(), tarSuffix) {
			return filepath.Join(tarFilesLocation, file.Name())
		}
	}

	log.Panicf("failed to get *%v*.tgz artifact path", repoName)
	return ""
}

// Deploys code from local Gitea repo using ArgoCd
// Takes 3 arguments:
// 1) Name of the repo on Gitea
// 2) Path to the helm chart inside the repo
// 3) Namespace in which chart will be deployed.
func installRootApp(edgeManageabilityFrameworkFolder, orchInstallerProfile, giteaServiceURL string) error {
	namespace := "onprem"
	if err := createGiteaCredsSecret(edgeManageabilityFrameworkRepo, namespace, giteaServiceURL); err != nil {
		return fmt.Errorf("failed to create gitea secret with creds - %w", err)
	}

	return sh.RunV("helm", "upgrade", "--install", "root-app",
		filepath.Join(edgeManageabilityFrameworkFolder, edgeManageabilityFrameworkRepo, "argocd/root-app"),
		"-f", filepath.Join(edgeManageabilityFrameworkFolder, edgeManageabilityFrameworkRepo, "orch-configs/clusters", orchInstallerProfile+".yaml"),
		"-n", namespace, "--create-namespace")
}

func createGiteaCredsSecret(repoName string, namespace string, giteaServiceURL string) error {
	argoCDCredsSecret := "argocd-gitea-credential"

	giteaUsernameBase64, err := sh.Output("kubectl", "get", "secret",
		argoCDCredsSecret, "-n", "gitea", "-o", "jsonpath={.data.username}")
	if err != nil {
		return fmt.Errorf("failed to fetch gitea username - %w", err)
	}
	giteaPasswordBase64, err := sh.Output("kubectl", "get", "secret",
		argoCDCredsSecret, "-n", "gitea", "-o", "jsonpath={.data.password}")
	if err != nil {
		return fmt.Errorf("failed to fetch gitea password - %w", err)
	}

	usernameReader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(giteaUsernameBase64))
	passwordReader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(giteaPasswordBase64))

	giteaPasswordBytes, err := io.ReadAll(passwordReader)
	if err != nil {
		return fmt.Errorf("failed to decode gitea password - %w", err)
	}
	giteaPassword := string(giteaPasswordBytes)

	giteaUsernameBytes, err := io.ReadAll(usernameReader)
	if err != nil {
		return fmt.Errorf("failed to decode gitea password - %w", err)
	}
	giteaUsername := string(giteaUsernameBytes)

	commandTemplate := template.Must(template.New("template").
		Parse(`kubectl create -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: {{ .repoName }}
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url:  https://{{ .giteaURL }}/{{ .username }}/{{ .repoName }}
  password: {{ .password }}
  username: {{ .username }}
EOF
`))

	_, _ = sh.Output("kubectl", "delete", "secret", repoName, "-n", "argocd")

	buf := &bytes.Buffer{}

	templateParams := map[string]string{
		"repoName":  repoName,
		"namespace": namespace,
		"username":  giteaUsername,
		"password":  giteaPassword,
		"giteaURL":  giteaServiceURL,
	}
	if err := commandTemplate.Execute(buf, templateParams); err != nil {
		return fmt.Errorf("failed to run command - %w", err)
	}

	if err := sh.RunV("sh", "-c", buf.String()); err != nil {
		return fmt.Errorf("failed to create secret with credentials - %w", err)
	}
	return nil
}

func getGiteaServiceURL() (string, error) {
	port, err := sh.Output("kubectl", "get", "svc", "gitea-http", "-n", "gitea", "-o", "jsonpath={.spec.ports[0].port}")
	if err != nil {
		return "", fmt.Errorf("failed to get Gitea service port - %w", err)
	}
	if port == "443" {
		return giteaSvcDomain, nil
	}
	return fmt.Sprintf("%s:%s", giteaSvcDomain, port), nil
}
