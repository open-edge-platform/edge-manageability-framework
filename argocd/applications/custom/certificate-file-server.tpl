# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

rootMatchHost: Host(`{{ .Values.argo.clusterDomain }}`)
orchSecretName: {{ .Values.argo.tlsSecret }}

{{- if .Values.argo.traefik }}
tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}

{{- with .Values.argo.resources.certificateFileServer }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}

# Infrastructure configuration for edge node installer
infraConfig:
  # Proxy Configuration
  enProxyHTTP: {{ .Values.argo.proxy.enHttpProxy }}
  enProxyHTTPS: {{ .Values.argo.proxy.enHttpsProxy }}
  enProxyFTP: {{ .Values.argo.proxy.enFtpProxy }}
  enProxySocks: {{ .Values.argo.proxy.enSocksProxy }}
  enProxyNoProxy: {{ .Values.argo.proxy.enNoProxy }}

  # Orchestrator Endpoints
  orchCluster: cluster-orch-node.{{ .Values.argo.clusterDomain }}:443
  orchInfra: infra-node.{{ .Values.argo.clusterDomain }}:443
  orchKeycloak: keycloak.{{ .Values.argo.clusterDomain }}:443
  orchRelease: release.{{ .Values.argo.clusterDomain }}
  orchFileServer: {{ .Values.argo.releaseService.fileServer }}:60444
  orchRegistry: {{ .Values.argo.releaseService.ociRegistry }}:9443
  orchRPSHost: rps.{{ .Values.argo.clusterDomain }}:443
  orchDeviceManager: device-manager-node.{{ .Values.argo.clusterDomain }}:443
  orchAptSrcProxyPort: "60444"

  # Edge Node Service Configuration
  enServiceClients: "{{ index .Values.argo "infra-onboarding" "enServiceClients" | default "platform-manageability-agent" }}"
  enOutboundClients: "{{ index .Values.argo "infra-onboarding" "enOutboundClients" | default " " }}"
  enMetricsEnabled: "{{ index .Values.argo "infra-onboarding" "enMetricsEnabled" | default "false" }}"
  enTokenClients: "{{ index .Values.argo "infra-onboarding" "enTokenClients" | default "node-agent,platform-manageability-agent" }}"

  # Registry and Repository Configuration
  registryService: {{ .Values.argo.releaseService.ociRegistry }}
  enDebianPackagesRepo: "edge-orch/en/deb"
  enFilesRsRoot: "files-edge-orch"
  enManifestRepo: "edge-orch/en/files/ena-manifest"
  enAgentManifestTag: "1.5.8"

  # Release Service Configuration
  rsType: "{{ index .Values.argo "infra-onboarding" "rsType" | default "no-auth" }}"

  # System Configuration
  systemConfigVmOverCommitMemory: "{{ index .Values.argo "infra-onboarding" "systemConfigVmOverCommitMemory" | default "1" }}"
  systemConfigKernelPanic: "{{ index .Values.argo "infra-onboarding" "systemConfigKernelPanic" | default "10" }}"
  systemConfigKernelPanicOnOops: "{{ index .Values.argo "infra-onboarding" "systemConfigKernelPanicOnOops" | default "1" }}"
  systemConfigFsInotifyMaxUserInstances: "{{ index .Values.argo "infra-onboarding" "systemConfigFsInotifyMaxUserInstances" | default "8192" }}"

  # NTP Configuration
  ntpServer: "{{ index .Values.argo "infra-onboarding" "ntpServer" | default "ntp1.server.org,ntp2.server.org" }}"

  # Onboarding Manager Configuration
  omSvc: onboarding-node.{{ .Values.argo.clusterDomain }}
  omStreamSvc: onboarding-stream.{{ .Values.argo.clusterDomain }}

  # Firewall Configuration
  firewallReqAllow: |-
    [{
      "sourceIp": "{{ .Values.argo.clusterDomain }}",
      "ports": "6443,10250",
      "ipVer": "ipv4",
      "protocol": "tcp"
    }]
  firewallCfgAllow:
  {{- with index .Values.argo "infra-onboarding" "firewallCfgAllow" }}
    {{- toYaml . | nindent 4 }}
  {{- end }}

