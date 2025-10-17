# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

namespace: orch-platform
image:
  registry: {{.Values.argo.containerRegistryURL }}
  repository: common/keycloak-tenant-controller
proxy:
  httpProxy: {{.Values.argo.proxy.httpProxy}}
  httpsProxy: {{.Values.argo.proxy.httpsProxy}}
  noProxy: {{.Values.argo.proxy.noProxy}}
imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 2 }}
  {{- end }}
serviceAccount:
  # Annotations to add to the service account
  annotations: {}
  # The name of the service account to use.
  name: orch-svc
podSecurityContext:
  seccompProfile:
    type: RuntimeDefault
securityContext:
  capabilities:
    drop:
      - ALL
  allowPrivilegeEscalation: false
keycloakAdmin:
  # Use ktc-m2m-client service account with client credentials grant
  # The ktc-client-secret-job creates this secret with the client credentials
  client: ktc-m2m-client
  clientSecret:
    name: ktc-m2m-client # name of the secret created by ktc-client-secret-job
    key: client-secret # key containing the client secret
keycloak_realm: "master"
argo:
  clusterDomain: {{.Values.argo.clusterDomain}}
keycloak_si_groups: ""
keycloak_org_groups: |-
  {
    "<org-id>_Project-Manager-Group": [
      "<org-id>_project-read-role",
      "<org-id>_project-write-role",
      "<org-id>_project-update-role",
      "<org-id>_project-delete-role"
    ]
  }
keycloak_proj_groups: |-
  {
    "<project-id>_Edge-Node-M2M-Service-Account": [
      "rs-access-r",
      "rs-proxy-r",
      "<project-id>_cat-r",
      "<project-id>_reg-r",
      "<project-id>_en-agent-rw"
    ],
    "<project-id>_Edge-Manager-Group": [
      "account/manage-account",
      "account/view-profile",
      "<project-id>_tc-r",
      "<project-id>_ao-rw",
      "<project-id>_cat-rw",
      "<project-id>_cl-rw",
      "<project-id>_cl-tpl-rw",
      "<project-id>_reg-a",
      "<project-id>_reg-r",
      "<project-id>_im-r",
      "<project-id>_alrt-rw",
      "<org-id>_<project-id>_m"
    ],
    "<project-id>_Edge-Onboarding-Group": [
      "rs-access-r",
      "<project-id>_en-ob"
    ],
    "<project-id>_Edge-Operator-Group": [
      "account/manage-account",
      "account/view-profile",
      "<project-id>_tc-r",
      "<project-id>_ao-rw",
      "<project-id>_cat-r",
      "<project-id>_cl-r",
      "<project-id>_cl-tpl-r",
      "<project-id>_reg-r",
      "<project-id>_im-r",
      "<project-id>_alrt-r",
      "<org-id>_<project-id>_m"
    ],
    "<project-id>_Host-Manager-Group": [
      "account/manage-account",
      "account/view-profile",
      "<project-id>_tc-r",
      "<project-id>_im-rw",
      "<project-id>_en-ob",
      "<org-id>_<project-id>_m"
    ]
  }
{{- with .Values.argo.resources.keycloakTenantController }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
