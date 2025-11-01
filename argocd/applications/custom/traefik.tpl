# SPDX-FileCopyrightText: 2025 Intel Corporation
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
    port: 8080
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
    # NOTE the middlewared name is <namespace>-<name>
    middlewares:
      - orch-gateway-rate-limit@kubernetescrd
      {{- if index .Values "argo" "cors"}}
      {{- if index .Values "argo" "cors" "enabled" }}
      - orch-gateway-cors@kubernetescrd
      {{- end }}
      {{- end }}
  tcpamt:
    middlewares:
      - orch-gateway-tcp-rate-limit@kubernetescrd
    port: 4433
    exposedPort: 4433
    expose:
      default: true
    protocol: TCP
