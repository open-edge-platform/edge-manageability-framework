
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Template Notes:
# Placeholder variables expanded by generation script:
#   ${O11Y_PROFILE}             optional observability enable profile line (or blank)
#   ${AO_PROFILE}               optional app orchestrator enable profile (or blank)
#   ${CO_PROFILE}               optional cluster orchestrator enable profile (or blank)
#   ${SRE_PROFILE}              optional SRE enable profile (or blank)
#   ${EMAIL_PROFILE}            optional alert email profile (or blank)
#   ${PROFILE_FILE}             sizing/capacity profile (profile-onprem.yaml, profile-onprem-oxm.yaml, profile-onprem-1k.yaml)
#   ${EDGEINFRA_PROFILE}        edge infrastructure profile (enable-edgeinfra.yaml or enable-edgeinfra-1k.yaml)
#   ${ONPREM_PROFILE}           on-prem base enable profile (enable-onprem.yaml or variant specific)
#   ${ORCH_INSTALLER_PROFILE}   base installer cluster fragment include
#   ${PROJECT_NAME}             argo project / logical project
#   ${NAMESPACE}                namespace for root app
#   ${CLUSTER_NAME}             cluster identifier
#   ${CLUSTER_DOMAIN}           base domain (e.g. cluster.onprem)
#   ${TARGET_CLUSTER}           orchestratorDeployment.targetCluster value
#   ${ARGO_IP}                  MetalLB assigned Argo CD service IP
#   ${TRAEFIK_IP}               MetalLB assigned Traefik service IP
#   ${NGINX_IP}                 MetalLB assigned Ingress NGINX controller service IP
#   ${PLATFORM_PROFILE}  line for enable-platform profile include
#   ${KYVERNO_PROFILE}   line for enable-kyverno profile include
#   ${FULL_UI_PROFILE}   line for enable-full-ui profile include
#   ${PROXY_NONE_PROFILE}       line for proxy-none profile include
#   ${PROFILE_FILE_PROFILE}     line for dynamic sizing profile (wraps ${PROFILE_FILE})
#   ${ARTIFACT_RS_PROFILE}      line for artifact-rs-production-noauth profile
#   ${OSRM_MANUAL_PROFILE}      line for enable-osrm-manual-mode profile
#   ${RESOURCE_DEFAULT_PROFILE} line for resource-default profile
#   ${ORCH_INSTALLER_FILE_NAME}  line for orch-configs/clusters/${ORCH_INSTALLER_PROFILE}.yaml include

# Cluster specific values applied to root-app only
root:
  useLocalValues: false
  clusterValues:
    ${PLATFORM_PROFILE}
    ${O11Y_PROFILE}
    ${KYVERNO_PROFILE}
    ${AO_PROFILE}
    ${CO_PROFILE}
    ${EDGEINFRA_PROFILE}
    ${FULL_UI_PROFILE}
    ${ONPREM_PROFILE}
    ${SRE_PROFILE}
    ${PROXY_NONE_PROFILE}
    ${PROFILE_FILE_NAME}
    ${PROFILE_FILE_NAME_EXT}
    ${EMAIL_PROFILE}
    ${ARTIFACT_RS_PROFILE}
    ${O11Y_ONPREM_PROFILE}
    ${EXPLICIT_PROXY_PROFILE}
    ${OSRM_MANUAL_PROFILE}
    ${RESOURCE_DEFAULT_PROFILE}
    ${ORCH_INSTALLER_FILE_NAME}

# Values applied to both root app and shared among all child apps
argo:
  ## Basic cluster information
  project: ${PROJECT}
  namespace: ${NAMESPACE}
  clusterName: ${CLUSTER_NAME}
  # Base domain name for all Orchestrator services. This base domain will be concatenated with a service's subdomain
  # name to produce the service's domain name. For example, given the domain name of `orchestrator.io`, the Web UI
  # service will be accessible via `web-ui.orchestrator.io`. Not to be confused with the K8s cluster domain.
  clusterDomain: ${CLUSTER_DOMAIN}

  ## Argo CD configs
  utilsRepoURL: "https://gitea-http.gitea.svc.cluster.local/argocd/orch-utils"
  utilsRepoRevision: main
  deployRepoURL: "https://gitea-http.gitea.svc.cluster.local/argocd/edge-manageability-framework"
  deployRepoRevision: main

  targetServer: "https://kubernetes.default.svc"
  autosync: true

  o11y:
    # If the cluster has a node dedicated to edgenode observability services
    dedicatedEdgenodeEnabled: false
    sre:
      customerLabel: local
      tls:
        enabled: ${SRE_TLS_ENABLED}
        caSecretEnabled: ${SRE_DEST_CA_CERT}
orchestratorDeployment:
  targetCluster: ${TARGET_CLUSTER}

# Post custom template overwrite values should go to /root-app/environments/<env>/<appName>.yaml
# This is a placeholder to prevent error when there isn't any overwrite needed
postCustomTemplateOverwrite:
  argocd:
    server:
      service:
        annotations:
          metallb.universe.tf/address-pool: argocd-server
  traefik:
    service:
      annotations:
        metallb.universe.tf/address-pool: traefik
  ingress-nginx:
    controller:
      service:
        annotations:
          metallb.universe.tf/address-pool: ingress-nginx-controller
  metallb-config:
    ArgoIP: ${ARGO_IP}
    TraefikIP: ${TRAEFIK_IP}
    NginxIP: ${NGINX_IP}
