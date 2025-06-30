# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

global:
  registry:
    name: "{{ .Values.argo.containerRegistryURL }}/"
    {{- $imagePullSecretsLength := len .Values.argo.imagePullSecrets }}
    {{- if eq $imagePullSecretsLength 0 }}
    imagePullSecrets: []
    {{- else }}
    imagePullSecrets:
    {{- with .Values.argo.imagePullSecrets }}
      {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- end }}

import:
  onboarding-manager:
    enabled: {{ index .Values.argo "infra-onboarding" "onboarding-manager" "enabled" }}
  pxe-server:
    enabled: {{ index .Values.argo "infra-onboarding" "pxe-server" "enabled" }}

infra-config:
  config:
    enProxyHTTP: {{ .Values.argo.proxy.enHttpProxy }}
    enProxyHTTPS: {{ .Values.argo.proxy.enHttpsProxy }}
    enProxyFTP: {{ .Values.argo.proxy.enFtpProxy }}
    enProxySocks: {{ .Values.argo.proxy.enSocksProxy }}
    enProxyNoProxy: {{ .Values.argo.proxy.enNoProxy }}

    orchInfra: infra-node.{{ .Values.argo.clusterDomain }}:443
    orchCluster: cluster-orch-node.{{ .Values.argo.clusterDomain }}:443
    orchUpdate: update-node.{{ .Values.argo.clusterDomain }}:443
    orchRelease: release.{{ .Values.argo.clusterDomain }}
    orchPlatformObsLogs: logs-node.{{ .Values.argo.clusterDomain }}:443
    orchPlatformObsMetrics: metrics-node.{{ .Values.argo.clusterDomain }}:443
    orchKeycloak: keycloak.{{ .Values.argo.clusterDomain }}:443
    orchTelemetry: telemetry-node.{{ .Values.argo.clusterDomain }}:443
    orchAttestationStatus: attest-node.{{ .Values.argo.clusterDomain }}:443
    orchRegistry: {{ .Values.argo.releaseService.ociRegistry }}:9443
    orchFileServer: {{ .Values.argo.releaseService.fileServer }}:60444

    rsType: "{{ index .Values.argo "infra-onboarding" "rsType" | default "no-auth" }}"
    netIp: "{{ index .Values.argo "infra-onboarding" "netIp" | default "dynamic" }}"
    ntpServer: "{{ index .Values.argo "infra-onboarding" "ntpServer" | default "ntp1.server.org,ntp2.server.org" }}"
    {{- $nameServers := index .Values.argo "infra-onboarding" "nameServers" | default list }}
    {{- $nameServersLength := len $nameServers }}
    {{- if eq $nameServersLength 0 }}
    nameServers: []
    {{- else }}
    nameServers:
      {{- toYaml $nameServers | nindent 6 }}
    {{- end }}
    systemConfigFsInotifyMaxUserInstances: "{{ index .Values.argo "infra-onboarding" "systemConfigFsInotifyMaxUserInstances" | default "8192" }}"
    systemConfigVmOverCommitMemory: "{{ index .Values.argo "infra-onboarding" "systemConfigVmOverCommitMemory" | default "1" }}"
    systemConfigKernelPanicOnOops: "{{ index .Values.argo "infra-onboarding" "systemConfigKernelPanicOnOops" | default "1" }}"
    systemConfigKernelPanic: "{{ index .Values.argo "infra-onboarding" "systemConfigKernelPanic" | default "10" }}"

    cdnSvc: {{ .Values.argo.releaseService.fileServer }}
    provisioningSvc: tinkerbell-nginx.{{ .Values.argo.clusterDomain }}
    tinkerSvc: tinkerbell-server.{{ .Values.argo.clusterDomain }}
    omSvc: onboarding-node.{{ .Values.argo.clusterDomain }}
    omStreamSvc: onboarding-stream.{{ .Values.argo.clusterDomain }}
    extraHosts: "{{ index .Values.argo "infra-onboarding" "dkamExtraHost" | default "" }}"

    firewallReqAllow: |-
      [{
        "sourceIp": "{{ .Values.argo.clusterDomain }}",
        "ports": "6443,10250",
        "ipVer": "ipv4",
        "protocol": "tcp"
      }]
    firewallCfgAllow:
    {{- with index .Values.argo "infra-onboarding" "firewallCfgAllow" }}
      {{- toYaml . | nindent 6 }}
    {{- end }}

tinkerbell:
  pvc:
    storageClassName: {{ index .Values.argo "infra-onboarding" "tinkerbellStorageClass" | default "standard" }}
  traefikReverseProxy:
    enabled: &traefikReverseProxy_enabled true
    tinkServerDnsname: "tinkerbell-server.{{ .Values.argo.clusterDomain }}"
    nginxDnsname: &nginxDnsname "tinkerbell-nginx.{{ .Values.argo.clusterDomain }}"
  stack:
    resources:
      limits:
        {{- if .Values.argo.nginxCDN }}
        cpu: {{ .Values.argo.nginxCDN.resources.limits.cpu }}
        memory: {{ .Values.argo.nginxCDN.resources.limits.memory }}
        {{- else }}
        cpu: "64"
        memory: "64Gi"
      {{- end }}
      requests:
        {{- if .Values.argo.nginxCDN }}
        cpu: {{ .Values.argo.nginxCDN.resources.requests.cpu }}
        memory: {{ .Values.argo.nginxCDN.resources.requests.memory }}
        {{- else }}
        cpu: 200m
        memory: 256Mi
        {{- end }}
  tinkerbell_tink:
    server:
      metrics:
        enabled: {{ index .Values.argo "infra-onboarding" "enableMetrics" | default false }}
  tinkerbell_smee:
    # these additional arguments serve as kernel arguments for Edge Node
    additionalKernelArgs:
      - "http_proxy={{ .Values.argo.proxy.enHttpProxy }}"
      - "https_proxy={{ .Values.argo.proxy.enHttpsProxy }}"
      - "no_proxy={{ .Values.argo.proxy.enNoProxy }}"
      - "HTTP_PROXY={{ .Values.argo.proxy.enHttpProxy }}"
      - "HTTPS_PROXY={{ .Values.argo.proxy.enHttpsProxy }}"
      - "NO_PROXY={{ .Values.argo.proxy.enNoProxy }}"
      - "DEBUG=false"
      - "TIMEOUT=120s"
      - "syslog_host=127.0.0.1"
    traefikReverseProxy:
      enabled: *traefikReverseProxy_enabled
      nginxDnsname: *nginxDnsname

dkam:
  pvc:
    storageClassName: {{ index .Values.argo "infra-onboarding" "dkamStorageClass" | default "standard" }}
  managerArgs:
    enableTracing: {{ index .Values.argo "infra-onboarding" "enableTracing" | default false }}
{{- if and (index .Values.argo "infra-external") (index .Values.argo "infra-external" "loca") }}
    legacyMode: true
{{- end }}
  metrics:
    enabled: {{ index .Values.argo "infra-onboarding" "enableMetrics" | default false }}
  env:
    mode: "{{ index .Values.argo "infra-onboarding" "dkamMode" | default "prod" }}"
  proxies:
    http_proxy: {{ .Values.argo.proxy.httpProxy }}
    https_proxy: {{ .Values.argo.proxy.httpsProxy }}
    no_proxy: {{ .Values.argo.proxy.noProxy }}
  {{- if index .Values.argo "infra-onboarding" "dkam" }}
  {{- if index .Values.argo "infra-onboarding" "dkam" "resources" }}
  resources:
  {{- with index .Values.argo "infra-onboarding" "dkam" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}

onboarding-manager:
  managerArgs:
    enableTracing: {{ index .Values.argo "infra-onboarding" "enableTracing" | default false }}
  metrics:
    enabled: {{ index .Values.argo "infra-onboarding" "enableMetrics" | default false }}
  pvc:
    storageClassName: {{ index .Values.argo "infra-onboarding" "onboardingManagerStorageClass" | default "standard" }}
  traefikReverseProxy:
    host:
      grpc:
        name: "onboarding-node.{{ .Values.argo.clusterDomain }}"
      stream:
        name: "onboarding-stream.{{ .Values.argo.clusterDomain }}"
{{- if .Values.argo.traefik }}
    tlsOption: {{ .Values.argo.traefik.tlsOption | default "" | quote }}
{{- end }}
  env:
    mode: "{{ index .Values.argo "infra-onboarding" "dkamMode" | default "prod" }}"
    userName: "{{ index .Values.argo "infra-onboarding" "userName" | default "user" }}"
    passWord: "{{ index .Values.argo "infra-onboarding" "passWord" | default "user" }}"
    enableTinkActionTimestamp: {{ index .Values.argo "infra-onboarding" "enableTinkActionTimestamp" | default false }}
  {{- if index .Values.argo "infra-onboarding" "onboarding-manager" }}
  {{- if index .Values.argo "infra-onboarding" "onboarding-manager" "resources" }}
  resources:
  {{- with index .Values.argo "infra-onboarding" "onboarding-manager" "resources" }}
    {{- toYaml . | nindent 4 }}
  {{- end}}
  {{- end}}
  {{- end}}

{{- if index .Values.argo "infra-onboarding" "pxe-server" "enabled" }}
pxe-server:
  config:
    interface: {{ index .Values.argo "infra-onboarding" "pxe-server" "interface" | default "" }}
    bootServerIP: {{ index .Values.argo "infra-onboarding" "pxe-server" "bootServerIP" | default "" }}
    subnetAddress: {{ index .Values.argo "infra-onboarding" "pxe-server" "subnetAddress" | default "" }}
{{- end }}
