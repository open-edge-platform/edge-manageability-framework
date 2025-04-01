// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/bitfield/script"
)

var dockerHubChartOrgs = []string{"bitnamicharts"}

func initialSecretInternal() (string, error) {
	kubeCmd := "kubectl get secret -n argocd argocd-initial-admin-secret -o json"

	data, err := script.Exec(kubeCmd).String()
	if err != nil {
		argocdAdminPassword, err := GetDefaultOrchPassword()
		if err != nil {
			return "", fmt.Errorf("could not get default password: %w", err)
		}
		// Return default password if argocd-initial-admin-secret does not exist
		return argocdAdminPassword, nil
	}
	cert, err := script.Echo(data).JQ(`.data."password"`).Replace(`"`, "").String()
	if err != nil {
		return "", fmt.Errorf("parse JSON for argo secret: %w", err)
	}

	secret, err := base64.StdEncoding.DecodeString(cert)
	if err != nil {
		return "", fmt.Errorf("decode base64 argo secret: %w", err)
	}
	return string(secret), nil
}

func (Argo) initSecret() error {
	secret, err := initialSecretInternal()
	if err != nil {
		return err
	}
	fmt.Println(secret)
	return nil
}

func (Argo) login() error {
	secret, err := initialSecretInternal()
	if err != nil {
		return err
	}

	argoIP := os.Getenv("ARGO_IP")
	if argoIP == "" {
		// login to Argo without using the router
		ip, err := lookupGenericIP("argocd", "argocd-server")
		if err != nil {
			return fmt.Errorf("performing argo IP lookup %w", err)
		}
		argoIP = ip
	}
	argoPort := os.Getenv("ARGO_PORT")
	if argoPort == "" {
		argoPort = "443"
	}

	cmd := fmt.Sprintf("argocd login %s:%s --username admin --password %s --insecure --grpc-web", argoIP, argoPort, secret)
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}
	return nil
}

func (Argo) repoAdd() error {
	gitUser := os.Getenv("GIT_USER")
	if gitUser == "" {
		return fmt.Errorf("must set environment variable GIT_USER")
	}

	gitToken := os.Getenv("GIT_TOKEN")
	if gitToken == "" {
		return fmt.Errorf("must set environment variable GIT_TOKEN")
	}

	for _, repo := range privateRepos {
		cmd := fmt.Sprintf("argocd repo add %s --username %s --password %s --upsert", repo, gitUser, gitToken)
		if _, err := script.Exec(cmd).Stdout(); err != nil {
			return err
		}
	}
	return nil
}

func (Argo) dockerHubChartOrgAdd() error {
	dockerToken := os.Getenv("DOCKERHUB_TOKEN")
	dockerUsername := os.Getenv("DOCKERHUB_USERNAME")

	if dockerToken != "" && dockerUsername != "" {
		cmdTemplate := template.Must(template.New("argocd-oci-secret").Parse(`
apiVersion: v1
stringData:
  password: {{ .Password }}
  username: {{ .Username }}
  type: helm
  url: registry-1.docker.io/{{ .ChartOrg }}
  enableOCI: "true"
  ForceHttpBasicAuth: "true"
kind: Secret
metadata:
  labels:
    argocd.argoproj.io/secret-type: repository
  name: {{ .Name }}
  namespace: argocd
type: Opaque`))

		for _, org := range dockerHubChartOrgs {
			buf := &bytes.Buffer{}

			err := cmdTemplate.Execute(buf, struct {
				Username string
				Password string
				Name     string
				ChartOrg string
			}{
				Username: dockerUsername,
				Password: dockerToken,
				Name:     fmt.Sprintf("dockerhub-%s", org),
				ChartOrg: org,
			})
			if err != nil {
				return fmt.Errorf("executing template: %w", err)
			}

			if _, err := script.Echo(buf.String()).Exec("kubectl apply -f - ").Stdout(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Lists all ArgoCD Applications, sorted by syncWave
func (Argo) appSeq() error {
	const childAppDir = "./argocd/applications/templates/"

	entries, err := os.ReadDir(childAppDir)
	if err != nil {
		return fmt.Errorf("could not read dir: %w", err)
	}
	type tup struct {
		Seq  int
		Name string
	}
	yaml := []tup{}
	re := regexp.MustCompile(`syncWave.*\:\=`)
	for _, e := range entries {
		stdErr := &bytes.Buffer{}
		fullPath := filepath.Join(childAppDir, e.Name())
		data, err := script.File(fullPath).WithStderr(stdErr).FilterLine(
			func(line string) string {
				if re.MatchString(line) {
					_, s1, ok := strings.Cut(line, `"`)
					if !ok {
						panic("could not find double quote in first cut cmd")
					}
					s2, _, ok := strings.Cut(s1, `"`)
					if !ok {
						panic("could not find double quote in second cut cmd")
					}
					return strings.TrimSpace(s2)
				}
				return ""
			}).String()
		if err != nil {
			return fmt.Errorf("filtering yaml template: %w %s", err, stdErr.String())
		}
		n, err := strconv.Atoi(strings.TrimSpace(data))
		if err != nil {
			return fmt.Errorf("converting sync wave: %w", err)
		}
		yaml = append(yaml, tup{
			Seq:  n,
			Name: fullPath,
		})
	}
	sort.Slice(yaml, func(i, j int) bool {
		return yaml[i].Seq < yaml[j].Seq
	})
	for _, y := range yaml {
		fmt.Printf("%d\t%s\n", y.Seq, y.Name)
	}
	return nil
}
