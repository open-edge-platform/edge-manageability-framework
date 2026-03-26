#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: post-orch-upgrade.sh
# Description: Deb-free post-upgrade script for Edge Orchestrator.
#              Replaces the onprem-gitea-installer, onprem-argocd-installer,
#              and onprem-orch-installer debian packages with pure bash.
#
#              This script handles everything AFTER the Kubernetes cluster
#              has been upgraded (by pre-orch-upgrade.sh):
#               - Retrieving and updating cluster configuration
#               - Upgrading Gitea (TLS certs, Helm chart, accounts)
#               - Upgrading ArgoCD (proxy config, Helm chart)
#               - Deploying the orchestrator (root-app via Helm)
#               - PostgreSQL migration to CloudNativePG
#               - Service recovery (MPS/RPS, Vault, restarts)
#               - Cleanup (external-secrets CRDs, Kyverno, nginx)
#
# Prerequisites:
#   - pre-orch-upgrade.sh has completed (K8s cluster upgraded, OS configured)
#   - onprem.env is configured with correct values
#   - kubectl, helm, yq, openssl are available
#   - sudo access (for cert installation)
#   - Repo tarball in repo_archives/ (or running from a git checkout)
#
# Usage:
#   ./post-orch-upgrade.sh [options]
#
# Options:
#   -l    Use local packages (skip artifact download)
#   -s    Skip interactive prompts (non-interactive mode)
#   -h    Show help

set -euo pipefail

export PATH="/usr/local/bin:${PATH}"
export KUBECONFIG="${KUBECONFIG:-/home/$USER/.kube/config}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck disable=SC1091
source "${SCRIPT_DIR}/onprem.env"

# shellcheck disable=SC1091
source "${SCRIPT_DIR}/upgrade_postgres.sh"

# shellcheck disable=SC1091
source "${SCRIPT_DIR}/vault_unseal.sh"

################################
# Logging
################################

LOG_FILE="post_orch_upgrade_$(date +'%Y%m%d_%H%M%S').log"
LOG_DIR="/var/log/orch-upgrade"

sudo mkdir -p "$LOG_DIR"
sudo chown "$(whoami):$(whoami)" "$LOG_DIR"

FULL_LOG_PATH="$LOG_DIR/$LOG_FILE"

log_message() {
  echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$FULL_LOG_PATH"
}

log_info() {
  log_message "INFO: $*"
}

log_warn() {
  log_message "WARN: $*"
}

log_error() {
  log_message "ERROR: $*"
}

# Redirect all output to both console and log file
exec > >(tee -a "$FULL_LOG_PATH")
exec 2> >(tee -a "$FULL_LOG_PATH" >&2)

log_info "Starting post-orch-upgrade script"
log_info "Log file: $FULL_LOG_PATH"

################################
# Defaults / Configuration
################################

apps_ns="${APPS_NS:-onprem}"
argo_cd_ns="${ARGO_CD_NS:-argocd}"
gitea_ns="${GITEA_NS:-gitea}"
si_config_repo="edge-manageability-framework"

cwd="$(pwd)"
git_arch_name="repo_archives"

GIT_REPOS="${GIT_REPOS:-$cwd/$git_arch_name}"
export GIT_REPOS

ORCH_INSTALLER_PROFILE="${ORCH_INSTALLER_PROFILE:-onprem}"
INSTALL_GITEA="${INSTALL_GITEA:-true}"
GITEA_IMAGE_REGISTRY="${GITEA_IMAGE_REGISTRY:-docker.io}"
USE_LOCAL_PACKAGES="${USE_LOCAL_PACKAGES:-false}"
DEPLOY_VERSION="${DEPLOY_VERSION:-v3.1.0}"
SKIP_INTERACTIVE="${SKIP_INTERACTIVE:-false}"

GITEA_CHART_VERSION="${GITEA_CHART_VERSION:-10.4.0}"
ARGOCD_CHART_VERSION="${ARGOCD_CHART_VERSION:-8.2.7}"

# Paths set during setup_working_dir
WORK_DIR=""
REPO_DIR=""
ONPREM_INSTALLERS_DIR=""

################################
# Cleanup trap
################################

cleanup_work_dir() {
  if [[ -n "${WORK_DIR:-}" && -d "${WORK_DIR:-}" ]]; then
    log_info "Cleaning up working directory: $WORK_DIR"
    rm -rf "$WORK_DIR"
  fi
}

trap cleanup_work_dir EXIT

################################
# Prerequisites
################################

check_prerequisites() {
  log_info "Checking prerequisites..."

  local missing=()
  for cmd in kubectl helm yq openssl; do
    if ! command -v "$cmd" &>/dev/null; then
      missing+=("$cmd")
    fi
  done

  if [[ ${#missing[@]} -gt 0 ]]; then
    log_error "Missing required tools: ${missing[*]}"
    log_error "Run pre-orch-upgrade.sh first (installs helm and yq), or install manually."
    exit 1
  fi

  if ! kubectl cluster-info &>/dev/null; then
    log_error "Cannot reach Kubernetes cluster. Check KUBECONFIG."
    exit 1
  fi

  log_info "All prerequisites met."
}

################################
# Helper Functions
################################

update_config_variable() {
  local config_file="$1" var_name="$2" var_value="$3"
  if [[ -n "${var_value:-}" ]]; then
    if grep -q "^export ${var_name}=" "$config_file"; then
      sed -i "s|^export ${var_name}=.*|export ${var_name}='${var_value}'|" "$config_file"
    else
      echo "export ${var_name}='${var_value}'" >> "$config_file"
    fi
  fi
}

wait_for_pods_running() {
  local ns="$1"
  log_info "Waiting for all pods to be Ready in namespace $ns..."
  kubectl wait pod --selector='!job-name' --all --for=condition=Ready \
    --namespace="$ns" --timeout=600s
}

resync_all_apps() {
  if [[ ! -f /tmp/argo-cd/sync-patch.yaml ]]; then
    sudo mkdir -p /tmp/argo-cd
    cat <<SYNCEOF | sudo tee /tmp/argo-cd/sync-patch.yaml >/dev/null
operation:
  sync:
    syncStrategy:
      hook: {}

SYNCEOF
  fi
  kubectl patch application root-app -n "$apps_ns" --type merge \
    -p '{"operation":null}' || true
  kubectl patch application root-app -n "$apps_ns" --type json \
    -p '[{"op": "remove", "path": "/status/operationState"}]' || true
  sleep 10
  kubectl patch application root-app -n "$apps_ns" \
    --patch-file /tmp/argo-cd/sync-patch.yaml --type merge
}

terminate_existing_sync() {
  local app_name="$1" namespace="$2"
  local current_phase
  current_phase=$(kubectl get application "$app_name" -n "$namespace" \
    -o jsonpath='{.status.operationState.phase}' 2>/dev/null || true)

  if [[ "$current_phase" == "Running" ]]; then
    log_info "Terminating existing sync operation for $app_name..."
    kubectl patch application "$app_name" -n "$namespace" \
      --type='merge' -p='{"operation": null}'
    timeout 30 bash -c "
      while [[ \"\$(kubectl get application '$app_name' -n '$namespace' \
        -o jsonpath='{.status.operationState.phase}' 2>/dev/null)\" == 'Running' ]]; do
        sleep 2
      done
    " || true
  fi
}

check_and_patch_sync_app() {
  local app_name="$1" namespace="$2"
  local max_retries=2

  for ((i=1; i<=max_retries; i++)); do
    local app_status
    app_status=$(kubectl get application "$app_name" -n "$namespace" \
      -o jsonpath='{.status.sync.status} {.status.health.status}' \
      2>/dev/null || echo "NotFound NotFound")

    if [[ "$app_status" == "Synced Healthy" ]]; then
      log_info "$app_name is Synced and Healthy"
      return 0
    fi

    log_warn "$app_name not healthy (status: $app_status). Syncing (attempt $i/$max_retries)"

    set +e
    terminate_existing_sync "$app_name" "$namespace"
    kubectl patch -n "$namespace" application "$app_name" \
      --patch-file /tmp/argo-cd/sync-patch.yaml --type merge
    set -e

    local check_timeout=90 check_interval=3 elapsed=0
    while (( elapsed < check_timeout )); do
      app_status=$(kubectl get application "$app_name" -n "$namespace" \
        -o jsonpath='{.status.sync.status} {.status.health.status}' \
        2>/dev/null || echo "NotFound NotFound")

      if [[ "$app_status" == "Synced Healthy" ]]; then
        log_info "$app_name became Synced and Healthy"
        return 0
      fi
      sleep "$check_interval"
      elapsed=$((elapsed + check_interval))
    done
  done
  log_warn "$app_name may still require attention after $max_retries attempts"
}

wait_for_app_synced_healthy() {
  resync_all_apps
  local app_name="$1" namespace="$2" timeout_s="${3:-120}"
  local start_time
  start_time=$(date +%s)

  set +e
  while true; do
    local app_status
    app_status=$(kubectl get application "$app_name" -n "$namespace" \
      -o jsonpath='{.status.sync.status} {.status.health.status}' \
      2>/dev/null || echo "NotFound NotFound")

    if [[ "$app_status" == "Synced Healthy" ]]; then
      log_info "$app_name is Synced and Healthy."
      set -e
      return 0
    fi

    local current_time elapsed
    current_time=$(date +%s)
    elapsed=$((current_time - start_time))
    if (( elapsed > timeout_s )); then
      log_warn "Timeout waiting for $app_name after ${timeout_s}s (status: $app_status)"
      set -e
      return 0
    fi

    log_info "Waiting for $app_name (${elapsed}s/${timeout_s}s, status: $app_status)"
    sleep 3
  done
}

restart_statefulset() {
  local name="$1" namespace="$2"
  log_info "Restarting StatefulSet $name in $namespace..."
  local replicas
  replicas=$(kubectl get statefulset "$name" -n "$namespace" \
    -o jsonpath='{.spec.replicas}')
  kubectl scale statefulset "$name" -n "$namespace" --replicas=0
  kubectl wait --for=delete pod -l "app=$name" -n "$namespace" \
    --timeout=300s || true
  kubectl scale statefulset "$name" -n "$namespace" --replicas="$replicas"
  log_info "$name restarted"
}

cleanup_gitea_secrets() {
  log_info "Cleaning up old Gitea secrets..."
  local secrets=("gitea-apporch-token" "gitea-argocd-token" "gitea-clusterorch-token")
  for secret in "${secrets[@]}"; do
    if kubectl get secret "$secret" -n gitea >/dev/null 2>&1; then
      kubectl delete secret "$secret" -n gitea
      log_info "Deleted secret: $secret"
    fi
  done
}

delete_nginx_if_any() {
  log_info "Checking and deleting nginx ingress (if any)..."
  kubectl delete application ingress-nginx -n "$apps_ns" \
    --ignore-not-found=true || true
  kubectl delete application nginx-ingress-pxe-boots -n "$apps_ns" \
    --ignore-not-found=true || true

  local harbor_pods
  harbor_pods=$(kubectl get pods -n orch-harbor --no-headers 2>/dev/null \
    | awk '/harbor-oci-nginx/ {print $1}' || true)
  if [[ -n "${harbor_pods:-}" ]]; then
    log_info "Deleting harbor nginx pods"
    # shellcheck disable=SC2086
    kubectl delete pod -n orch-harbor $harbor_pods || true
  fi
  log_info "Nginx cleanup done"
}

################################################################################
#                     PHASE 1: CONFIGURATION
################################################################################

retrieve_and_update_config() {
  log_info "=== Phase 1a: Retrieving cluster configuration ==="
  local config_file="$cwd/onprem.env"

  # Get LoadBalancer IPs — fall back to existing onprem.env values when
  # services are absent (e.g. freshly recreated KIND cluster).
  local argo_ip traefik_ip haproxy_ip
  argo_ip=$(kubectl get svc argocd-server -n argocd \
    -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
  traefik_ip=$(kubectl get svc traefik -n orch-gateway \
    -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)

  if kubectl get svc ingress-haproxy-kubernetes-ingress -n orch-boots >/dev/null 2>&1; then
    haproxy_ip=$(kubectl get svc ingress-haproxy-kubernetes-ingress -n orch-boots \
      -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
  elif kubectl get svc ingress-nginx-controller -n orch-boots >/dev/null 2>&1; then
    haproxy_ip=$(kubectl get svc ingress-nginx-controller -n orch-boots \
      -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
  else
    log_warn "No ingress service found in orch-boots namespace — using existing onprem.env values"
    haproxy_ip="${HAPROXY_IP:-}"
  fi

  # Only update config when a non-empty value was retrieved; otherwise keep existing
  [[ -n "$argo_ip" ]]    && update_config_variable "$config_file" "ARGO_IP" "$argo_ip"
  [[ -n "$traefik_ip" ]] && update_config_variable "$config_file" "TRAEFIK_IP" "$traefik_ip"
  [[ -n "$haproxy_ip" ]] && update_config_variable "$config_file" "HAPROXY_IP" "$haproxy_ip"

  # SRE TLS Configuration
  local sre_tls_enabled
  sre_tls_enabled=$(kubectl get applications -n "$apps_ns" sre-exporter \
    -o jsonpath='{.spec.sources[*].helm.valuesObject.otelCollector.tls.enabled}' \
    2>/dev/null || echo "false")

  if [[ "$sre_tls_enabled" == "true" ]]; then
    update_config_variable "$config_file" "SRE_TLS_ENABLED" "true"
    local sre_dest_ca_cert
    sre_dest_ca_cert=$(kubectl get applications -n "$apps_ns" sre-exporter \
      -o jsonpath='{.spec.sources[*].helm.valuesObject.otelCollector.tls.caSecret.enabled}' \
      2>/dev/null || echo "false")
    [[ "$sre_dest_ca_cert" == "true" ]] && \
      update_config_variable "$config_file" "SRE_DEST_CA_CERT" "true"
  else
    update_config_variable "$config_file" "SRE_TLS_ENABLED" "false"
  fi

  # Detect profiles from ArgoCD root-app
  local value_files
  value_files=$(kubectl get application root-app -n "$apps_ns" \
    -o jsonpath='{.spec.sources[0].helm.valueFiles[*]}' 2>/dev/null || true)

  if [[ -z "$value_files" ]]; then
    log_warn "No value files found in root-app"
  else
    local disable_co="false" disable_ao="false" disable_o11y="false" single_tenancy="false"
    echo "$value_files" | grep -q "enable-cluster-orch.yaml" || disable_co="true"
    echo "$value_files" | grep -q "enable-app-orch.yaml" || disable_ao="true"
    echo "$value_files" | grep -qE "(enable-o11y\.yaml|o11y-onprem-1k\.yaml)" || disable_o11y="true"
    echo "$value_files" | grep -q "enable-singleTenancy.yaml" && single_tenancy="true"

    INSTALL_GITEA="true"
    if [[ "$disable_co" == "true" || "$disable_ao" == "true" ]]; then
      INSTALL_GITEA="false"
    fi

    update_config_variable "$config_file" "DISABLE_CO_PROFILE" "$disable_co"
    update_config_variable "$config_file" "DISABLE_AO_PROFILE" "$disable_ao"
    update_config_variable "$config_file" "DISABLE_O11Y_PROFILE" "$disable_o11y"
    update_config_variable "$config_file" "SINGLE_TENANCY_PROFILE" "$single_tenancy"
    update_config_variable "$config_file" "INSTALL_GITEA" "$INSTALL_GITEA"
  fi

  # SMTP configuration
  local smtp_skip_verify
  smtp_skip_verify=$(kubectl get application alerting-monitor -n "$apps_ns" \
    -o jsonpath='{.spec.sources[*].helm.valuesObject.alertingMonitor.smtp.insecureSkipVerify}' \
    2>/dev/null || echo "false")
  update_config_variable "$config_file" "SMTP_SKIP_VERIFY" "$smtp_skip_verify"

  log_info "Configuration retrieval completed."

  # Re-source the updated config
  # shellcheck disable=SC1090
  source "$config_file"
}

setup_working_dir() {
  log_info "=== Phase 1b: Setting up working directory ==="

  # Try to discover repo root from script location (git checkout mode)
  # Check common locations: two dirs up from script, HOME, or cwd
  local candidates=(
    "$(cd "$SCRIPT_DIR/../.." 2>/dev/null && pwd)"
    "$HOME/edge-manageability-framework"
    "$(cd "$cwd/../edge-manageability-framework" 2>/dev/null && pwd)"
  )

  for candidate in "${candidates[@]}"; do
    if [[ -d "$candidate/orch-configs" && -d "$candidate/argocd" && -d "$candidate/on-prem-installers" ]]; then
      log_info "Running from git checkout: $candidate"
      REPO_DIR="$candidate"
      ONPREM_INSTALLERS_DIR="$REPO_DIR/on-prem-installers"
      return 0
    fi
  done

  # Tarball mode: extract the repo tarball
  if [[ ! -d "$GIT_REPOS" ]]; then
    log_error "Repo archives directory not found: $GIT_REPOS"
    exit 1
  fi

  local repo_file
  repo_file=$(find "$GIT_REPOS" -name "*${si_config_repo}*.tgz" -type f | head -1)
  if [[ -z "$repo_file" ]]; then
    log_error "No $si_config_repo tarball found in $GIT_REPOS"
    exit 1
  fi

  WORK_DIR="$(mktemp -d)"
  log_info "Extracting repo tarball to $WORK_DIR"
  tar -xf "$repo_file" -C "$WORK_DIR"

  REPO_DIR="$WORK_DIR/$si_config_repo"
  ONPREM_INSTALLERS_DIR="$REPO_DIR/on-prem-installers"

  if [[ ! -d "$REPO_DIR/orch-configs" || ! -d "$REPO_DIR/argocd" ]]; then
    log_error "Extracted tarball does not look like a valid $si_config_repo repo"
    exit 1
  fi

  log_info "Repo extracted to: $REPO_DIR"
}

apply_cluster_config() {
  log_info "=== Phase 1c: Generating and applying cluster config ==="

  local gen_script="$REPO_DIR/installer/generate_cluster_yaml.sh"
  if [[ ! -x "$gen_script" ]]; then
    log_warn "generate_cluster_yaml.sh not found at $gen_script, trying current directory..."
    gen_script="./generate_cluster_yaml.sh"
  fi

  if [[ -x "$gen_script" ]]; then
    rm -f "${ORCH_INSTALLER_PROFILE}.yaml"

    # generate_cluster_yaml.sh sources onprem.env from its own directory and
    # reads cluster_onprem.tpl from $PWD.  Symlink both into place.
    local gen_dir
    gen_dir="$(dirname "$gen_script")"
    if [[ ! -f "$gen_dir/onprem.env" ]]; then
      ln -sf "$cwd/onprem.env" "$gen_dir/onprem.env"
    fi
    local tpl_file="$cwd/cluster_onprem.tpl"
    if [[ ! -f "$gen_dir/cluster_onprem.tpl" && -f "$tpl_file" ]]; then
      ln -sf "$tpl_file" "$gen_dir/cluster_onprem.tpl"
    fi

    (cd "$gen_dir" && bash "$gen_script" onprem)

    # Move generated output to cwd if it landed in gen_dir
    if [[ -f "$gen_dir/${ORCH_INSTALLER_PROFILE}.yaml" && "$gen_dir" != "$cwd" ]]; then
      mv "$gen_dir/${ORCH_INSTALLER_PROFILE}.yaml" "$cwd/"
    fi
  else
    log_warn "generate_cluster_yaml.sh not found. Expecting ${ORCH_INSTALLER_PROFILE}.yaml to exist."
  fi

  local cluster_yaml="$cwd/${ORCH_INSTALLER_PROFILE}.yaml"
  if [[ ! -f "$cluster_yaml" ]]; then
    log_error "Cluster config not found: $cluster_yaml"
    exit 1
  fi

  # Copy cluster config into repo for root-app and Gitea push
  local target_dir="$REPO_DIR/orch-configs/clusters"
  if [[ -d "$target_dir" ]]; then
    cp "$cluster_yaml" "$target_dir/${ORCH_INSTALLER_PROFILE}.yaml"
    log_info "Cluster config copied to $target_dir/"
  fi

  if [[ "$SKIP_INTERACTIVE" != "true" ]]; then
    while true; do
      read -rp "Edit values.yaml if required. Ready to proceed? (yes/no): " yn
      case $yn in
        [Yy]* ) break;;
        [Nn]* ) exit 1;;
        * ) echo "Please answer yes or no.";;
      esac
    done
  fi

  log_info "Cluster config ready: $cluster_yaml"
}

################################################################################
#                     PHASE 2: GITEA UPGRADE
################################################################################

upgrade_gitea() {
  if [[ "$INSTALL_GITEA" != "true" ]]; then
    log_info "Skipping Gitea upgrade (INSTALL_GITEA=$INSTALL_GITEA)"
    return 0
  fi

  log_info "=== Phase 2: Upgrading Gitea ==="

  local image_registry="${GITEA_IMAGE_REGISTRY:-docker.io}"
  local values_file="$ONPREM_INSTALLERS_DIR/assets/gitea/values.yaml"
  if [[ ! -r "$values_file" ]]; then
    log_error "Gitea values file not found: $values_file"
    exit 1
  fi

  # Fetch Gitea chart from helm repo
  local chart_dir
  chart_dir="$(mktemp -d)"
  trap 'rm -rf "${chart_dir:-}"' RETURN

  log_info "Fetching Gitea chart v${GITEA_CHART_VERSION}..."
  helm repo add gitea-charts https://dl.gitea.com/charts/ --force-update >/dev/null 2>&1
  helm fetch gitea-charts/gitea --version "$GITEA_CHART_VERSION" \
    --untar --untardir "$chart_dir"

  # Ensure namespaces exist
  kubectl create ns gitea >/dev/null 2>&1 || true
  kubectl create ns orch-platform >/dev/null 2>&1 || true

  # Generate TLS cert if not present
  if ! kubectl -n gitea get secret gitea-tls-certs >/dev/null 2>&1; then
    log_info "Generating self-signed TLS cert for Gitea..."
    local tmp_cert
    tmp_cert="$(mktemp -d)"

    openssl genrsa -out "$tmp_cert/infra-tls.key" 4096 2>/dev/null
    openssl req -key "$tmp_cert/infra-tls.key" -new -x509 -days 365 \
      -out "$tmp_cert/infra-tls.crt" \
      -subj "/C=US/O=Orch Deploy/OU=Open Edge Platform" \
      -addext "subjectAltName=DNS:localhost,DNS:gitea-http.gitea.svc.cluster.local" \
      2>/dev/null

    sudo install -D -m 0644 "$tmp_cert/infra-tls.crt" \
      /usr/local/share/ca-certificates/gitea_cert.crt
    sudo update-ca-certificates -f

    kubectl create secret tls gitea-tls-certs -n gitea \
      --cert="$tmp_cert/infra-tls.crt" \
      --key="$tmp_cert/infra-tls.key"

    rm -rf "$tmp_cert"
  fi

  # Generate random passwords (use openssl to avoid SIGPIPE under pipefail)
  local admin_pw argocd_pw app_pw cluster_pw
  admin_pw="$(openssl rand -base64 24 | tr -dc A-Za-z0-9 | cut -c1-16)"
  argocd_pw="$(openssl rand -base64 24 | tr -dc A-Za-z0-9 | cut -c1-16)"
  app_pw="$(openssl rand -base64 24 | tr -dc A-Za-z0-9 | cut -c1-16)"
  cluster_pw="$(openssl rand -base64 24 | tr -dc A-Za-z0-9 | cut -c1-16)"

  # Create secrets
  _create_gitea_secret "gitea-cred" "gitea_admin" "$admin_pw" "gitea"
  _create_gitea_secret "argocd-gitea-credential" "argocd" "$argocd_pw" "gitea"
  _create_gitea_secret "app-gitea-credential" "apporch" "$app_pw" "orch-platform"
  _create_gitea_secret "cluster-gitea-credential" "clusterorch" "$cluster_pw" "orch-platform"

  # Scale down Gitea before upgrade
  kubectl scale deployment gitea -n gitea --replicas=0 2>/dev/null || true

  # Helm upgrade Gitea
  log_info "Running helm upgrade for Gitea..."
  helm upgrade --install gitea "$chart_dir/gitea" \
    --values "$values_file" \
    --set gitea.admin.existingSecret=gitea-cred \
    --set "image.registry=${image_registry}" \
    -n gitea --timeout 15m0s --wait

  wait_for_pods_running "$gitea_ns"

  # Create/update Gitea accounts
  _create_gitea_account "argocd-gitea-credential" "argocd" "$argocd_pw" \
    "argocd@orch-installer.com"
  _create_gitea_account "app-gitea-credential" "apporch" "$app_pw" \
    "test@test.com"
  _create_gitea_account "cluster-gitea-credential" "clusterorch" "$cluster_pw" \
    "test@test2.com"

  log_info "Gitea upgrade completed."
}

_create_gitea_secret() {
  local secret_name="$1" account_name="$2" password="$3" namespace="$4"
  kubectl create secret generic "$secret_name" -n "$namespace" \
    --from-literal=username="$account_name" \
    --from-literal=password="$password" \
    --dry-run=client -o yaml | kubectl apply -f -
}

_create_gitea_account() {
  local secret_name="$1" account_name="$2" password="$3" email="$4"

  local gitea_pod
  gitea_pod=$(kubectl get pods -n gitea -l app=gitea \
    -o jsonpath="{.items[0].metadata.name}" 2>/dev/null || true)

  if [[ -z "$gitea_pod" ]]; then
    # Try newer label selector
    gitea_pod=$(kubectl get pods -n gitea \
      -l 'app.kubernetes.io/instance=gitea,app.kubernetes.io/name=gitea' \
      -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  fi

  if [[ -z "$gitea_pod" ]]; then
    log_error "No Gitea pods found"
    return 1
  fi

  if ! kubectl exec -n gitea "$gitea_pod" -c gitea -- \
    gitea admin user list 2>/dev/null | grep -q "$account_name"; then
    log_info "Creating Gitea account: $account_name"
    kubectl exec -n gitea "$gitea_pod" -c gitea -- \
      gitea admin user create --username "$account_name" --password "$password" \
      --email "$email" --must-change-password=false
  else
    log_info "Updating Gitea account password: $account_name"
    kubectl exec -n gitea "$gitea_pod" -c gitea -- \
      gitea admin user change-password --username "$account_name" \
      --password "$password" --must-change-password=false
  fi

  # Generate access token
  local user_token token
  user_token=$(kubectl exec -n gitea "$gitea_pod" -c gitea -- \
    gitea admin user generate-access-token \
    --scopes write:repository,write:user \
    --username "$account_name" \
    --token-name "${account_name}-$(date +%s)" 2>/dev/null || true)
  token=$(echo "$user_token" | awk '{print $NF}')

  if [[ -n "$token" ]]; then
    kubectl create secret generic "gitea-${account_name}-token" -n gitea \
      --from-literal=token="$token" \
      --dry-run=client -o yaml | kubectl apply -f -
  fi
}

################################################################################
#                     PHASE 3: ARGOCD UPGRADE
################################################################################

upgrade_argocd() {
  log_info "=== Phase 3: Upgrading ArgoCD ==="

  local values_tmpl="$ONPREM_INSTALLERS_DIR/assets/argo-cd/values.tmpl"
  if [[ ! -r "$values_tmpl" ]]; then
    log_error "ArgoCD values template not found: $values_tmpl"
    exit 1
  fi

  # Fetch ArgoCD chart from helm repo
  local chart_dir
  chart_dir="$(mktemp -d)"
  trap 'rm -rf "${chart_dir:-}"' RETURN

  log_info "Fetching ArgoCD chart v${ARGOCD_CHART_VERSION}..."
  helm repo add argo-helm https://argoproj.github.io/argo-helm \
    --force-update >/dev/null 2>&1
  helm fetch argo-helm/argo-cd --version "$ARGOCD_CHART_VERSION" \
    --untar --untardir "$chart_dir"

  # Process proxy configuration via helm template
  cp "$values_tmpl" "$chart_dir/argo-cd/templates/values.tmpl"

  cat <<EOF >"$chart_dir/proxy-values.yaml"
http_proxy: ${http_proxy:-}
https_proxy: ${https_proxy:-}
no_proxy: ${no_proxy:-}
EOF

  helm template -s templates/values.tmpl "$chart_dir/argo-cd" \
    --values "$chart_dir/proxy-values.yaml" > "$chart_dir/values.yaml"
  rm -f "$chart_dir/argo-cd/templates/values.tmpl"

  # Generate volume mounts for node CA bundle and Gitea TLS
  cat <<EOF >"$chart_dir/mounts.yaml"
notifications:
  extraVolumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  - mountPath: /etc/ssl/certs/gitea_cert.crt
    name: gitea-tls
  extraVolumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
  - name: gitea-tls
    hostPath:
      path: /usr/local/share/ca-certificates/gitea_cert.crt
server:
  volumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  - mountPath: /etc/ssl/certs/gitea_cert.crt
    name: gitea-tls
  volumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
  - name: gitea-tls
    hostPath:
      path: /usr/local/share/ca-certificates/gitea_cert.crt
repoServer:
  volumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  - mountPath: /etc/ssl/certs/gitea_cert.crt
    name: gitea-tls
  volumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
  - name: gitea-tls
    hostPath:
      path: /usr/local/share/ca-certificates/gitea_cert.crt
applicationSet:
  extraVolumeMounts:
  - mountPath: /etc/ssl/certs/ca-certificates.crt
    name: tls-from-node
  - mountPath: /etc/ssl/certs/gitea_cert.crt
    name: gitea-tls
  extraVolumes:
  - name: tls-from-node
    hostPath:
      path: /etc/ssl/certs/ca-certificates.crt
  - name: gitea-tls
    hostPath:
      path: /usr/local/share/ca-certificates/gitea_cert.crt
EOF

  log_info "Running helm upgrade for ArgoCD..."
  kubectl create ns "$argo_cd_ns" >/dev/null 2>&1 || true
  helm upgrade --install argocd "$chart_dir/argo-cd" \
    --values "$chart_dir/values.yaml" \
    -f "$chart_dir/mounts.yaml" \
    -n "$argo_cd_ns" --create-namespace --wait --timeout 15m0s

  wait_for_pods_running "$argo_cd_ns"

  log_info "ArgoCD upgrade completed."
}

################################################################################
#                     PHASE 4: ORCHESTRATOR DEPLOYMENT
################################################################################

get_gitea_service_url() {
  if [[ "$INSTALL_GITEA" != "true" ]]; then
    echo ""
    return
  fi

  local port
  port=$(kubectl get svc gitea-http -n gitea \
    -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || true)

  if [[ "$port" == "443" ]]; then
    echo "gitea-http.gitea.svc.cluster.local"
  elif [[ -n "$port" ]]; then
    echo "gitea-http.gitea.svc.cluster.local:${port}"
  else
    log_warn "Could not determine Gitea service URL"
    echo "gitea-http.gitea.svc.cluster.local"
  fi
}

push_repo_to_gitea() {
  local gitea_url="$1"
  log_info "Pushing repository to Gitea..."

  # Clean up any previous push job
  kubectl delete job gitea-init-${si_config_repo} -n gitea \
    --ignore-not-found=true 2>/dev/null || true

  # Create K8s Job to push repo content via git
  kubectl apply -f - <<JOBEOF
apiVersion: batch/v1
kind: Job
metadata:
  name: gitea-init-${si_config_repo}
  namespace: gitea
  labels:
    managed-by: edge-manageability-framework
spec:
  template:
    spec:
      volumes:
      - name: repo
        hostPath:
          path: ${REPO_DIR}
      - name: tls
        secret:
          secretName: gitea-tls-certs
      containers:
      - name: alpine
        image: alpine/git:2.49.1
        env:
        - name: GITEA_USERNAME
          valueFrom:
            secretKeyRef:
              name: argocd-gitea-credential
              key: username
        - name: GITEA_PASSWORD
          valueFrom:
            secretKeyRef:
              name: argocd-gitea-credential
              key: password
        command:
        - /bin/sh
        - -c
        args:
        - |
          git config --global credential.helper store
          git config --global user.email \$GITEA_USERNAME@orch-installer.com
          git config --global user.name \$GITEA_USERNAME
          git config --global http.sslCAInfo /usr/local/share/ca-certificates/tls.crt
          git config --global --add safe.directory /repo
          echo "https://\$GITEA_USERNAME:\$GITEA_PASSWORD@${gitea_url}" > /root/.git-credentials
          cd /repo
          git init
          git remote add gitea "https://${gitea_url}/\$GITEA_USERNAME/${si_config_repo}.git" 2>/dev/null || \
            git remote set-url gitea "https://${gitea_url}/\$GITEA_USERNAME/${si_config_repo}.git"
          git checkout -B main
          git add .
          git commit --allow-empty -m 'Recreate repo from artifact'
          git push --force gitea main
        volumeMounts:
        - name: repo
          mountPath: /repo
        - name: tls
          mountPath: /usr/local/share/ca-certificates/
      restartPolicy: Never
  backoffLimit: 5
JOBEOF

  log_info "Waiting for Gitea push job to complete..."
  kubectl wait --for=condition=complete --timeout=300s \
    -n gitea "job/gitea-init-${si_config_repo}"

  log_info "Repo pushed to Gitea successfully."
}

create_gitea_creds_secret() {
  local gitea_url="$1"
  log_info "Creating ArgoCD repository secret for Gitea..."

  # Fetch Gitea credentials
  local username_b64 password_b64 username password
  username_b64=$(kubectl get secret argocd-gitea-credential -n gitea \
    -o jsonpath='{.data.username}')
  password_b64=$(kubectl get secret argocd-gitea-credential -n gitea \
    -o jsonpath='{.data.password}')
  username=$(echo "$username_b64" | base64 -d)
  password=$(echo "$password_b64" | base64 -d)

  kubectl delete secret "$si_config_repo" -n argocd --ignore-not-found

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${si_config_repo}
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: https://${gitea_url}/${username}/${si_config_repo}
  password: ${password}
  username: ${username}
EOF

  log_info "ArgoCD Gitea repository secret created."
}

create_github_creds_secret() {
  log_info "Creating ArgoCD repository secret for GitHub..."

  local git_token="${GIT_TOKEN:-}"
  local git_user="${GIT_USER:-}"
  local deploy_repo_url="${DEPLOY_REPO_URL:-https://github.com/open-edge-platform/edge-manageability-framework}"

  kubectl delete secret "$si_config_repo" -n argocd --ignore-not-found

  if [[ -n "$git_token" && -n "$git_user" ]]; then
    kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${si_config_repo}
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: ${deploy_repo_url}
  password: ${git_token}
  username: ${git_user}
EOF
  else
    kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${si_config_repo}
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: ${deploy_repo_url}
EOF
  fi

  log_info "ArgoCD GitHub repository secret created."
}

install_root_app() {
  log_info "=== Phase 4: Deploying orchestrator (root-app) ==="

  local cluster_yaml="$cwd/${ORCH_INSTALLER_PROFILE}.yaml"
  local root_app_chart="$REPO_DIR/argocd/root-app"

  if [[ ! -d "$root_app_chart" ]]; then
    log_error "Root-app chart not found: $root_app_chart"
    exit 1
  fi

  if [[ ! -f "$cluster_yaml" ]]; then
    log_error "Cluster config not found: $cluster_yaml"
    exit 1
  fi

  # Clean up stale jobs before re-deploy
  kubectl delete job -n gitea -l managed-by=edge-manageability-framework \
    --ignore-not-found=true 2>/dev/null || true
  kubectl delete sts -n orch-database postgresql --ignore-not-found=true 2>/dev/null || true
  kubectl delete job -n orch-infra credentials --ignore-not-found=true 2>/dev/null || true
  kubectl delete job -n orch-infra loca-credentials --ignore-not-found=true 2>/dev/null || true
  kubectl delete secret -l managed-by=edge-manageability-framework -A \
    --ignore-not-found=true 2>/dev/null || true

  # Set up repo credentials and push
  local gitea_url
  gitea_url=$(get_gitea_service_url)

  if [[ "$INSTALL_GITEA" == "true" ]]; then
    push_repo_to_gitea "$gitea_url"
    create_gitea_creds_secret "$gitea_url"
  else
    if [[ -z "${WORK_DIR:-}" ]]; then
      # Running from git checkout — extract tarball for local root-app
      log_info "GitHub mode: using local repo checkout for root-app"
    fi
    create_github_creds_secret
  fi

  # Install root-app via Helm
  log_info "Installing root-app Helm chart..."
  helm upgrade --install root-app "$root_app_chart" \
    -f "$cluster_yaml" \
    -n "$apps_ns" --create-namespace

  log_info "Orchestrator deployment initiated."
}

################################################################################
#                     PHASE 5: POSTGRESQL MIGRATION
################################################################################

save_postgres_passwords() {
  log_info "=== Phase 5a: Saving PostgreSQL passwords ==="

  if [[ -s postgres-secrets-password.txt ]]; then
    log_info "postgres-secrets-password.txt already exists, skipping save."
    return 0
  fi

  local alerting catalog inventory iam_tenancy platform_keycloak vault_pw postgresql mps rps

  alerting=$(kubectl get secret alerting-local-postgresql -n orch-infra \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  catalog=$(kubectl get secret app-orch-catalog-local-postgresql -n orch-app \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  inventory=$(kubectl get secret inventory-local-postgresql -n orch-infra \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  iam_tenancy=$(kubectl get secret iam-tenancy-local-postgresql -n orch-iam \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  platform_keycloak=$(kubectl get secret platform-keycloak-local-postgresql -n orch-platform \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  vault_pw=$(kubectl get secret vault-local-postgresql -n orch-platform \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  postgresql=$(kubectl get secret orch-database-postgresql -n orch-database \
    -o jsonpath='{.data.password}' 2>/dev/null || true)
  mps=$(kubectl get secret mps-local-postgresql -n orch-infra \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)
  rps=$(kubectl get secret rps-local-postgresql -n orch-infra \
    -o jsonpath='{.data.PGPASSWORD}' 2>/dev/null || true)

  {
    echo "Alerting: $alerting"
    echo "CatalogService: $catalog"
    echo "Inventory: $inventory"
    echo "IAMTenancy: $iam_tenancy"
    echo "PlatformKeycloak: $platform_keycloak"
    echo "Vault: $vault_pw"
    echo "PostgreSQL: $postgresql"
    echo "Mps: $mps"
    echo "Rps: $rps"
  } > postgres-secrets-password.txt

  log_info "PostgreSQL passwords saved to postgres-secrets-password.txt"
}

delete_mps_rps_secrets() {
  log_info "=== Phase 5b: Deleting MPS/RPS secrets for recreation ==="

  if kubectl get secret mps -n orch-infra >/dev/null 2>&1; then
    kubectl get secret mps -n orch-infra -o yaml > mps_secret.yaml
    kubectl delete secret mps -n orch-infra
    log_info "MPS secret backed up and deleted"
  fi

  if kubectl get secret rps -n orch-infra >/dev/null 2>&1; then
    kubectl get secret rps -n orch-infra -o yaml > rps_secret.yaml
    kubectl delete secret rps -n orch-infra
    log_info "RPS secret backed up and deleted"
  fi
}

patch_secrets() {
  log_info "Patching secrets with saved passwords..."

  # Read passwords from file
  local alerting="" catalog="" inventory="" iam_tenancy=""
  local platform_keycloak="" vault_pw="" postgresql="" mps="" rps=""

  if [[ -s postgres-secrets-password.txt ]]; then
    while IFS=': ' read -r key value; do
      case "$key" in
        Alerting) alerting="$value" ;;
        CatalogService) catalog="$value" ;;
        Inventory) inventory="$value" ;;
        IAMTenancy) iam_tenancy="$value" ;;
        PlatformKeycloak) platform_keycloak="$value" ;;
        Vault) vault_pw="$value" ;;
        PostgreSQL) postgresql="$value" ;;
        Mps) mps="$value" ;;
        Rps) rps="$value" ;;
      esac
    done < postgres-secrets-password.txt
  fi

  wait_for_app_synced_healthy postgresql-secrets "$apps_ns"
  check_and_patch_sync_app postgresql-secrets "$apps_ns"
  wait_for_app_synced_healthy postgresql-secrets "$apps_ns"

  # If postgresql-secrets still not healthy, try root-app sync
  local app_status
  app_status=$(kubectl get application postgresql-secrets -n "$apps_ns" \
    -o jsonpath='{.status.sync.status} {.status.health.status}' \
    2>/dev/null || echo "NotFound NotFound")
  if [[ "$app_status" != "Synced Healthy" ]]; then
    check_and_patch_sync_app root-app "$apps_ns"
  fi

  # Wait for secrets to appear
  local secrets_to_check=(
    "orch-app:app-orch-catalog-local-postgresql"
    "orch-app:app-orch-catalog-reader-local-postgresql"
    "orch-iam:iam-tenancy-local-postgresql"
    "orch-iam:iam-tenancy-reader-local-postgresql"
    "orch-infra:alerting-local-postgresql"
    "orch-infra:alerting-reader-local-postgresql"
    "orch-infra:inventory-local-postgresql"
    "orch-infra:inventory-reader-local-postgresql"
    "orch-platform:platform-keycloak-local-postgresql"
    "orch-platform:platform-keycloak-reader-local-postgresql"
    "orch-platform:vault-local-postgresql"
    "orch-platform:vault-reader-local-postgresql"
    "orch-infra:mps-local-postgresql"
    "orch-infra:mps-reader-local-postgresql"
    "orch-infra:rps-local-postgresql"
    "orch-infra:rps-reader-local-postgresql"
  )

  local max_wait=600 check_interval=5

  log_info "Waiting for all required secrets to exist..."
  for entry in "${secrets_to_check[@]}"; do
    local ns="${entry%%:*}" secret_name="${entry##*:}"
    local elapsed=0
    while ! kubectl get secret "$secret_name" -n "$ns" >/dev/null 2>&1; do
      if (( elapsed >= max_wait )); then
        log_error "Timeout waiting for secret $secret_name in $ns"
        exit 1
      fi
      sleep "$check_interval"
      elapsed=$((elapsed + check_interval))
    done
  done
  log_info "All required secrets exist."

  # Patch all database secrets
  kubectl patch secret -n orch-app app-orch-catalog-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$catalog\"}}" --type=merge
  kubectl patch secret -n orch-app app-orch-catalog-reader-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$catalog\"}}" --type=merge
  kubectl patch secret -n orch-iam iam-tenancy-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$iam_tenancy\"}}" --type=merge
  kubectl patch secret -n orch-iam iam-tenancy-reader-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$iam_tenancy\"}}" --type=merge
  kubectl patch secret -n orch-infra alerting-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$alerting\"}}" --type=merge
  kubectl patch secret -n orch-infra alerting-reader-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$alerting\"}}" --type=merge
  kubectl patch secret -n orch-infra inventory-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$inventory\"}}" --type=merge
  kubectl patch secret -n orch-infra inventory-reader-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$inventory\"}}" --type=merge
  kubectl patch secret -n orch-platform platform-keycloak-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$platform_keycloak\"}}" --type=merge
  kubectl patch secret -n orch-platform platform-keycloak-reader-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$platform_keycloak\"}}" --type=merge
  kubectl patch secret -n orch-platform vault-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$vault_pw\"}}" --type=merge
  kubectl patch secret -n orch-platform vault-reader-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$vault_pw\"}}" --type=merge
  kubectl patch secret -n orch-infra mps-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$mps\"}}" --type=merge
  kubectl patch secret -n orch-infra mps-reader-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$mps\"}}" --type=merge
  kubectl patch secret -n orch-infra rps-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$rps\"}}" --type=merge
  kubectl patch secret -n orch-infra rps-reader-local-postgresql \
    -p "{\"data\": {\"PGPASSWORD\": \"$rps\"}}" --type=merge

  # CloudNativePG secrets (if applicable)
  if kubectl get secret orch-app-app-orch-catalog -n orch-database >/dev/null 2>&1; then
    kubectl patch secret -n orch-database orch-app-app-orch-catalog \
      -p "{\"data\": {\"password\": \"$catalog\"}}" --type=merge
    kubectl patch secret -n orch-database orch-iam-iam-tenancy \
      -p "{\"data\": {\"password\": \"$iam_tenancy\"}}" --type=merge
    kubectl patch secret -n orch-database orch-infra-alerting \
      -p "{\"data\": {\"password\": \"$alerting\"}}" --type=merge
    kubectl patch secret -n orch-database orch-infra-inventory \
      -p "{\"data\": {\"password\": \"$inventory\"}}" --type=merge
    kubectl patch secret -n orch-database orch-platform-platform-keycloak \
      -p "{\"data\": {\"password\": \"$platform_keycloak\"}}" --type=merge
    kubectl patch secret -n orch-database orch-platform-vault \
      -p "{\"data\": {\"password\": \"$vault_pw\"}}" --type=merge
    kubectl patch secret -n orch-database orch-infra-mps \
      -p "{\"data\": {\"password\": \"$mps\"}}" --type=merge
    kubectl patch secret -n orch-database orch-infra-rps \
      -p "{\"data\": {\"password\": \"$rps\"}}" --type=merge
  fi

  # Patch Keycloak secret with username & password fields
  if kubectl get secret platform-keycloak -n orch-platform >/dev/null 2>&1; then
    local admin_password
    admin_password=$(kubectl get secret platform-keycloak -n orch-platform \
      -o jsonpath='{.data.admin-password}' 2>/dev/null | base64 -d 2>/dev/null || true)
    if [[ -n "$admin_password" ]]; then
      kubectl patch secret platform-keycloak -n orch-platform --type='merge' \
        -p "{\"stringData\": {\"username\": \"admin\", \"password\": \"$admin_password\"}}" || true
    fi
  fi

  # Patch PostgreSQL main secret
  kubectl patch secret -n orch-database orch-database-postgresql \
    -p "{\"data\": {\"password\": \"$postgresql\"}}" --type=merge

  log_info "All secrets patched."
}

migrate_postgres_to_cnpg() {
  log_info "=== Phase 5: PostgreSQL migration to CloudNativePG ==="

  # Delete rke2-metrics-server if present
  helm delete -n kube-system rke2-metrics-server 2>/dev/null || true

  resync_all_apps

  # Wait for postgresql-secrets to sync
  log_info "Waiting for postgresql-secrets application..."
  local start_time timeout_s=3600
  start_time=$(date +%s)

  set +e
  while true; do
    local app_status
    app_status=$(kubectl get application postgresql-secrets -n "$apps_ns" \
      -o jsonpath='{.status.sync.status} {.status.health.status}' 2>/dev/null || true)
    if [[ "$app_status" == "Synced Healthy" ]]; then
      log_info "postgresql-secrets is Synced and Healthy."
      break
    fi
    local current_time elapsed
    current_time=$(date +%s)
    elapsed=$((current_time - start_time))
    if (( elapsed > timeout_s )); then
      log_error "Timeout waiting for postgresql-secrets (${timeout_s}s)"
      exit 1
    fi
    log_info "Waiting for postgresql-secrets (status: ${app_status:-pending}, ${elapsed}s)"
    sleep 5
  done
  set -e

  # Delete old PostgreSQL (from upgrade_postgres.sh)
  delete_postgres

  # Stop root-app sync
  kubectl patch application root-app -n "$apps_ns" --type merge \
    -p '{"operation":null}' || true
  kubectl patch application root-app -n "$apps_ns" --type json \
    -p '[{"op": "remove", "path": "/status/operationState"}]' || true

  # Force postgresql sync with hook strategy
  cat <<EOF | sudo tee /tmp/sync-postgresql-patch.yaml >/dev/null
operation:
  sync:
    syncStrategy:
      hook: {}
EOF
  kubectl patch -n "$apps_ns" application root-app \
    --patch-file /tmp/sync-postgresql-patch.yaml --type merge

  # Wait for postgresql-secrets again after root-app sync
  start_time=$(date +%s)
  set +e
  while true; do
    local app_status
    app_status=$(kubectl get application postgresql-secrets -n "$apps_ns" \
      -o jsonpath='{.status.sync.status} {.status.health.status}' 2>/dev/null || true)
    if [[ "$app_status" == "Synced Healthy" ]]; then
      log_info "postgresql-secrets is Synced and Healthy."
      break
    fi
    local current_time elapsed
    current_time=$(date +%s)
    elapsed=$((current_time - start_time))
    if (( elapsed > timeout_s )); then
      log_error "Timeout waiting for postgresql-secrets after resync (${timeout_s}s)"
      exit 1
    fi
    sleep 5
  done
  set -e

  # Vault unseal
  vault_unseal

  # Resync and patch secrets
  resync_all_apps
  sleep 120
  patch_secrets
  sleep 10

  # Apply saved PostgreSQL superuser secret (stripped of metadata)
  if [[ -f postgres_secret.yaml ]]; then
    yq e '
      del(.metadata.labels) |
      del(.metadata.annotations) |
      del(.metadata.ownerReferences) |
      del(.metadata.finalizers) |
      del(.metadata.managedFields) |
      del(.metadata.resourceVersion) |
      del(.metadata.uid) |
      del(.metadata.creationTimestamp)
    ' postgres_secret.yaml | kubectl apply -f -
  fi

  sleep 30

  # Wait for CloudNativePG primary pod
  log_info "Waiting for CloudNativePG primary pod..."
  start_time=$(date +%s)
  local pg_timeout=300

  set +e
  while true; do
    local pod_status
    pod_status=$(kubectl get pods -n orch-database \
      -l cnpg.io/cluster=postgresql-cluster,cnpg.io/instanceRole=primary \
      -o jsonpath='{.items[0].status.phase}' 2>/dev/null || true)
    if [[ "$pod_status" == "Running" ]]; then
      log_info "PostgreSQL CNPG pod is Running."
      sleep 30
      break
    fi
    local current_time elapsed
    current_time=$(date +%s)
    elapsed=$((current_time - start_time))
    if (( elapsed > pg_timeout )); then
      log_error "Timeout waiting for PostgreSQL CNPG pod (${pg_timeout}s)"
      exit 1
    fi
    log_info "Waiting for PostgreSQL (status: ${pod_status:-pending}, ${elapsed}s)"
    sleep 5
  done
  set -e

  # Restore PostgreSQL from backup (from upgrade_postgres.sh)
  restore_postgres

  log_info "Database user passwords updated."

  # Unseal vault again
  vault_unseal

  log_info "PostgreSQL migration completed."
}

################################################################################
#                     PHASE 6: SERVICE RECOVERY
################################################################################

restore_mps_rps_secrets() {
  log_info "=== Phase 6a: Restoring MPS/RPS secrets ==="

  if [[ -s mps_secret.yaml ]]; then
    kubectl apply -f mps_secret.yaml
    log_info "MPS secret restored"
  fi

  if [[ -s rps_secret.yaml ]]; then
    kubectl apply -f rps_secret.yaml
    log_info "RPS secret restored"
  fi
}

fix_mps_rps_connections() {
  log_info "=== Phase 6b: Updating MPS/RPS connection strings for CloudNativePG ==="

  local mps_password rps_password
  mps_password=$(kubectl get secret mps-local-postgresql -n orch-infra \
    -o jsonpath='{.data.PGPASSWORD}' | base64 -d)
  rps_password=$(kubectl get secret rps-local-postgresql -n orch-infra \
    -o jsonpath='{.data.PGPASSWORD}' | base64 -d)

  # MPS connection string
  local mps_conn
  mps_conn="postgresql://orch-infra-mps_user:${mps_password}@postgresql-cluster-rw.orch-database/orch-infra-mps?search_path=public&sslmode=disable"
  local mps_b64
  mps_b64=$(echo -n "$mps_conn" | base64 -w 0)
  kubectl patch secret mps -n orch-infra \
    -p "{\"data\":{\"connectionString\":\"$mps_b64\"}}" --type=merge

  # RPS connection string
  local rps_conn
  rps_conn="postgresql://orch-infra-rps_user:${rps_password}@postgresql-cluster-rw.orch-database/orch-infra-rps?search_path=public&sslmode=disable"
  local rps_b64
  rps_b64=$(echo -n "$rps_conn" | base64 -w 0)
  kubectl patch secret rps -n orch-infra \
    -p "{\"data\":{\"connectionString\":\"$rps_b64\"}}" --type=merge

  log_info "MPS/RPS connection strings updated to use postgresql-cluster-rw.orch-database"
}

restart_services() {
  log_info "=== Phase 6c: Restarting services ==="

  kubectl rollout restart deployment rps -n orch-infra
  kubectl rollout restart deployment mps -n orch-infra
  log_info "MPS/RPS restarted"

  kubectl rollout restart deployment inventory -n orch-infra
  log_info "inventory restarted"

  kubectl rollout restart deployment onboarding-manager -n orch-infra
  log_info "onboarding-manager restarted"

  kubectl rollout restart deployment dkam -n orch-infra
  log_info "dkam restarted"

  restart_statefulset keycloak-tenant-controller-set orch-platform

  resync_all_apps
  sleep 10

  # Harbor restarts (skip for onprem-vpro profile)
  if [[ "${ORCH_INSTALLER_PROFILE:-}" != "onprem-vpro" ]]; then
    restart_statefulset harbor-oci-database orch-harbor || true
    kubectl rollout restart deployment harbor-oci-core -n orch-harbor || true
    log_info "harbor restarted"
  else
    log_info "Skipping Harbor restarts for onprem-vpro profile"
  fi
}

restore_gitea_vault_creds() {
  log_info "=== Phase 6d: Restoring Gitea credentials to Vault ==="

  # Sync root-app
  if [[ -f /tmp/argo-cd/sync-patch.yaml ]]; then
    kubectl patch application root-app -n "$apps_ns" \
      --patch-file /tmp/argo-cd/sync-patch.yaml --type merge || true
  fi

  if [[ "$INSTALL_GITEA" == "true" ]]; then
    local password username
    password=$(kubectl get secret app-gitea-credential -n orch-platform \
      -o jsonpath="{.data.password}" 2>/dev/null | base64 -d || true)
    username=$(kubectl get secret app-gitea-credential -n orch-platform \
      -o jsonpath="{.data.username}" 2>/dev/null | base64 -d || true)

    if [[ -n "$password" && -n "$username" ]]; then
      kubectl exec -it vault-0 -n orch-platform -c vault -- \
        vault kv put secret/ma_git_service \
        username="$username" password="$password" 2>/dev/null || true
      log_info "Gitea credentials stored in Vault"
    fi
  fi

  # Delete fleet-gitrepo-cred secrets
  kubectl get secret --all-namespaces --no-headers 2>/dev/null \
    | awk '/fleet-gitrepo-cred/ {print $1, $2}' \
    | while IFS=' ' read -r ns secret; do
        log_info "Deleting secret $secret in namespace $ns"
        kubectl delete secret "$secret" -n "$ns" || true
      done
}

################################################################################
#                     PHASE 7: CLEANUP
################################################################################

cleanup_external_secrets() {
  log_info "=== Phase 7a: Cleaning up external-secrets ==="

  for crd in clustersecretstores.external-secrets.io \
             secretstores.external-secrets.io \
             externalsecrets.external-secrets.io; do
    if kubectl get crd "$crd" >/dev/null 2>&1; then
      kubectl delete crd "$crd" &
      kubectl patch "crd/$crd" -p '{"metadata":{"finalizers":[]}}' --type=merge
    fi
  done

  # Apply External Secrets CRDs with server-side apply
  log_info "Applying external-secrets CRDs v0.20.4..."
  kubectl apply --server-side=true --force-conflicts \
    -f https://raw.githubusercontent.com/external-secrets/external-secrets/refs/tags/v0.20.4/deploy/crds/bundle.yaml || true

  # Final vault unseal
  vault_unseal
  log_info "Vault unsealed successfully."

  # Stop root-app sync
  kubectl patch application root-app -n "$apps_ns" --type merge \
    -p '{"operation":null}' || true
  kubectl patch application root-app -n "$apps_ns" --type json \
    -p '[{"op": "remove", "path": "/status/operationState"}]' || true
  sleep 5

  # Delete external-secrets application
  kubectl patch application external-secrets -n "$apps_ns" \
    --type=json -p='[{"op":"remove","path":"/metadata/finalizers"}]' 2>/dev/null || true
  kubectl delete application external-secrets -n "$apps_ns" \
    --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
  sleep 5

  log_info "external-secrets cleanup done."
}

cleanup_kyverno() {
  log_info "=== Phase 7b: Cleaning up Kyverno policies ==="

  for policy in restart-mps-deployment-on-secret-change \
                restart-rps-deployment-on-secret-change; do
    if kubectl get clusterpolicy "$policy" -o name >/dev/null 2>&1; then
      kubectl delete clusterpolicy "$policy"
      log_info "Deleted ClusterPolicy: $policy"
    fi
  done
}

final_prune_sync() {
  log_info "=== Phase 7c: Final prune sync ==="

  kubectl patch -n "$apps_ns" application root-app --type merge --patch '{
    "operation": {
      "initiatedBy": { "username": "admin" },
      "sync": {
        "prune": true,
        "syncStrategy": { "hook": {} }
      }
    }
  }'

  sleep 30

  delete_nginx_if_any
}

remove_gitea_if_disabled() {
  if [[ "${INSTALL_GITEA}" == "false" ]]; then
    log_info "=== Phase 7d: Removing Gitea ==="
    if helm list -n gitea | awk '{print $1}' | grep -q "^gitea$"; then
      helm uninstall gitea -n gitea
      log_info "Gitea uninstalled"
    else
      log_info "Gitea release not found, skipping"
    fi
  fi
}

################################################################################
#                     KYVERNO JOB CLEANUP
################################################################################

cleanup_kyverno_jobs() {
  if kubectl get job kyverno-clean-reports -n kyverno >/dev/null 2>&1; then
    log_info "Cleaning up kyverno-clean-reports job..."
    kubectl delete job kyverno-clean-reports -n kyverno &
    kubectl delete pods -l job-name="kyverno-clean-reports" -n kyverno &
    kubectl patch job kyverno-clean-reports -n kyverno --type=merge \
      -p='{"metadata":{"finalizers":[]}}' || true
  fi
}

################################################################################
#                     CLI PARSING
################################################################################

usage() {
  cat >&2 <<EOF
Purpose:
  Post-upgrade script for OnPrem Edge Orchestrator (deb-free).
  Upgrades Gitea, ArgoCD, deploys root-app, migrates PostgreSQL to
  CloudNativePG, restarts services, and performs cleanup.

Prerequisites:
  - pre-orch-upgrade.sh has completed successfully
  - onprem.env file is configured
  - kubectl, helm, yq, openssl are available
  - Repo tarball in repo_archives/ (or running from a git checkout)

Usage:
  $(basename "$0") [options]

Options:
  -l    Use local packages (skip artifact download)
  -s    Skip interactive prompts (non-interactive mode)
  -h    Show this help message
EOF
}

################################################################################
#                     MAIN
################################################################################

main() {
  local help_flag="" local_flag="" skip_flag=""

  while getopts 'hls' flag; do
    case "${flag}" in
      h) help_flag="true" ;;
      l) local_flag="true" ;;
      s) skip_flag="true" ;;
      *) help_flag="true" ;;
    esac
  done

  if [[ "${help_flag:-}" == "true" ]]; then
    usage
    exit 0
  fi

  if [[ "${local_flag:-}" == "true" ]]; then
    USE_LOCAL_PACKAGES="true"
  fi

  if [[ "${skip_flag:-}" == "true" ]]; then
    SKIP_INTERACTIVE="true"
  fi

  check_prerequisites

  # Phase 1: Configuration
  retrieve_and_update_config
  setup_working_dir
  cleanup_gitea_secrets
  apply_cluster_config

  # Pre-deployment cleanup
  cleanup_kyverno_jobs

  # Phase 2: Gitea Upgrade
  upgrade_gitea

  # Phase 3: ArgoCD Upgrade
  upgrade_argocd

  # Phase 5a-b: Save DB passwords and backup MPS/RPS secrets
  save_postgres_passwords
  delete_mps_rps_secrets

  # Phase 4: Orchestrator Deployment (root-app)
  install_root_app

  log_info "Edge Orchestrator upgrade initiated, waiting for deployment..."

  # Phase 5: PostgreSQL Migration
  migrate_postgres_to_cnpg

  # Phase 6: Service Recovery
  restore_mps_rps_secrets
  fix_mps_rps_connections
  restart_services
  restore_gitea_vault_creds

  # Phase 7: Cleanup
  cleanup_external_secrets
  cleanup_kyverno
  final_prune_sync
  remove_gitea_if_disabled

  sleep 10

  log_info "================================================"
  log_info "Post-upgrade completed!"
  log_info "Wait ~5-10 minutes for ArgoCD to sync all applications."
  log_info "Then run: ./after_upgrade_restart.sh"
  log_info "================================================"
}

main "$@"