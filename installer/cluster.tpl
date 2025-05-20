# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Cluster specific values applied to root-app only
root:
  useLocalValues: true
  clusterValues:
    - orch-configs/profiles/enable-platform.yaml
    - orch-configs/profiles/enable-multitenancy.yaml
    - orch-configs/profiles/enable-o11y.yaml
    - orch-configs/profiles/enable-kyverno.yaml
    - orch-configs/profiles/enable-app-orch.yaml
    - orch-configs/profiles/enable-cluster-orch.yaml
    - orch-configs/profiles/enable-edgeinfra.yaml
    - orch-configs/profiles/enable-full-ui.yaml
    - orch-configs/profiles/enable-aws.yaml
    ${SRE_PROFILE}
    # proxy group should be specified as the first post-"enable" profile
    - orch-configs/profiles/proxy-none.yaml
    - orch-configs/profiles/profile-aws.yaml
    - orch-configs/profiles/resource-default.yaml
    ${AWS_PROD_PROFILE}
    ${O11Y_PROFILE}
    ${EMAIL_PROFILE}
    ${AUTOCERT_PROFILE}
    - orch-configs/profiles/artifact-rs-production-noauth.yaml
    - orch-configs/clusters/${CLUSTER_NAME}.yaml

# Values applied to both root app and shared among all child apps
argo:
  ## Basic cluster information
  project: ${CLUSTER_NAME}
  namespace: ${CLUSTER_NAME}
  clusterName: ${CLUSTER_NAME}
  clusterDomain: ${CLUSTER_FQDN}
  adminEmail: ${ADMIN_EMAIL}

  deployRepoURL: https://gitea.${CLUSTER_FQDN}/argocd/edge-manageability-framework
  deployRepoRevision: main

  git:
    server: https://gitea.${CLUSTER_FQDN}

  targetServer: "https://kubernetes.default.svc"
  autosync: true

  o11y:
    # If the cluster has a node dedicated to edgenode observability services
    dedicatedEdgenodeEnabled: true

  ## AWS Account Info
  aws:
    account: "${AWS_ACCOUNT}"
    region: ${AWS_REGION}
    bucketPrefix: ${CLUSTER_NAME}-${S3_PREFIX}
    efs:
      repository: 602401143452.dkr.ecr.${AWS_REGION}.amazonaws.com/eks/aws-efs-csi-driver
      fsid: "${FILE_SYSTEM_ID}"
    targetGroup:
      traefik: "${TRAEFIK_TG_ARN}"
      traefikGrpc: "${TRAEFIKGRPC_TG_ARN}"
      # nginx: "${NGINX_TG_ARN}"
      argocd: "${ARGOCD_TG_ARN}"

  traefik:
    tlsOption: ""

orchestratorDeployment:
  targetCluster: cloud

postCustomTemplateOverwrite: {}
