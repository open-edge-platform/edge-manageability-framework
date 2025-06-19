// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bitfield/script"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

type ArgoCDStep struct {
	RootPath               string
	KeepGeneratedFiles     bool
	orchConfigReaderWriter config.OrchConfigReaderWriter
	StepLabels             []string
}

func CreateArgoStep(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) *ArgoCDStep {
	return &ArgoCDStep{
		RootPath:               rootPath,
		KeepGeneratedFiles:     keepGeneratedFiles,
		orchConfigReaderWriter: orchConfigReaderWriter,
	}
}

func (s *ArgoCDStep) Name() string {
	return "ArgoStep"
}

func (s *ArgoCDStep) Labels() []string {
	return s.StepLabels
}

func (s *ArgoCDStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *ArgoCDStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "install" {
		// no-op for now
		err := argocdValues(config)
		if err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("Error installing Argocd %v \n", err),
			}
		}
	}
	return runtimeState, nil
}

func (s *ArgoCDStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "install" {
		// InstallArgoCD
		err := InstallArgoCD()
		if err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("Error installing Argocd %v \n", err),
			}
		}
	}
	if runtimeState.Action == "uninstall" {
		// UninstallArgoCD
		err := UninstallArgoCD()
		if err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("Error installing Argocd %v \n", err),
			}
		}
	}
	return runtimeState, nil
}

func (s *ArgoCDStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "install" {
		// Wait for ArgoCD namespace creation normally taken from an env var
		argoCDNS := "argocd"
		if argoCDNS == "" {
			argoCDNS = "argocd"
		}
		err := WaitForNamespaceCreation(argoCDNS)
		if err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
				ErrorMsg:  fmt.Sprintf("Error installing Argocd %v \n", err),
			}
		}
	}
	return runtimeState, prevStepError
}

func addArgoHelmRepo() error {
	argocdHelmVersion := "8.0.0"
	argocdPath := "/tmp/argo-cd/"
	if err := os.RemoveAll(filepath.Join(argocdPath, "argo-cd")); err != nil {
		fmt.Println("Failed to remove file:", err)
		return err
	}
	cmd := exec.Command("helm", "repo", "add", "argo-helm", "https://argoproj.github.io/argo-helm", "--force-update")
	cmd.Stdout = nil // or os.Stdout to print output
	cmd.Stderr = nil // or os.Stderr to print errors
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add argo-helm repo: %w", err)
	}

	cmd_str := fmt.Sprintf("helm fetch argo-helm/argo-cd --version %v --untar --untardir %v", argocdHelmVersion, argocdPath)
	if _, err := script.Exec(cmd_str).Stdout(); err != nil {
		return fmt.Errorf("failed to fetch argo-cd chart: %w", err)
	}
	return nil
}

func UninstallArgoCD() error {
	// Print ASCII art
	fmt.Println(`
     _                     ____ ____    ____
    / \   _ __ __ _  ___  / ___|  _ \  |  _ \ ___ _ __ ___   _____   _____
   / _ \ | '__/ _  |/ _ \| |   | | | | | |_) / _ \  _   _ \ /_  \ \ / / _ \
  / ___ \| | | (_| | (_) | |___| |_| | |  _ <  __/ | | | | | (_) \ V /  __/
 /_/   \_\_|  \__, |\___/ \____|____/  |_| \_\___|_| |_| |_|\___/ \_/ \___|
              |___/`)

	cmd := exec.Command("helm", "delete", "argocd", "-n", "argocd")
	_ = cmd.Run() // ignore error, like '|| true' in shell

	// Remove artifacts
	_ = os.RemoveAll("/tmp/argo-cd") // ignore error, like '|| true' in shell

	return nil
}

func InstallArgoCD() error {
	// Print ASCII art
	fmt.Println(`
     _                     ____ ____
    / \   _ __ __ _  ___  / ___|  _ \
   / _ \ | '__/ _  |/ _ \| |   | | | |
  / ___ \| | | (_| | (_) | |___| |_| |
 /_/   \_\_|  \__, |\___/ \____|____/
              |___/`)

	// Run helm template
	helmTemplateCmd := exec.Command(
		"helm", "template", "-s", "templates/values.tmpl", "/tmp/argo-cd/argo-cd",
		"--values", "/tmp/argo-cd/proxy-values.yaml",
	)

	valuesYaml, err := helmTemplateCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run helm template: %w", err)
	}

	if err := os.WriteFile("/tmp/argo-cd/values.yaml", valuesYaml, 0o644); err != nil {
		return fmt.Errorf("failed to write values.yaml: %w", err)
	}

	// Remove values.tmpl
	if err := os.Remove("/tmp/argo-cd/argo-cd/templates/values.tmpl"); err != nil {
		return fmt.Errorf("failed to remove values.tmpl: %w", err)
	}

	// Write mounts.yaml
	mountsYaml := `
notifications:
  extraVolumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  - mountPath: /etc/ssl/certs/gitea_cert.crt
    name: gitea-tls
  extraVolumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
  - name: gitea-tls
    hostPath:
      path: /usr/local/share/ca-certificates/gitea_cert.crt
server:
  volumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  - mountPath: /etc/ssl/certs/gitea_cert.crt
    name: gitea-tls
  volumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
  - name: gitea-tls
    hostPath:
      path: /usr/local/share/ca-certificates/gitea_cert.crt
repoServer:
  volumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  - mountPath: /etc/ssl/certs/gitea_cert.crt
    name: gitea-tls
  volumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
  - name: gitea-tls
    hostPath:
      path: /usr/local/share/ca-certificates/gitea_cert.crt
applicationSet:
  extraVolumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  - mountPath: /etc/ssl/certs/gitea_cert.crt
    name: gitea-tls
  extraVolumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
  - name: gitea-tls
    hostPath:
      path: /usr/local/share/ca-certificates/gitea_cert.crt
`
	if err := os.WriteFile("/tmp/argo-cd/mounts.yaml", []byte(mountsYaml), 0o644); err != nil {
		return fmt.Errorf("failed to write mounts.yaml: %w", err)
	}

	// Run helm install
	helmInstallCmd := exec.Command(
		"helm", "install", "argocd", "/tmp/argo-cd/argo-cd",
		"--values", "/tmp/argo-cd/values.yaml",
		"-f", "/tmp/argo-cd/mounts.yaml",
		"-n", "argocd", "--create-namespace",
	)
	helmInstallCmd.Stdout = os.Stdout
	helmInstallCmd.Stderr = os.Stderr
	if err := helmInstallCmd.Run(); err != nil {
		return fmt.Errorf("failed to run helm install: %w", err)
	}

	return nil
}

func argocdValues(config config.OrchInstallerConfig) error {
	//nolint
	valuesFile := `
server:
  service:
    type: LoadBalancer
configs:
  params:
    application.namespaces: "*"
    server.tls.minversion: "1.2"
    # // Note that for TLS v1.3, cipher suites are not configurable and will be chosen automatically.
    server.tls.ciphers: "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:TLS_AES_256_GCM_SHA384"
  # https://argo-cd.readthedocs.io/en/stable/operator-manual/health/#argocd-app
  cm:
    resource.customizations: |
      argoproj.io/Application:
        health.lua: |
          hs = {}
          hs.status = "Progressing"
          hs.message = ""
          if obj.status ~= nil then
            if obj.status.health ~= nil then
              hs.status = obj.status.health.status
              if obj.status.health.message ~= nil then
                hs.message = obj.status.health.message
              end
            end
          end
          return hs
    users.session.duration: "1h"
{{ if or .Values.http_proxy .Values.https_proxy }}
global:
  env:
    {{ if .Values.http_proxy }}
    - name: http_proxy
      value: {{ .Values.http_proxy }}
    {{ end }}
    {{ if .Values.https_proxy }}
    - name: https_proxy
      value: {{ .Values.https_proxy }}
    {{ end }}
    - name: no_proxy
      value: "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.0/8,localhost,.svc,.local,argocd-repo-server,argocd-application-controller,argocd-metrics,argocd-server,argocd-server-metrics,argocd-redis,argocd-dex-server,{{ .Values.no_proxy }}"
{{ end }}

# Disabled due to vulnerability report and we are not using it
dex:
  enabled: false
`
	// revive:enable:dupword

	path := "/tmp/argo-cd/argo-cd/templates/"
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	if err := chownToCurrentUserRecursive("/tmp/argo-cd/"); err != nil {
		return fmt.Errorf("failed to change ownership of /tmp/argo-cd/: %w", err)
	}

	if err := addArgoHelmRepo(); err != nil {
		return fmt.Errorf("failed to add helm template: %w", err)
	}

	if err := os.WriteFile("/tmp/argo-cd/argo-cd/templates/values.tmpl", []byte(valuesFile), 0o644); err != nil {
		return fmt.Errorf("failed to copy values.tmpl: %w", err)
	}

	proxyYaml := fmt.Sprintf("http_proxy: %s\nhttps_proxy: %s\nno_proxy: %s\n", config.Proxy.HTTPProxy, config.Proxy.HTTPProxy, config.Proxy.NoProxy)
	if err := os.WriteFile("/tmp/argo-cd/proxy-values.yaml", []byte(proxyYaml), 0o644); err != nil {
		return fmt.Errorf("failed to write proxy-values.yaml: %w", err)
	}
	return nil
}
