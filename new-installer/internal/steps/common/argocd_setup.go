// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package common

import (
  "fmt"
  "io/ioutil"
	"os"

	"context"
	"os/exec"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

type ArgoCDStep struct {
	RootPath               string
	KeepGeneratedFiles     bool
	orchConfigReaderWriter config.OrchConfigReaderWriter
	StepLabels             []string
}

func CreateGenericStep(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) *ArgoCDStep {
	return &ArgoCDStep{
		RootPath:               rootPath,
		KeepGeneratedFiles:     keepGeneratedFiles,
		orchConfigReaderWriter: orchConfigReaderWriter,
	}
}

func (s *ArgoCDStep) Name() string {
	return "GenericStep"
}

func (s *ArgoCDStep) Labels() []string {
	return s.StepLabels
}

func (s *ArgoCDStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	argocdValues(config)
	return runtimeState, nil
}

func (s *ArgoCDStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// no-op for now
	return runtimeState, nil
}

func (s *ArgoCDStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	// InstallArgoCD
	return runtimeState, nil
}

func (s *ArgoCDStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	    // Wait for ArgoCD namespace creation normally taken from an env var
    argoCDNS := "argocd"
    if argoCDNS == "" {
        argoCDNS = "argocd"
    }
	err := WaitForNamespaceCreation(argoCDNS);
    if  err != nil {
        // return nil,fmt.Errorf("failed to wait for ArgoCD namespace: %v", err)
		fmt.Printf("failed to wait for ArgoCD namespace: %v\n", err)
    }
	
	return runtimeState, prevStepError
}



// func InstallArgoCD() error {
//     // Print ASCII art
//     fmt.Println(`
//      _                     ____ ____
//     / \   _ __ __ _  ___  / ___|  _ \
//    / _ \ | '__/ _  |/ _ \| |   | | | |
//   / ___ \| | | (_| | (_) | |___| |_| |
//  /_/   \_\_|  \__, |\___/ \____|____/
//               |___/
// `)

   

//     // Run helm template
//     helmTemplateCmd := exec.Command(
//         "helm", "template", "-s", "templates/values.tmpl", "/tmp/argo-cd/argo-cd/",
//         "--values", "/tmp/argo-cd/proxy-values.yaml",
//     )
//     valuesYaml, err := helmTemplateCmd.Output()
//     if err != nil {
//         return fmt.Errorf("failed to run helm template: %v", err)
//     }
//     if err := ioutil.WriteFile("/tmp/argo-cd/values.yaml", valuesYaml, 0644); err != nil {
//         return fmt.Errorf("failed to write values.yaml: %v", err)
//     }

//     // Remove values.tmpl
//     if err := os.Remove("/tmp/argo-cd/argo-cd/templates/values.tmpl"); err != nil {
//         return fmt.Errorf("failed to remove values.tmpl: %v", err)
//     }

//     // Write mounts.yaml
//     mountsYaml := `
// notifications:
//   extraVolumeMounts:
//   - mountPath: /etc/ssl/certs/ca-certificates.crt
//     name: tls-from-node
//   - mountPath: /etc/ssl/certs/gitea_cert.crt
//     name: gitea-tls
//   extraVolumes:
//   - name: tls-from-node
//     hostPath:
//       path: /etc/ssl/certs/ca-certificates.crt
//   - name: gitea-tls
//     hostPath:
//       path: /usr/local/share/ca-certificates/gitea_cert.crt
// server:
//   volumeMounts:
//   - mountPath: /etc/ssl/certs/ca-certificates.crt
//     name: tls-from-node
//   - mountPath: /etc/ssl/certs/gitea_cert.crt
//     name: gitea-tls
//   volumes:
//   - name: tls-from-node
//     hostPath:
//       path: /etc/ssl/certs/ca-certificates.crt
//   - name: gitea-tls
//     hostPath:
//       path: /usr/local/share/ca-certificates/gitea_cert.crt
// repoServer:
//   volumeMounts:
//   - mountPath: /etc/ssl/certs/ca-certificates.crt
//     name: tls-from-node
//   - mountPath: /etc/ssl/certs/gitea_cert.crt
//     name: gitea-tls
//   volumes:
//   - name: tls-from-node
//     hostPath:
//       path: /etc/ssl/certs/ca-certificates.crt
//   - name: gitea-tls
//     hostPath:
//       path: /usr/local/share/ca-certificates/gitea_cert.crt
// applicationSet:
//   extraVolumeMounts:
//   - mountPath: /etc/ssl/certs/ca-certificates.crt
//     name: tls-from-node
//   - mountPath: /etc/ssl/certs/gitea_cert.crt
//     name: gitea-tls
//   extraVolumes:
//   - name: tls-from-node
//     hostPath:
//       path: /etc/ssl/certs/ca-certificates.crt
//   - name: gitea-tls
//     hostPath:
//       path: /usr/local/share/ca-certificates/gitea_cert.crt
// `
//     if err := ioutil.WriteFile("/tmp/argo-cd/mounts.yaml", []byte(mountsYaml), 0644); err != nil {
//         return fmt.Errorf("failed to write mounts.yaml: %v", err)
//     }

//     // Run helm install
//     helmInstallCmd := exec.Command(
//         "helm", "install", "argocd", "/tmp/argo-cd/argo-cd",
//         "--values", "/tmp/argo-cd/values.yaml",
//         "-f", "/tmp/argo-cd/mounts.yaml",
//         "-n", "argocd", "--create-namespace",
//     )
//     helmInstallCmd.Stdout = os.Stdout
//     helmInstallCmd.Stderr = os.Stderr
//     if err := helmInstallCmd.Run(); err != nil {
//         return fmt.Errorf("failed to run helm install: %v", err)
//     }

//     return nil
// }

func argocdValues(config config.OrchInstallerConfig){
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

    if err := ioutil.WriteFile("/tmp/argo-cd/argo-cd/templates/values.tmpl", []byte(valuesFile), 0644); err != nil {
      fmt.Printf("failed to copy values.tmpl: %v", err)
      //return fmt.Errorf("failed to copy values.tmpl: %v", err)  
    }

    // // Write proxy-values.yaml
    // httpProxy := os.Getenv("http_proxy")
    // httpsProxy := os.Getenv("https_proxy")
func argocdValues(config config.OrchInstallerConfig){

    // noProxy := os.Getenv("no_proxy")
    proxyYaml := fmt.Sprintf("http_proxy: %s\nhttps_proxy: %s\nno_proxy: %s\n", config.Proxy.NoProxy, config.Proxy.NoProxy, config.Proxy.NoProxy)
    if err := ioutil.WriteFile("/tmp/argo-cd/proxy-values.yaml", []byte(proxyYaml), 0644); err != nil {
        // return fmt.Errorf("failed to write proxy-values.yaml: %v", err)
		fmt.Printf("failed to write proxy-values.yaml: %v\n", err)
    }



}