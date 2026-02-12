#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: onprem_orch_install_from_repo.sh
# Description: Installs Edge Orchestrator components directly from this git checkout
#              (no .deb packages). It installs (optionally) Gitea + ArgoCD via Helm,
#              creates required namespaces/secrets, then bootstraps the deployment
#              by installing the root-app Helm chart from this repository.
#              ArgoCD is always configured to use the GitHub deployment repository
#              (not in-cluster Gitea) as the Git source.
#
# Usage: ./onprem_orch_install_from_repo.sh [OPTIONS]
#   Options (mirrors onprem_orch_install.sh where applicable):
#     -h, --help            Show help message
#     -s, --sre [PATH]      Enable SRE TLS with optional CA certificate path
#     -d, --notls           Disable SMTP TLS verification
#     -y, --yes             Assume 'yes' to all prompts and run non-interactively
#     --disable-co          Disable Cluster Orchestrator profile
#     --disable-ao          Disable Application Orchestrator profile (skips Gitea)
#     --disable-o11y        Disable Observability profile
#     -st, --single_tenancy Enable single tenancy mode
#     -t, --trace           Enable debug tracing

set -e
set -o pipefail

# Import shared functions
# shellcheck disable=SC1091
source "$(dirname "$0")/functions.sh"

cwd=$(pwd)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_URL="https://github.com/open-edge-platform/edge-manageability-framework"

repo_root=""
onprem_installers_dir=""
BOOTSTRAP_TMP_REPO_DIR=""

require_cmd() {
  local cmd=$1
  if ! command -v "$cmd" >/dev/null 2>&1; then
    return 1
  fi
  return 0
}

install_helm() {
  if require_cmd helm; then
    return 0
  fi

  if ! require_cmd sudo; then
    echo "❌ helm is required but is not installed, and sudo is not available to install it automatically"
    echo "   Install helm (https://helm.sh/docs/intro/install/) and retry."
    exit 1
  fi

  if ! require_cmd curl && ! require_cmd wget; then
    echo "❌ helm is required but is not installed. Need curl or wget to install it automatically."
    echo "   Install curl (or wget) and retry, or install helm manually (https://helm.sh/docs/intro/install/)."
    exit 1
  fi

  if [[ "${ASSUME_YES:-false}" != "true" ]]; then
    while true; do
      read -rp "helm not found. Install helm v3 now (requires sudo + internet)? [yes/no] " yn
      case $yn in
        [Yy]* ) break;;
        [Nn]* )
          echo "❌ helm is required. Install helm and retry."
          exit 1
        ;;
        * ) echo "Please answer yes or no.";;
      esac
    done
  fi

  echo "Installing helm..."
  local tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  local installer_url="https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3"
  local installer_path="$tmp/get-helm-3.sh"

  if require_cmd curl; then
    curl -fsSL "$installer_url" -o "$installer_path"
  else
    wget -qO "$installer_path" "$installer_url"
  fi
  chmod +x "$installer_path"

  if [[ -n "${HELM_VERSION:-}" ]]; then
    sudo -E env DESIRED_VERSION="${HELM_VERSION}" "$installer_path"
  else
    sudo -E "$installer_path"
  fi

  if ! require_cmd helm; then
    echo "❌ helm installation did not succeed; please install helm manually and retry."
    exit 1
  fi
}

ensure_prereqs() {
  if ! require_cmd kubectl; then
    echo "❌ kubectl not found. Install kubectl (or ensure RKE2 provides it) and retry."
    exit 1
  fi

  if ! require_cmd yq; then
    echo "yq not found; installing yq..."
    install_yq
  fi

  if ! require_cmd envsubst && ! require_cmd python3; then
    echo "❌ Need 'envsubst' (gettext-base) or 'python3' for cluster YAML generation."
    echo "   Install gettext-base or python3 and retry."
    exit 1
  fi

  install_helm
}

bootstrap_repo_root() {
  local candidate
  candidate="$(cd "$SCRIPT_DIR/../.." && pwd)"

  if [[ -d "$candidate/orch-configs" && -d "$candidate/argocd" && -d "$candidate/on-prem-installers" ]]; then
    repo_root="$candidate"
    onprem_installers_dir="$repo_root/on-prem-installers"
    return 0
  fi

  if ! command -v git >/dev/null 2>&1; then
    echo "❌ git not found and script is not running from an edge-manageability-framework checkout"
    echo "   Install git or run this script from a cloned repo."
    exit 1
  fi

  echo "Repo files not found next to this script; cloning ${REPO_URL} to a temp directory..."
  BOOTSTRAP_TMP_REPO_DIR="$(mktemp -d)"
  git clone --depth 1 "$REPO_URL" "$BOOTSTRAP_TMP_REPO_DIR" >/dev/null 2>&1

  repo_root="$BOOTSTRAP_TMP_REPO_DIR"
  onprem_installers_dir="$repo_root/on-prem-installers"

  if [[ ! -d "$repo_root/orch-configs" || ! -d "$repo_root/argocd" || ! -d "$repo_root/on-prem-installers" ]]; then
    echo "❌ Bootstrap clone does not look like a valid edge-manageability-framework repo: $repo_root"
    exit 1
  fi

  trap 'rm -rf "$BOOTSTRAP_TMP_REPO_DIR"' EXIT
}

generate_cluster_yaml_onprem_from_upstream() {
  local out_file="$cwd/${ORCH_INSTALLER_PROFILE}.yaml"

  if [[ "$ASSUME_YES" == "true" ]]; then
    if [[ -z "${ARGO_IP:-}" || -z "${TRAEFIK_IP:-}" || -z "${NGINX_IP:-}" ]]; then
      echo "❌ ARGO_IP, TRAEFIK_IP, and NGINX_IP must be set when running non-interactively (-y)"
      exit 1
    fi
  fi

  echo "Generating cluster config using installer/generate_cluster_yaml.sh onprem..."
  ONPREM_ENV_PATH="$MAIN_ENV_CONFIG" bash "$repo_root/installer/generate_cluster_yaml.sh" onprem >/dev/null

  if [[ ! -r "$out_file" ]]; then
    echo "❌ Cluster config was not generated: $out_file"
    exit 1
  fi

  echo "Generated cluster config: $out_file"
}

ASSUME_YES=false
ENABLE_TRACE=false
SINGLE_TENANCY_PROFILE=false
INSTALL_GITEA="true"

argo_cd_ns="argocd"
gitea_ns="gitea"
si_config_repo="edge-manageability-framework"

GITEA_CHART_VERSION="10.4.0"
ARGOCD_CHART_VERSION="8.2.7"

MAIN_ENV_CONFIG="$SCRIPT_DIR/onprem.env"

usage() {
  cat >&2 <<EOF
Purpose:
Install OnPrem Edge Orchestrator main components directly from this repository checkout.
No .deb packages are used.

Prerequisites:
- onprem_pre_install.sh or equivalent must have completed successfully (RKE2 running)
- onprem.env file must exist with proper configuration
- helm, kubectl must be available
- sudo access (for installing CA certs for local Gitea TLS)

Usage:
$(basename "$0") [OPTIONS]

Options:
  -h, --help                 Show this help message and exit
  -s, --sre [CA_CERT_PATH]   Enable TLS for SRE exporter; optionally provide CA cert path
  -d, --notls                Disable TLS verification for SMTP endpoint
  -y, --yes                  Assume 'yes' to all prompts (non-interactive)
  --disable-co               Disable Cluster Orchestrator profile
  --disable-ao               Disable Application Orchestrator profile (skips Gitea)
  --disable-o11y             Disable Observability profile
  -st, --single_tenancy      Enable single tenancy mode
  -t, --trace                Enable bash debug tracing (set -x)

Notes:
  - ArgoCD is always configured to use the GitHub deployment repository.
    Repository URL is hard-coded (open source, no credentials required).
EOF
}

print_env_variables() {
  echo; echo "========================================"
  echo "         Environment Variables"
  echo "========================================"
  printf "%-25s: %s\n" "ORCH_INSTALLER_PROFILE" "${ORCH_INSTALLER_PROFILE:-}"
  printf "%-25s: %s\n" "DEPLOY_REPO_BRANCH/tag/commit-id" "${DEPLOY_REPO_BRANCH:-}"
  echo "========================================"; echo
}

reset_runtime_variables() {
  local config_file="$cwd/onprem.env"

  echo "Cleaning up runtime variables from previous runs..."

  local temp_file="${config_file}.tmp"
  local in_multiline=0

  : >"$temp_file"
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ $in_multiline -eq 1 ]]; then
      [[ "$line" =~ [\'\"][[:space:]]*$ ]] && in_multiline=0
      continue
    fi

    if [[ "$line" =~ ^export\ (SRE_TLS_ENABLED|SRE_DEST_CA_CERT|SMTP_SKIP_VERIFY|DISABLE_CO_PROFILE|DISABLE_AO_PROFILE|DISABLE_O11Y_PROFILE|SINGLE_TENANCY_PROFILE)= ]]; then
      if [[ "$line" =~ =[\'\"]. ]] && ! [[ "$line" =~ =[\'\"].*[\'\"][[:space:]]*$ ]]; then
        in_multiline=1
      fi
      continue
    fi

    echo "$line" >>"$temp_file"
  done <"$config_file"

  mv "$temp_file" "$config_file"

  unset SRE_TLS_ENABLED SRE_DEST_CA_CERT SMTP_SKIP_VERIFY
  unset DISABLE_CO_PROFILE DISABLE_AO_PROFILE DISABLE_O11Y_PROFILE
  unset SINGLE_TENANCY_PROFILE

  echo "Runtime variables cleaned successfully."
}

write_shared_variables() {
  local config_file="$cwd/onprem.env"

  if [[ -n "${SRE_TLS_ENABLED:-}" && "${SRE_TLS_ENABLED}" != "false" ]]; then
    update_config_variable "$config_file" "SRE_TLS_ENABLED" "${SRE_TLS_ENABLED}"
    update_config_variable "$config_file" "SRE_DEST_CA_CERT" "${SRE_DEST_CA_CERT}"
  fi

  if [[ -n "${SMTP_SKIP_VERIFY:-}" && "${SMTP_SKIP_VERIFY}" == "true" ]]; then
    update_config_variable "$config_file" "SMTP_SKIP_VERIFY" "${SMTP_SKIP_VERIFY}"
  fi

  if [[ -n "${DISABLE_CO_PROFILE:-}" && "${DISABLE_CO_PROFILE}" == "true" ]]; then
    update_config_variable "$config_file" "DISABLE_CO_PROFILE" "${DISABLE_CO_PROFILE}"
  fi

  if [[ -n "${DISABLE_AO_PROFILE:-}" && "${DISABLE_AO_PROFILE}" == "true" ]]; then
    update_config_variable "$config_file" "DISABLE_AO_PROFILE" "${DISABLE_AO_PROFILE}"
  fi

  if [[ -n "${DISABLE_O11Y_PROFILE:-}" && "${DISABLE_O11Y_PROFILE}" == "true" ]]; then
    update_config_variable "$config_file" "DISABLE_O11Y_PROFILE" "${DISABLE_O11Y_PROFILE}"
  fi

  if [[ -n "${SINGLE_TENANCY_PROFILE:-}" && "${SINGLE_TENANCY_PROFILE}" == "true" ]]; then
    update_config_variable "$config_file" "SINGLE_TENANCY_PROFILE" "${SINGLE_TENANCY_PROFILE}"
  fi

  echo "Runtime configuration updated in: $config_file"
}

create_namespaces() {
  local orch_namespace_list=(
    "onprem"
    "orch-boots"
    "orch-database"
    "orch-platform"
    "orch-app"
    "orch-cluster"
    "orch-infra"
    "orch-sre"
    "orch-ui"
    "orch-secret"
    "orch-gateway"
    "orch-harbor"
    "cattle-system"
  )
  for ns in "${orch_namespace_list[@]}"; do
    kubectl create ns "$ns" --dry-run=client -o yaml | kubectl apply -f -
  done
}

create_smtp_secrets() {
  if [[ -z "${SMTP_ADDRESS:-}" || -z "${SMTP_PORT:-}" || -z "${SMTP_HEADER:-}" || -z "${SMTP_USERNAME:-}" || -z "${SMTP_PASSWORD:-}" ]]; then
    echo "Warning: SMTP configuration variables not set. Skipping SMTP secrets creation."
    return
  fi

  local namespace=orch-infra
  kubectl -n "$namespace" delete secret smtp --ignore-not-found
  kubectl -n "$namespace" delete secret smtp-auth --ignore-not-found

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: smtp
  namespace: $namespace
type: Opaque
stringData:
  smartHost: $SMTP_ADDRESS
  smartPort: "$SMTP_PORT"
  from: $SMTP_HEADER
  authUsername: $SMTP_USERNAME
EOF

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: smtp-auth
  namespace: $namespace
type: kubernetes.io/basic-auth
stringData:
  password: $SMTP_PASSWORD
EOF
}

create_sre_secrets() {
  if [[ -z "${SRE_USERNAME:-}" || -z "${SRE_PASSWORD:-}" || -z "${SRE_DEST_URL:-}" ]]; then
    echo "Warning: SRE configuration variables not set. Skipping SRE secrets creation."
    return
  fi

  local namespace=orch-sre
  kubectl -n "$namespace" delete secret basic-auth-username --ignore-not-found
  kubectl -n "$namespace" delete secret basic-auth-password --ignore-not-found
  kubectl -n "$namespace" delete secret destination-secret-url --ignore-not-found
  kubectl -n "$namespace" delete secret destination-secret-ca --ignore-not-found

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: basic-auth-username
  namespace: $namespace
stringData:
  username: $SRE_USERNAME
EOF

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: basic-auth-password
  namespace: $namespace
stringData:
  password: "$SRE_PASSWORD"
EOF

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: destination-secret-url
  namespace: $namespace
stringData:
  url: $SRE_DEST_URL
EOF

  if [[ -n "${SRE_DEST_CA_CERT-}" ]]; then
    kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: destination-secret-ca
  namespace: $namespace
stringData:
  ca.crt: |
$(printf "%s" "$SRE_DEST_CA_CERT" | sed -e $'s/^/    /')
EOF
  fi
}

install_gitea_from_repo() {
  (
    local image_registry="${GITEA_IMAGE_REGISTRY:-docker.io}"
    local values_file="$onprem_installers_dir/assets/gitea/values.yaml"

    # StorageClass handling:
    # The values file is typically configured for openebs-hostpath, but that
    # StorageClass may not exist yet on a fresh cluster.
    local storage_class=""
    if kubectl get storageclass openebs-hostpath >/dev/null 2>&1; then
      storage_class="openebs-hostpath"
    else
      storage_class="$(kubectl get storageclass -o jsonpath='{range .items[?(@.metadata.annotations.storageclass\.kubernetes\.io/is-default-class=="true")]}{.metadata.name}{"\n"}{end}' 2>/dev/null | head -n1 || true)"
    fi

    if [[ ! -r "$values_file" ]]; then
      echo "❌ Missing Gitea values: $values_file"
      exit 1
    fi

    echo "Installing Gitea via Helm (chart $GITEA_CHART_VERSION)..."

    helm repo add gitea-charts https://dl.gitea.com/charts/ --force-update >/dev/null

    local tmp
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT

    helm fetch gitea-charts/gitea --version "$GITEA_CHART_VERSION" --untar --untardir "$tmp"

    kubectl create ns gitea >/dev/null 2>&1 || true
    kubectl create ns orch-platform >/dev/null 2>&1 || true

  if ! kubectl -n gitea get secret gitea-tls-certs >/dev/null 2>&1; then
    echo "Generating self-signed TLS cert for in-cluster Gitea..."

    local tmp_cert
    tmp_cert="$(mktemp -d)"

    openssl genrsa -out "$tmp_cert/infra-tls.key" 4096
    openssl req -key "$tmp_cert/infra-tls.key" -new -x509 -days 365 -out "$tmp_cert/infra-tls.crt" \
      -subj "/C=US/O=Orch Deploy/OU=Orchestrator" \
      -addext "subjectAltName=DNS:localhost,DNS:gitea-http.gitea.svc.cluster.local"

    sudo install -D -m 0644 "$tmp_cert/infra-tls.crt" /usr/local/share/ca-certificates/gitea_cert.crt
    sudo update-ca-certificates

    kubectl -n gitea create secret tls gitea-tls-certs \
      --cert="$tmp_cert/infra-tls.crt" \
      --key="$tmp_cert/infra-tls.key"

    rm -rf "$tmp_cert"
  fi

  randomPassword() { openssl rand -hex 8; }

  createGiteaSecret() {
    local secretName=$1
    local accountName=$2
    local password=$3
    local namespace=$4

    kubectl create secret generic "$secretName" -n "$namespace" \
      --from-literal=username="$accountName" \
      --from-literal=password="$password" \
      --dry-run=client -o yaml | kubectl apply -f -
  }

  createGiteaAccount() {
    local accountName=$1
    local password=$2
    local email=$3

    local giteaPod
    giteaPod="$(kubectl get pods -n gitea -l 'app.kubernetes.io/instance=gitea,app.kubernetes.io/name=gitea' -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
    if [[ -z "${giteaPod}" ]]; then
      giteaPod="$(kubectl get pods -n gitea -l app=gitea -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
    fi
    if [[ -z "$giteaPod" ]]; then
      echo "❌ No Gitea pod found"
      exit 1
    fi

    if ! kubectl exec -n gitea "$giteaPod" -c gitea -- gitea admin user list | grep -q "$accountName"; then
      kubectl exec -n gitea "$giteaPod" -c gitea -- gitea admin user create \
        --username "$accountName" --password "$password" --email "$email" --must-change-password=false
    else
      kubectl exec -n gitea "$giteaPod" -c gitea -- gitea admin user change-password \
        --username "$accountName" --password "$password" --must-change-password=false
    fi
  }

  local adminGiteaPassword
  local argocdGiteaPassword
  local appGiteaPassword
  local clusterGiteaPassword

  adminGiteaPassword="$(randomPassword)"
  argocdGiteaPassword="$(randomPassword)"
  appGiteaPassword="$(randomPassword)"
  clusterGiteaPassword="$(randomPassword)"

  createGiteaSecret "gitea-cred" "gitea_admin" "$adminGiteaPassword" "gitea"
  createGiteaSecret "argocd-gitea-credential" "argocd" "$argocdGiteaPassword" "gitea"
  createGiteaSecret "app-gitea-credential" "apporch" "$appGiteaPassword" "orch-platform"
  createGiteaSecret "cluster-gitea-credential" "clusterorch" "$clusterGiteaPassword" "orch-platform"

  local -a helm_extra_args
  helm_extra_args=()
  if [[ -n "${storage_class}" ]]; then
    helm_extra_args+=(--set "persistence.storageClass=${storage_class}")
    helm_extra_args+=(--set "postgresql.primary.persistence.storageClass=${storage_class}")
  else
    echo "⚠️  No default StorageClass detected; PVCs may remain Pending."
  fi

  helm upgrade --install gitea "$tmp/gitea" \
    --values "$values_file" \
    --set gitea.admin.existingSecret=gitea-cred \
    --set image.registry="$image_registry" \
    "${helm_extra_args[@]}" \
    -n gitea --timeout 15m0s --wait

  wait_for_pods_running "$gitea_ns"

  createGiteaAccount "argocd" "$argocdGiteaPassword" "argocd@orch-installer.com"
  createGiteaAccount "apporch" "$appGiteaPassword" "apporch@orch-installer.com"
  createGiteaAccount "clusterorch" "$clusterGiteaPassword" "clusterorch@orch-installer.com"

    echo "Gitea installed"
  )
}

install_argocd_from_repo() {
  (
    local values_tmpl="$onprem_installers_dir/assets/argo-cd/values.tmpl"

    if [[ ! -r "$values_tmpl" ]]; then
      echo "❌ Missing ArgoCD values template: $values_tmpl"
      exit 1
    fi

    echo "Installing ArgoCD via Helm (chart $ARGOCD_CHART_VERSION)..."

    # If a previous run was interrupted while Helm was waiting on Service LB IPs,
    # the release can be left in a pending-* state and block subsequent upgrades.
    local existing_status
    existing_status="$(helm -n "$argo_cd_ns" status argocd 2>/dev/null | awk -F': ' '/^STATUS:/{print $2}' | tr -d '\r' || true)"
    if [[ "$existing_status" == pending-* ]]; then
      echo "⚠️  Existing ArgoCD Helm release is in status '$existing_status' (likely an interrupted install)."
      if [[ "${ASSUME_YES:-false}" == "true" ]]; then
        echo "Uninstalling stuck ArgoCD release and retrying..."
        helm -n "$argo_cd_ns" uninstall argocd >/dev/null 2>&1 || true
      else
        while true; do
          read -rp "ArgoCD release is '$existing_status'. Uninstall and retry install? [yes/no] " yn
          case $yn in
            [Yy]* )
              helm -n "$argo_cd_ns" uninstall argocd >/dev/null 2>&1 || true
              break
            ;;
            [Nn]* )
              echo "❌ Cannot proceed while ArgoCD release is in a pending state."
              echo "   Run: helm -n argocd uninstall argocd (then retry)"
              exit 1
            ;;
            * ) echo "Please answer yes or no.";;
          esac
        done
      fi
    fi

    helm repo add argo-helm https://argoproj.github.io/argo-helm --force-update >/dev/null

    local tmp
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT

    helm fetch argo-helm/argo-cd --version "$ARGOCD_CHART_VERSION" --untar --untardir "$tmp"

  # Render proxy-aware values.yaml (mirrors deb after-install logic)
  cp "$values_tmpl" "$tmp/argo-cd/templates/values.tmpl"
  cat <<EOF >"$tmp/proxy-values.yaml"
http_proxy: ${http_proxy:-}
https_proxy: ${https_proxy:-}
no_proxy: ${no_proxy:-}
EOF

  helm template -s templates/values.tmpl "$tmp/argo-cd" --values "$tmp/proxy-values.yaml" >"$tmp/values.yaml"
  rm -f "$tmp/argo-cd/templates/values.tmpl"

  # HostPath mounts for node CA bundle
  cat <<EOF >"$tmp/mounts.yaml"
notifications:
  extraVolumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  extraVolumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
server:
  volumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  volumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
repoServer:
  volumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  volumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
applicationSet:
  extraVolumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  extraVolumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
EOF

  # NOTE: values.tmpl sets server.service.type=LoadBalancer. On kind (and other
  # clusters without a LB provider ready), helm --wait will block waiting for an
  # external IP. We rely on pod readiness instead.
  helm upgrade --install argocd "$tmp/argo-cd" \
    --values "$tmp/values.yaml" \
    -f "$tmp/mounts.yaml" \
    -n "$argo_cd_ns" --create-namespace --timeout 15m0s

  wait_for_pods_running "$argo_cd_ns"
    echo "ArgoCD installed"
  )
}

create_argocd_repo_secret() {
  local repo_url=$1

  kubectl -n argocd delete secret "$si_config_repo" --ignore-not-found

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: $si_config_repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: $repo_url
EOF
}

install_root_app() {
  local namespace="onprem"
  local cluster_values="$cwd/${ORCH_INSTALLER_PROFILE}.yaml"

  if [[ ! -r "$cluster_values" ]]; then
    echo "❌ Missing cluster config: $cluster_values"
    exit 1
  fi

  local repo_url
  repo_url="https://github.com/open-edge-platform/edge-manageability-framework"

  create_argocd_repo_secret "$repo_url"

  echo "Installing root-app Helm chart..."
  helm upgrade --install root-app "$repo_root/argocd/root-app" \
    -f "$cluster_values" \
    -n "$namespace" --create-namespace
}

install_openebs_localpv() {
  # Hardcoded version (change it here when needed)
  LOCALPV_VERSION="4.3.0"

  echo "👉 Using OpenEBS LocalPV version: $LOCALPV_VERSION"

  echo "👉 Adding OpenEBS LocalPV Helm repo..."
  helm repo add openebs-localpv https://openebs.github.io/dynamic-localpv-provisioner

  echo "🔄 Updating Helm repos..."
  helm repo update

  echo "🚀 Installing/Upgrading OpenEBS LocalPV..."
  helm upgrade --install openebs-localpv openebs-localpv/localpv-provisioner \
    --version "$LOCALPV_VERSION" \
    --namespace openebs-system --create-namespace \
    --set hostpathClass.enabled=true \
    --set hostpathClass.name=openebs-hostpath \
    --set hostpathClass.isDefaultClass=true \
    --set deviceClass.enabled=false

  echo "📦 OpenEBS Pods in openebs-system namespace:"
  kubectl get pods -n openebs-system
}


################################
##### INSTALL SCRIPT START #####
################################

if [[ -f "$MAIN_ENV_CONFIG" ]]; then
  # shellcheck disable=SC1090
  source "$MAIN_ENV_CONFIG"
else
  echo "❌ onprem.env file not found at $MAIN_ENV_CONFIG"
  exit 1
fi

bootstrap_repo_root

reset_runtime_variables

# Re-source the config file after cleanup to get fresh values
# shellcheck disable=SC1090
source "$MAIN_ENV_CONFIG"

if [ -n "${1-}" ]; then
  while :; do
    case "$1" in
      -h|--help)
        usage
        exit 0
      ;;
      -s|--sre)
        SRE_TLS_ENABLED="true"
        if [ "${2-}" ]; then
          SRE_DEST_CA_CERT="$(cat "$2")"
          shift
        fi
      ;;
      -y|--yes)
        ASSUME_YES=true
      ;;
      -d|--notls)
        SMTP_SKIP_VERIFY="true"
      ;;
      --disable-co)
        DISABLE_CO_PROFILE="true"
      ;;
      --disable-ao)
        DISABLE_AO_PROFILE="true"
      ;;
      --disable-o11y)
        DISABLE_O11Y_PROFILE="true"
      ;;
      -t|--trace)
        set -x
        ENABLE_TRACE=true
      ;;
      -st|--single_tenancy)
        SINGLE_TENANCY_PROFILE="true"
      ;;
      -?*)
        echo "Unknown argument $1"
        exit 1
      ;;
      *) break
    esac
    shift
  done
fi

ensure_prereqs

if [ "$ENABLE_TRACE" = true ]; then
  set -x
fi

print_env_variables
write_shared_variables

# Generate cluster config YAML in the current working directory
generate_cluster_yaml_onprem_from_upstream

# Decide whether to install Gitea.
# - Default: install Gitea
# - Skip when Application Orchestrator is disabled (--disable-ao)
# - Allow explicit override via env var GITEA_ENABLED=false
if [[ "${DISABLE_AO_PROFILE:-false}" == "true" ]]; then
  INSTALL_GITEA="false"
elif [[ "${GITEA_ENABLED:-true}" != "true" ]]; then
  INSTALL_GITEA="false"
else
  INSTALL_GITEA="true"
fi

if [ "$ASSUME_YES" = false ]; then
  while true; do
    read -rp "Edit config values.yaml files with custom configurations if necessary!!!
  The generated cluster config file is located at:
  $cwd/$ORCH_INSTALLER_PROFILE.yaml
Enter 'yes' to confirm that configuration is done in order to progress with installation
('no' will exit the script) !!!

Ready to proceed with installation? " yn
    case $yn in
      [Yy]* ) break;;
      [Nn]* ) exit 1;;
      * ) echo "Please answer yes or no.";;
    esac
  done
fi

# Keep kubectl calls pointed at a kubeconfig readable by the invoking user.
if [[ -n "${KUBECONFIG:-}" ]]; then
  :
else
  export KUBECONFIG="/home/${SUDO_USER:-$USER}/.kube/config"
fi

if [[ "$INSTALL_GITEA" == "true" ]]; then
  install_gitea_from_repo
else
  echo "Gitea installation skipped (Application Orchestrator is disabled)"
fi

install_argocd_from_repo

create_namespaces
create_sre_secrets
create_smtp_secrets

harbor_password=$(openssl rand -hex 50)
keycloak_password=$(generate_password)
postgres_password=$(generate_password)
create_harbor_secret orch-harbor "$harbor_password"
create_harbor_password orch-harbor "$harbor_password"
create_keycloak_password orch-platform "$keycloak_password"
create_postgres_password orch-database "$postgres_password"

install_root_app
install_openebs_localpv

printf "\nEdge Orchestrator SW is being deployed, please wait for all applications to deploy...\n\
To check the status of the deployment run 'kubectl get applications -A'.\n\
Installation is completed when 'root-app' Application is in 'Healthy' and 'Synced' state.\n\
Once it is completed, you might want to configure DNS for UI and other services by running generate_fqdn script and following instructions\n"


