# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/tenancy-init
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}

keycloak:
  service:
    name: "platform-keycloak"
    port: "8080"
    namespace: "keycloak-system"
  roles:
    admin:
      groups: "Project-Manager-Group"
    edge:
      groups: "Edge-Manager-Group,Edge-Onboarding-Group,Edge-Operator-Group,Host-Manager-Group"
