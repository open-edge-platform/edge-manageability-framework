# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/tenancy-manager
  tag: "nexus-replacement-20260422"
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}

{{- with .Values.argo.resources.tenancyManager }}
resources:
  {{- toYaml . | nindent 4 }}
{{- end }}
postgres:
  secrets: iam-tenancy-local-postgresql

# nexus-replacement: override registered controllers to match what is
# actually deployed. Remove app-orch-tenant-controller and
# app-deployment-manager (not deployed in nexus-replacement).
# Use "tenant-controller" (the appName set in infra-core tenant-controller)
# instead of "infra-tenant-controller".
tenancyManager:
  controllers:
    org:
      - keycloak-tenant-controller
    project:
      - keycloak-tenant-controller
      - tenant-controller
      - cluster-manager
      - observability-tenant-controller
      - metadata-broker
