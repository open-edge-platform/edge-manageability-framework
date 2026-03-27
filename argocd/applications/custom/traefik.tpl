# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

{{- with .Values.argo.resources.traefik }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.argo.traefik.nodeSelector }}
nodeSelector:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.argo.traefik.tolerations }}
tolerations:
  {{- toYaml . | nindent 2 }}
{{- end }}
service:
  {{- if .Values.argo.traefikSvcType }}
  type: {{ .Values.argo.traefikSvcType }}
  {{- else}}
  type: NodePort
  {{- end}}
ports:
{{- if index .Values.argo.enabled "squid-proxy" }}
  squidproxy:
    port: 9000
    exposedPort: 8080
    expose:
      default: true
    protocol: TCP
{{- end }}
  web:
    expose:
      default: false
  websecure:
    nodePort: 30443
    # Breaking change when migrating to v39.0.0. See:
    # https://github.com/traefik/traefik-helm-chart/releases/tag/v39.0.0
    http:
        # NOTE the middlewared name is <namespace>-<name>    
        middlewares:
        - orch-gateway-rate-limit@kubernetescrd
        {{- if index .Values "argo" "cors"}}
        {{- if index .Values "argo" "cors" "enabled" }}
        - orch-gateway-cors@kubernetescrd
        {{- end }}
        {{- end }}
  tcpamt:
    # Breaking change when migrating to v39.0.0. See:
    # https://github.com/traefik/traefik-helm-chart/releases/tag/v39.0.0
    http:
        middlewares:
        - orch-gateway-tcp-rate-limit@kubernetescrd
    port: 4433
    exposedPort: 4433
    expose:
      default: true
    protocol: TCP
