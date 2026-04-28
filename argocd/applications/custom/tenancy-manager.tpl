# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/tenancy-manager
  tag: "nexus-replacement-20260428-0546"
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}

{{- with .Values.argo.resources.tenancyManager }}
resources:
  {{- toYaml . | nindent 4 }}
{{- end }}
postgresql:
  existingSecret: "iam-tenancy-local-postgresql"
  existingSecretPasswordKey: "PGPASSWORD"
  host: "postgresql-cluster-rw.orch-database"
  database: "orch-iam-iam-tenancy"
  username: "orch-iam-iam-tenancy_user"

# Registered controllers list. App-orch controllers (app-orch-tenant-controller,
# app-deployment-manager) are conditionally included only when app-orch is
# enabled in the deployment profile (argo.enabled.app-orch-tenant-controller).
# This keeps the registered controller list accurate so projects don't get
# stuck waiting for controllers that are not deployed.
#
# Note: the chart uses .Values.tenancyManager.* for controller configuration.
tenancyManager:
  oidcServerURL: "http://platform-keycloak.orch-platform.svc:8080/realms/master"
  defaultOrgName: ""
  defaultProjectName: ""
  controllers:
    org:
      - keycloak-tenant-controller
    project:
      {{- if (index .Values.argo.enabled "app-orch-tenant-controller") }}
      - app-orch-tenant-controller
      - app-deployment-manager
      {{- end }}
      - keycloak-tenant-controller
      - infra-tenant-controller
      - cluster-manager
      - observability-tenant-controller
      - metadata-broker
