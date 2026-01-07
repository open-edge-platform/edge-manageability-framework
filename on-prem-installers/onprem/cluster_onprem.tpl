
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Cluster specific values applied to root-app only
root:
  useLocalValues: false
  clusterValues:
    - orch-configs/profiles/enable-platform.yaml
    ${O11Y_ENABLE_PROFILE}
    ${KYVERNO_PROFILE}
    ${AO_PROFILE}
    ${CO_PROFILE}
    ${EDGEINFRA_PROFILE}
    - orch-configs/profiles/enable-full-ui.yaml
    ${ONPREM_PROFILE}
    ${SRE_PROFILE}
    - orch-configs/profiles/proxy-none.yaml
    ${PROFILE_FILE_NAME}
    ${PROFILE_FILE_NAME_EXT}
    ${EMAIL_PROFILE}
    ${O11Y_PROFILE}
    ${SINGLE_TENANCY_PROFILE}
    ${EXPLICIT_PROXY_PROFILE}
    - orch-configs/profiles/resource-default.yaml
    - orch-configs/profiles/artifact-rs-production-noauth.yaml
    - orch-configs/clusters/${ORCH_INSTALLER_PROFILE}.yaml

# Values applied to both root app and shared among all child apps
argo:
  ## Basic cluster information
  project: ${CLUSTER_NAME}
  namespace: ${CLUSTER_NAME}
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
  targetCluster: ${CLUSTER_NAME}

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
