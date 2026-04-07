#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: pre-orch-upgrade.sh
# Description: Deb-free pre-upgrade script for Edge Orchestrator.
#              This script handles the infrastructure-level upgrade steps
#              that previously depended on onprem-config-installer and
#              onprem-ke-installer debian packages:
#               - Loading configuration from onprem.env
#               - Setting up OS-level dependencies (sysctl, yq, helm, kernel modules)
#               - Upgrading the Kubernetes cluster (RKE2, K3s, or KIND)
#               - Configuring basic cluster components (OpenEBS LocalPV)
#
# Usage:
#   ./pre-orch-upgrade.sh [kind|k3s|rke2] upgrade [options]
#   ./pre-orch-upgrade.sh upgrade [options]        # auto-detects provider
#   ./pre-orch-upgrade.sh [options]                 # auto-detects provider, implies 'upgrade'
#
# After this script completes, run the post-upgrade orchestrator script
# (onprem_upgrade.sh) to upgrade ArgoCD, Gitea, and Edge Orchestrator apps.

set -euo pipefail

# Prefer binaries installed to /usr/local/bin (e.g., avoid asdf shims).
export PATH="/usr/local/bin:${PATH}"

# Source environment configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/onprem.env"

################################
# Logging
################################

LOG_FILE="pre_orch_upgrade_$(date +'%Y%m%d_%H%M%S').log"
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

log_info "Starting pre-orch-upgrade script"
log_info "Log file: $FULL_LOG_PATH"

################################
# Defaults / Configuration
################################

WAIT_TIMEOUT_SECONDS="${WAIT_TIMEOUT_SECONDS:-600}"
WAIT_INTERVAL_SECONDS="${WAIT_INTERVAL_SECONDS:-5}"
LOCALPV_VERSION="${LOCALPV_VERSION:-4.3.0}"
SKIP_OS_CONFIG="${SKIP_OS_CONFIG:-false}"
HELM_VERSION="${HELM_VERSION:-}"

# KIND
KIND_CLUSTER_NAME_DEFAULT="kind-cluster"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-$KIND_CLUSTER_NAME_DEFAULT}"
KIND_API_PORT="${KIND_API_PORT:-6443}"
KIND_VERSION="${KIND_VERSION:-}"
KIND_FORCE_RECREATE="${KIND_FORCE_RECREATE:-false}"

# K3s
K3S_VERSION_DEFAULT="v1.34.5+k3s1"
K3S_VERSION="${K3S_VERSION:-$K3S_VERSION_DEFAULT}"

# RKE2
RKE2_TARGET_VERSION_DEFAULT="v1.34.5+rke2r1"
RKE2_TARGET_VERSION="${RKE2_TARGET_VERSION:-$RKE2_TARGET_VERSION_DEFAULT}"
DOCKER_USERNAME="${DOCKER_USERNAME:-}"
DOCKER_PASSWORD="${DOCKER_PASSWORD:-}"

# System-upgrade-controller version for RKE2 upgrades
SYSTEM_UPGRADE_CONTROLLER_VERSION="v0.13.2"

################################
# Helpers
################################

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "❌ Required command not found in PATH: $cmd"
    exit 1
  fi
}

cmd_exists() {
  command -v "$1" >/dev/null 2>&1
}

# Auto-detect the installed Kubernetes provider by checking
# systemd services, binaries, and kubeconfig context.
detect_k8s_provider() {
  # Check RKE2 first (most common for on-prem)
  if systemctl list-unit-files rke2-server.service >/dev/null 2>&1 && \
     systemctl is-enabled rke2-server.service >/dev/null 2>&1; then
    echo "rke2"
    return 0
  fi
  if [[ -d /etc/rancher/rke2 ]] || cmd_exists rke2; then
    echo "rke2"
    return 0
  fi

  # Check K3s
  if systemctl list-unit-files k3s.service >/dev/null 2>&1 && \
     systemctl is-enabled k3s.service >/dev/null 2>&1; then
    echo "k3s"
    return 0
  fi
  if [[ -d /etc/rancher/k3s ]] || cmd_exists k3s; then
    echo "k3s"
    return 0
  fi

  # Check KIND (look for kind binary + running cluster, or kubeconfig context)
  if cmd_exists kind && kind get clusters 2>/dev/null | grep -q .; then
    echo "kind"
    return 0
  fi
  if kubectl config current-context 2>/dev/null | grep -q "^kind-"; then
    echo "kind"
    return 0
  fi

  # Could not detect
  return 1
}

install_helm() {
  if cmd_exists helm; then
    echo "✅ helm is already installed: $(helm version --short 2>/dev/null || echo 'unknown')"
    return 0
  fi

  if ! cmd_exists curl && ! cmd_exists wget; then
    echo "❌ helm is required but is not installed. Need curl or wget to install it automatically."
    echo "   Install curl (or wget) and retry, or install helm manually (https://helm.sh/docs/intro/install/)."
    exit 1
  fi

  echo "👉 helm not found; installing helm v3..."

  local tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "${tmp:-}"' RETURN

  local installer_url="https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3"
  local installer_path="$tmp/get-helm-3.sh"

  if cmd_exists curl; then
    curl -fsSL "$installer_url" -o "$installer_path"
  else
    wget -qO "$installer_path" "$installer_url"
  fi
  chmod +x "$installer_path"

  if [[ -w /usr/local/bin ]]; then
    if [[ -n "${HELM_VERSION}" ]]; then
      HELM_INSTALL_DIR="/usr/local/bin" DESIRED_VERSION="${HELM_VERSION}" "$installer_path"
    else
      HELM_INSTALL_DIR="/usr/local/bin" "$installer_path"
    fi
  elif cmd_exists sudo; then
    if [[ -n "${HELM_VERSION}" ]]; then
      sudo -E env DESIRED_VERSION="${HELM_VERSION}" "$installer_path"
    else
      sudo -E "$installer_path"
    fi
  else
    mkdir -p "${HOME}/.local/bin"
    if [[ -n "${HELM_VERSION}" ]]; then
      HELM_INSTALL_DIR="${HOME}/.local/bin" DESIRED_VERSION="${HELM_VERSION}" "$installer_path"
    else
      HELM_INSTALL_DIR="${HOME}/.local/bin" "$installer_path"
    fi
  fi

  if ! cmd_exists helm; then
    echo "❌ helm installation did not succeed; please install helm manually and retry."
    exit 1
  fi
  echo "✅ helm installed: $(helm version --short 2>/dev/null || echo 'unknown')"
}

install_yq() {
  if cmd_exists yq; then
    echo "✅ yq is already installed: $(yq --version 2>/dev/null || echo 'unknown')"
    return 0
  fi

  require_cmd curl

  echo "👉 yq not found; installing yq..."

  local arch
  arch="$(uname -m)"
  local yq_binary="yq_linux_amd64"
  case "${arch}" in
    x86_64|amd64) yq_binary="yq_linux_amd64" ;;
    aarch64|arm64) yq_binary="yq_linux_arm64" ;;
    *)
      echo "❌ Unsupported architecture for yq: ${arch}"
      exit 1
      ;;
  esac

  local tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "${tmp:-}"' RETURN

  # Get latest version
  local version
  version="$(curl -s https://api.github.com/repos/mikefarah/yq/releases/latest | grep '"tag_name"' | cut -d '"' -f 4)"
  if [[ -z "${version}" ]]; then
    version="v4.44.1"  # fallback
  fi

  local yq_url="https://github.com/mikefarah/yq/releases/download/${version}/${yq_binary}.tar.gz"
  echo "👉 Downloading yq ${version} from ${yq_url}..."
  curl -fsSL -o "$tmp/${yq_binary}.tar.gz" "$yq_url"
  tar xf "$tmp/${yq_binary}.tar.gz" -C "$tmp"

  if [[ -f "$tmp/${yq_binary}" ]]; then
    sudo mv "$tmp/${yq_binary}" /usr/local/bin/yq
    sudo chmod +x /usr/local/bin/yq
  elif [[ -f "$tmp/yq" ]]; then
    sudo mv "$tmp/yq" /usr/local/bin/yq
    sudo chmod +x /usr/local/bin/yq
  else
    echo "❌ yq binary not found after extraction"
    exit 1
  fi

  if ! cmd_exists yq; then
    echo "❌ yq installation did not succeed; please install yq manually and retry."
    exit 1
  fi
  echo "✅ yq installed: $(yq --version 2>/dev/null || echo 'unknown')"
}

usage() {
  cat >&2 <<EOF
Usage:
  $(basename "$0") [kind|k3s|rke2] [upgrade] [options]

Description:
  Deb-free pre-upgrade script for Edge Orchestrator infrastructure.
  Handles OS configuration, Kubernetes cluster upgrade, and basic
  cluster components (OpenEBS LocalPV).

  The Kubernetes provider is auto-detected if not specified.
  The 'upgrade' action is implied and can be omitted.

Global options:
  --wait-timeout <seconds>       Default: ${WAIT_TIMEOUT_SECONDS}
  --wait-interval <seconds>      Default: ${WAIT_INTERVAL_SECONDS}
  --localpv-version <version>    Default: ${LOCALPV_VERSION}
  --helm-version <version>       Default: latest
  --skip-os-config               Skip OS-level configuration step

KIND options:
  --cluster-name <name>          Default: ${KIND_CLUSTER_NAME_DEFAULT}
  --api-port <port>              Default: ${KIND_API_PORT}
  --kind-version <version>       Default: latest
  --force-recreate               Delete and recreate cluster even if version matches

K3s options:
  --k3s-version <version>        Default: ${K3S_VERSION_DEFAULT}
  --docker-username <user>       Optional (for Docker Hub auth)
  --docker-password <pass>       Optional (for Docker Hub auth)

RKE2 options:
  --rke2-target-version <ver>    Default: ${RKE2_TARGET_VERSION_DEFAULT}
  --docker-username <user>       Optional (for Docker Hub auth)
  --docker-password <pass>       Optional (for Docker Hub auth)

Examples:
  $(basename "$0")                                           # auto-detect provider
  $(basename "$0") upgrade                                   # auto-detect, explicit action
  $(basename "$0") rke2 upgrade                              # explicit provider and action
  $(basename "$0") rke2 upgrade --rke2-target-version v1.34.4+rke2r1
  $(basename "$0") k3s upgrade --k3s-version v1.34.3+k3s1
  $(basename "$0") kind upgrade
  $(basename "$0") --skip-os-config                          # auto-detect, skip OS config
EOF
}

################################
# Wait / Readiness helpers
################################

wait_for_k8s_ready() {
  local kube_context="${1:-}"

  local kubectl_ctx_args=()
  if [[ -n "${kube_context}" ]]; then
    kubectl_ctx_args+=(--context "${kube_context}")
  fi

  echo "👉 Waiting for Kubernetes API to be reachable..."
  local deadline=$((SECONDS + WAIT_TIMEOUT_SECONDS))
  until kubectl "${kubectl_ctx_args[@]}" get --raw='/readyz' >/dev/null 2>&1; do
    if (( SECONDS >= deadline )); then
      echo "❌ Timed out waiting for API server to be ready after ${WAIT_TIMEOUT_SECONDS}s"
      kubectl "${kubectl_ctx_args[@]}" cluster-info || true
      exit 1
    fi
    sleep "${WAIT_INTERVAL_SECONDS}"
  done
  echo "✅ API server is ready"

  echo "👉 Waiting for all nodes to be Ready (timeout: ${WAIT_TIMEOUT_SECONDS}s)..."
  if ! kubectl "${kubectl_ctx_args[@]}" wait --for=condition=Ready node --all --timeout="${WAIT_TIMEOUT_SECONDS}s"; then
    echo "❌ Timed out waiting for nodes to become Ready"
    kubectl "${kubectl_ctx_args[@]}" get nodes -o wide || true
    kubectl "${kubectl_ctx_args[@]}" get pods -A || true
    exit 1
  fi
  echo "✅ All nodes are Ready"
}

################################
# OS Configuration
# (replaces onprem-config-installer deb)
################################

upgrade_os_config() {
  log_info "=== OS Configuration Upgrade ==="

  # -------------------------------------------------------
  # 1. sysctl tuning: inotify limits
  # -------------------------------------------------------
  echo "👉 Configuring sysctl inotify parameters..."

  local sysctl_file="/etc/sysctl.conf"
  local sysctl_params=(
    "fs.inotify.max_queued_events = 1048576"
    "fs.inotify.max_user_instances = 1048576"
    "fs.inotify.max_user_watches = 1048576"
  )

  for param in "${sysctl_params[@]}"; do
    local key="${param%% =*}"
    # Remove existing entries (commented or not) and append correct value
    if ! grep -q "^${key}" "${sysctl_file}" 2>/dev/null; then
      echo "${param}" | sudo tee -a "${sysctl_file}" >/dev/null
      echo "  Added: ${param}"
    else
      echo "  Already set: ${key}"
    fi
  done

  sudo sysctl -p >/dev/null 2>&1 || true
  echo "✅ sysctl parameters configured"

  # -------------------------------------------------------
  # 2. Install / update yq
  # -------------------------------------------------------
  echo "👉 Ensuring yq is installed..."
  install_yq

  # -------------------------------------------------------
  # 3. Install / update helm
  # -------------------------------------------------------
  echo "👉 Ensuring helm is installed..."
  install_helm

  # -------------------------------------------------------
  # 4. Ensure hostpath directories exist
  # -------------------------------------------------------
  echo "👉 Ensuring hostpath directories exist..."
  local hostpath_dirs=("/var/openebs/local")
  for dir in "${hostpath_dirs[@]}"; do
    if [[ ! -d "${dir}" ]]; then
      sudo mkdir -p "${dir}"
      echo "  Created: ${dir}"
    else
      echo "  Exists: ${dir}"
    fi
  done
  echo "✅ Hostpath directories ready"

  # -------------------------------------------------------
  # 5. Kernel modules for LVM snapshots
  # -------------------------------------------------------
  echo "👉 Configuring kernel modules for LVM snapshots..."

  local modules_file="/etc/modules-load.d/lv-snapshots.conf"
  printf "dm-snapshot\ndm-mirror\n" | sudo tee "${modules_file}" >/dev/null

  sudo modprobe dm-snapshot 2>/dev/null || log_warn "modprobe dm-snapshot failed (non-fatal)"
  sudo modprobe dm-mirror 2>/dev/null || log_warn "modprobe dm-mirror failed (non-fatal)"
  echo "✅ Kernel modules configured"

  log_info "=== OS Configuration Upgrade Complete ==="
}

################################
# OpenEBS LocalPV
################################

upgrade_openebs_localpv() {
  local kube_context="${1:-}"

  install_helm
  require_cmd kubectl

  local helm_ctx_args=()
  local kubectl_ctx_args=()
  if [[ -n "${kube_context}" ]]; then
    helm_ctx_args+=(--kube-context "${kube_context}")
    kubectl_ctx_args+=(--context "${kube_context}")
  fi

  echo "👉 Using OpenEBS LocalPV version: ${LOCALPV_VERSION}"

  echo "👉 Adding OpenEBS LocalPV Helm repo..."
  helm repo add openebs-localpv https://openebs.github.io/dynamic-localpv-provisioner >/dev/null 2>&1 || true

  echo "🔄 Updating Helm repos..."
  helm repo update >/dev/null

  echo "🚀 Upgrading OpenEBS LocalPV..."
  helm upgrade --install openebs-localpv openebs-localpv/localpv-provisioner \
    "${helm_ctx_args[@]}" \
    --version "${LOCALPV_VERSION}" \
    --namespace openebs-system --create-namespace \
    --set hostpathClass.enabled=true \
    --set hostpathClass.name=openebs-hostpath \
    --set hostpathClass.isDefaultClass=true \
    --set deviceClass.enabled=false \
    --wait --timeout 10m0s

  echo "📦 OpenEBS Pods in openebs-system namespace:"
  kubectl "${kubectl_ctx_args[@]}" get pods -n openebs-system
  echo "✅ OpenEBS LocalPV upgrade complete"
}

################################
# RKE2 Upgrade
# (replaces onprem-ke-installer deb)
################################

# RKE2 version ladder — must be traversed one minor version at a time.
# Matches the Go implementation in mage/upgrade.go:determineUpgradePath()
RKE2_VERSION_LADDER=(
  "v1.30.14+rke2r2"   # Patch update within 1.30
  "v1.31.13+rke2r1"   # Upgrade to 1.31
  "v1.32.9+rke2r1"    # Upgrade to 1.32
  "v1.33.5+rke2r1"    # Upgrade to 1.33
  "v1.34.1+rke2r1"    # Upgrade to 1.34.1
  "v1.34.5+rke2r1"    # Final target version
)

# Extract minor version: "v1.30.14+rke2r2" -> "v1.30"
_rke2_minor() {
  local ver="$1"
  echo "${ver}" | cut -d. -f1,2
}

# Determine the upgrade path from current version to target version.
# Outputs space-separated list of versions to upgrade through.
determine_rke2_upgrade_path() {
  local current="$1"
  local target="$2"

  local current_minor target_minor
  current_minor="$(_rke2_minor "${current}")"
  target_minor="$(_rke2_minor "${target}")"

  local start_idx=-1
  local end_idx=-1
  local i

  # Find starting index (version after current)
  for i in "${!RKE2_VERSION_LADDER[@]}"; do
    local v="${RKE2_VERSION_LADDER[$i]}"
    if [[ "${v}" == "${current}" ]]; then
      start_idx=$i
      break
    fi
    local v_minor
    v_minor="$(_rke2_minor "${v}")"
    if [[ "${v_minor}" == "${current_minor}" && ${start_idx} -eq -1 ]]; then
      start_idx=$i
    fi
  done

  if [[ ${start_idx} -eq -1 ]]; then
    start_idx=0
  else
    start_idx=$((start_idx + 1))
  fi

  # Find ending index
  for i in "${!RKE2_VERSION_LADDER[@]}"; do
    local v="${RKE2_VERSION_LADDER[$i]}"
    if [[ "${v}" == "${target}" ]]; then
      end_idx=$i
      break
    fi
    local v_minor
    v_minor="$(_rke2_minor "${v}")"
    if [[ "${v_minor}" == "${target_minor}" ]]; then
      end_idx=$i
    fi
  done

  if [[ ${end_idx} -eq -1 ]]; then
    end_idx=$(( ${#RKE2_VERSION_LADDER[@]} - 1 ))
  fi

  # Build path
  if (( start_idx <= end_idx )); then
    for (( i=start_idx; i<=end_idx; i++ )); do
      echo "${RKE2_VERSION_LADDER[$i]}"
    done
  fi
}

# Wait for node to report a specific kubelet version
rke2_wait_for_version() {
  local node_name="$1"
  local expected_version="$2"
  local deadline=$((SECONDS + WAIT_TIMEOUT_SECONDS))

  echo "👉 Waiting for node to report kubelet version ${expected_version}..."

  while true; do
    local found_version
    found_version="$(kubectl get "${node_name}" -o jsonpath='{.status.nodeInfo.kubeletVersion}' 2>/dev/null || echo "")"

    if [[ "${found_version}" == "${expected_version}" ]]; then
      echo "✅ Node reports version ${expected_version}"
      return 0
    fi

    if (( SECONDS >= deadline )); then
      echo "❌ Timed out waiting for node version ${expected_version} (current: ${found_version})"
      exit 1
    fi

    echo "  Current version: ${found_version}, waiting... ($(( deadline - SECONDS ))s remaining)"
    sleep "${WAIT_INTERVAL_SECONDS}"
  done
}

# Wait for node to be Ready and schedulable
rke2_wait_for_node_ready() {
  local node_name="$1"
  local deadline=$((SECONDS + WAIT_TIMEOUT_SECONDS))

  echo "👉 Waiting for node to be Ready and schedulable..."

  while true; do
    local ready
    ready="$(kubectl get "${node_name}" -o jsonpath='{range .status.conditions[?(@.type=="Ready")]}{.status}{end}' 2>/dev/null || echo "Unknown")"

    local schedulable="True"
    local taints
    taints="$(kubectl get "${node_name}" -o json 2>/dev/null | python3 -c "
import sys, json
data = json.load(sys.stdin)
taints = data.get('spec', {}).get('taints', [])
noschedule = [t for t in taints if t.get('effect') == 'NoSchedule']
print('True' if len(noschedule) == 0 else 'False')
" 2>/dev/null || echo "True")"
    schedulable="${taints}"

    if [[ "${ready}" == "True" && "${schedulable}" == "True" ]]; then
      echo "✅ Node is Ready and schedulable"
      return 0
    fi

    if (( SECONDS >= deadline )); then
      echo "❌ Timed out waiting for node Ready (ready=${ready}, schedulable=${schedulable})"
      exit 1
    fi

    echo "  Node status: ready=${ready}, schedulable=${schedulable}, waiting... ($(( deadline - SECONDS ))s remaining)"
    sleep "${WAIT_INTERVAL_SECONDS}"
  done
}

rke2_upgrade() {
  require_cmd sudo
  require_cmd kubectl
  require_cmd curl

  if [[ "$(uname -s)" != "Linux" ]]; then
    echo "❌ RKE2 upgrade currently supports Linux only"
    exit 1
  fi

  log_info "=== RKE2 Cluster Upgrade ==="

  # Get the node name
  local node_name
  node_name="$(kubectl get nodes -o name | head -1)"
  if [[ -z "${node_name}" ]]; then
    echo "❌ No nodes found in the cluster"
    exit 1
  fi
  echo "👉 Orchestrator node: ${node_name}"

  # Get current version
  local current_version
  current_version="$(kubectl get "${node_name}" -o jsonpath='{.status.nodeInfo.kubeletVersion}')"
  echo "👉 Current RKE2 version: ${current_version}"
  echo "👉 Target RKE2 version: ${RKE2_TARGET_VERSION}"

  # Check if already at target
  if [[ "${current_version}" == "${RKE2_TARGET_VERSION}" ]]; then
    echo "✅ RKE2 is already at the target version ${RKE2_TARGET_VERSION}. No upgrade needed."
    return 0
  fi

  # Determine upgrade path
  local -a upgrade_path
  mapfile -t upgrade_path < <(determine_rke2_upgrade_path "${current_version}" "${RKE2_TARGET_VERSION}")

  if [[ ${#upgrade_path[@]} -eq 0 ]]; then
    echo "❌ Unable to determine upgrade path from ${current_version} to ${RKE2_TARGET_VERSION}"
    exit 1
  fi

  echo "👉 Upgrade path: ${upgrade_path[*]}"

  # Install system-upgrade-controller
  echo "👉 Installing system-upgrade-controller ${SYSTEM_UPGRADE_CONTROLLER_VERSION}..."
  kubectl apply -f \
    "https://github.com/rancher/system-upgrade-controller/releases/download/${SYSTEM_UPGRADE_CONTROLLER_VERSION}/system-upgrade-controller.yaml"

  echo "👉 Waiting for system-upgrade-controller deployment to be ready..."
  kubectl rollout status deployment/system-upgrade-controller \
    -n system-upgrade --timeout=10m

  # Wait for CRDs to propagate
  sleep 15

  # Delete any existing upgrade Plans
  echo "👉 Cleaning up existing upgrade Plans..."
  kubectl delete -n system-upgrade plans.upgrade.cattle.io --all 2>/dev/null || true

  # Label node for upgrade
  echo "👉 Labeling node for upgrade..."
  kubectl label "${node_name}" rke2-upgrade=true --overwrite

  # Perform sequential upgrades through the version ladder
  local total=${#upgrade_path[@]}
  local step=0
  for rke2_version in "${upgrade_path[@]}"; do
    step=$((step + 1))
    echo ""
    log_info "--- RKE2 Upgrade Step ${step}/${total}: upgrading to ${rke2_version} ---"

    # Generate and apply upgrade Plan
    kubectl apply -f - <<EOF
apiVersion: upgrade.cattle.io/v1
kind: Plan
metadata:
  name: server-plan
  namespace: system-upgrade
  labels:
    rke2-upgrade: server
spec:
  concurrency: 1
  nodeSelector:
    matchExpressions:
      - { key: rke2-upgrade, operator: Exists }
      - { key: rke2-upgrade, operator: NotIn, values: ["disabled", "false"] }
      - {
          key: node-role.kubernetes.io/control-plane,
          operator: In,
          values: ["true"],
        }
  tolerations:
    - key: "CriticalAddonsOnly"
      operator: "Equal"
      value: "true"
      effect: "NoExecute"
  serviceAccountName: system-upgrade
  cordon: true
  upgrade:
    image: rancher/rke2-upgrade
  version: ${rke2_version}
EOF

    echo "👉 Upgrade Plan applied for version ${rke2_version}, waiting for upgrade..."

    # The kubeletVersion uses "+" instead of "-" in the version string
    local expected_kubelet_version="${rke2_version//-/+}"
    rke2_wait_for_version "${node_name}" "${expected_kubelet_version}"
    rke2_wait_for_node_ready "${node_name}"

    if (( step < total )); then
      echo "✅ Upgraded to intermediate version ${rke2_version}, proceeding to next step..."
    fi
  done

  # Cleanup
  echo "👉 Cleaning up upgrade resources..."
  kubectl label "${node_name}" rke2-upgrade=false --overwrite

  # Delete finalizers to prevent blocking
  kubectl patch clusterrolebinding system-upgrade \
    -p '{"metadata":{"finalizers":null}}' 2>/dev/null || true

  kubectl delete -f \
    "https://github.com/rancher/system-upgrade-controller/releases/download/${SYSTEM_UPGRADE_CONTROLLER_VERSION}/system-upgrade-controller.yaml" \
    2>/dev/null || true

  # Refresh kubeconfig and kubectl binary after upgrade
  echo "👉 Refreshing kubeconfig and tools..."
  rke2_refresh_kubeconfig_and_tools

  log_info "=== RKE2 Cluster Upgrade Complete ==="
}

rke2_refresh_kubeconfig_and_tools() {
  mkdir -p "${HOME}/.kube"
  sudo cp /etc/rancher/rke2/rke2.yaml "${HOME}/.kube/config" 2>/dev/null || true
  sudo chown -R "${USER}:${USER}" "${HOME}/.kube" 2>/dev/null || true
  chmod 600 "${HOME}/.kube/config" 2>/dev/null || true
  export KUBECONFIG="${HOME}/.kube/config"

  # Copy updated binaries
  if [[ -f /var/lib/rancher/rke2/bin/ctr ]]; then
    sudo cp /var/lib/rancher/rke2/bin/ctr /usr/local/bin/ || true
  fi
  if [[ -f /var/lib/rancher/rke2/bin/kubectl ]]; then
    sudo cp /var/lib/rancher/rke2/bin/kubectl /usr/local/bin/ || true
  fi
}

################################
# K3s Upgrade
################################

k3s_configure_registries() {
  if [[ -z "${DOCKER_USERNAME}" || -z "${DOCKER_PASSWORD}" ]]; then
    return 1
  fi

  sudo mkdir -p /etc/rancher/k3s
  sudo tee /etc/rancher/k3s/registries.yaml >/dev/null <<EOF
configs:
  "registry-1.docker.io":
    auth:
      username: "${DOCKER_USERNAME}"
      password: "${DOCKER_PASSWORD}"
EOF

  return 0
}

k3s_setup_kubeconfig() {
  local src="$1"

  mkdir -p "${HOME}/.kube"
  sudo cp "${src}" "${HOME}/.kube/config"
  sudo chown "${USER}:${USER}" "${HOME}/.kube/config"
  chmod 600 "${HOME}/.kube/config"

  export KUBECONFIG="${HOME}/.kube/config"
}

k3s_upgrade() {
  require_cmd sudo
  require_cmd curl

  if [[ "$(uname -s)" != "Linux" ]]; then
    echo "❌ K3s upgrade currently supports Linux only"
    exit 1
  fi

  log_info "=== K3s Cluster Upgrade ==="

  # Get current version
  local current_version=""
  if cmd_exists kubectl; then
    current_version="$(kubectl get nodes -o jsonpath='{.items[0].status.nodeInfo.kubeletVersion}' 2>/dev/null || echo "")"
  fi
  echo "👉 Current K3s version: ${current_version:-unknown}"
  echo "👉 Target K3s version: ${K3S_VERSION}"

  # Check if already at target
  if [[ "${current_version}" == "${K3S_VERSION}" ]]; then
    echo "✅ K3s is already at the target version ${K3S_VERSION}. No upgrade needed."
    return 0
  fi

  # Configure registries before upgrade
  local registries_written="false"
  if k3s_configure_registries; then
    registries_written="true"
  fi

  # Run the K3s installer with the new version — it handles in-place upgrade
  echo "👉 Upgrading K3s to ${K3S_VERSION}..."
  curl -sfL https://get.k3s.io | sudo INSTALL_K3S_VERSION="${K3S_VERSION}" sh -s - server \
    --write-kubeconfig-mode=0644 \
    --disable traefik \
    --disable local-storage \
    --kubelet-arg=max-pods=200

  if [[ "${registries_written}" == "true" ]]; then
    echo "👉 Restarting k3s.service to apply registry configuration..."
    sudo systemctl restart k3s.service
  fi

  # Refresh kubeconfig
  echo "👉 Refreshing kubeconfig..."
  k3s_setup_kubeconfig /etc/rancher/k3s/k3s.yaml

  wait_for_k8s_ready

  # Verify new version
  local new_version
  new_version="$(kubectl get nodes -o jsonpath='{.items[0].status.nodeInfo.kubeletVersion}' 2>/dev/null || echo "")"
  echo "✅ K3s upgraded to: ${new_version}"

  log_info "=== K3s Cluster Upgrade Complete ==="
}

################################
# KIND Upgrade
################################

kind_os() {
  uname | tr '[:upper:]' '[:lower:]'
}

kind_arch() {
  local arch
  arch="$(uname -m)"
  case "${arch}" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)
      echo "❌ Unsupported architecture for kind: ${arch}"
      exit 1
      ;;
  esac
}

get_latest_kind() {
  curl -s https://api.github.com/repos/kubernetes-sigs/kind/releases/latest \
    | grep '"tag_name"' \
    | cut -d '"' -f 4 \
    || true
}

install_kind_bin() {
  require_cmd curl
  require_cmd sudo

  local os arch version current_version=""
  os="$(kind_os)"
  arch="$(kind_arch)"

  # Detect currently installed version (e.g. "v0.31.0")
  if cmd_exists kind; then
    current_version=$(kind version 2>/dev/null | grep -oP 'v[\d.]+' | head -1 || true)
  fi

  if [[ -z "${KIND_VERSION}" ]]; then
    version="$(get_latest_kind)"
  else
    version="${KIND_VERSION}"
  fi

  # If we couldn't resolve target version (e.g. API rate limit) and kind
  # is already installed, keep the existing binary.
  if [[ -z "${version}" ]]; then
    if [[ -n "${current_version}" ]]; then
      log_info "Could not determine latest KIND version (GitHub API rate limit?). Keeping existing KIND ${current_version}."
      echo "✅ KIND ${current_version} already installed — skipping download"
      return 0
    else
      log_error "Cannot determine KIND version to install and no existing binary found."
      return 1
    fi
  fi

  # Skip download if the requested version is already installed
  if [[ "${version}" == "${current_version}" ]]; then
    echo "✅ KIND ${version} already installed — skipping download"
    return 0
  fi

  echo "👉 Installing KIND ${version}..."
  curl -Lo kind "https://kind.sigs.k8s.io/dl/${version}/kind-${os}-${arch}"
  chmod +x kind
  sudo mv kind /usr/local/bin/kind
  echo "✅ KIND ${version} installed"
}

create_kind_config() {
  local cfg_file="$1"

  cat <<EOF > "${cfg_file}"
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "127.0.0.1"
  apiServerPort: ${KIND_API_PORT}
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: KubeletConfiguration
    maxPods: 250
    serializeImagePulls: false
EOF
}

get_kind_node_k8s_version() {
  # Returns the K8s server version (e.g. v1.35.0) from the running cluster
  kubectl version --short 2>/dev/null | awk '/Server Version:/ {print $3}' || true
}

get_kind_target_k8s_version() {
  # Ask the kind binary what K8s version it would create
  # kind images list the default node image tag
  local img
  img=$(kind build node-image --help 2>&1 | grep -oP 'kindest/node:v[\d.]+' | head -1 || true)
  if [[ -n "$img" ]]; then
    echo "${img#kindest/node:}"
  else
    echo ""
  fi
}

kind_upgrade() {
  require_cmd kubectl

  log_info "=== KIND Cluster Upgrade ==="

  local kind_config
  kind_config="/tmp/kind-${KIND_CLUSTER_NAME}-${KIND_API_PORT}.yaml"
  local context="kind-${KIND_CLUSTER_NAME}"

  # ── Step 1: Install/update kind binary ──
  install_kind_bin

  # ── Step 2: Decide whether to recreate ──
  local need_recreate="false"
  local current_version="" cluster_exists="false"

  if cmd_exists kind && kind get clusters 2>/dev/null | grep -qx "${KIND_CLUSTER_NAME}"; then
    cluster_exists="true"
    # Export KIND kubeconfig so kubectl talks to the right cluster
    kind export kubeconfig --name "${KIND_CLUSTER_NAME}" 2>/dev/null || true
    current_version=$(kubectl --context "${context}" version -o json 2>/dev/null \
      | grep -oP '"gitVersion"\s*:\s*"\Kv[^"]+' | tail -1 || true)
    log_info "Existing KIND cluster found: ${KIND_CLUSTER_NAME} (K8s ${current_version:-unknown})"
  fi

  if [[ "${KIND_FORCE_RECREATE}" == "true" ]]; then
    log_info "--force-recreate specified — cluster will be deleted and recreated."
    need_recreate="true"
  elif [[ "${cluster_exists}" == "false" ]]; then
    log_info "No existing cluster found — will create a new one."
    need_recreate="true"
  else
    # Compare current K8s version with the one KIND would create.
    # KIND embeds a default K8s version per release.  If user ran
    # --kind-version to get a newer release, the default K8s version
    # may differ.  When we can't determine the target, fall back to
    # keeping the cluster (safe path).
    # kind doesn't expose the target K8s version directly; use the
    # image tag from "kind create cluster --help" or default images.
    log_info "KIND binary updated. Checking if cluster recreation is needed..."

    if [[ -n "${current_version}" ]]; then
      echo "   Current cluster K8s version: ${current_version}"
      echo "   KIND binary version:         $(kind version 2>/dev/null || echo unknown)"
      echo ""
      echo "✅ Existing cluster is intact — skipping recreation."
      echo "   The cluster and all workloads are preserved."
      echo "   Use --force-recreate if you need a fresh cluster."
      log_info "Cluster preserved (K8s ${current_version}). Skipping delete/create."

      # Ensure kubectl context is set correctly
      kind export kubeconfig --name "${KIND_CLUSTER_NAME}" 2>/dev/null || true
      kubectl config use-context "${context}" 2>/dev/null || true
      wait_for_k8s_ready "${context}"
      log_info "=== KIND Cluster Upgrade Complete (cluster preserved) ==="
      return 0
    else
      log_warn "Could not determine current K8s version — recreating cluster to be safe."
      need_recreate="true"
    fi
  fi

  # ── Step 3: Recreate cluster (only if needed) ──
  if [[ "${need_recreate}" == "true" ]]; then
    echo "⚠️  KIND clusters do not support in-place K8s upgrades."
    echo "   The cluster will be deleted and recreated."

    # Kill any stale processes holding the API port
    local stale_pid
    stale_pid=$(sudo ss -tlnp "sport = :${KIND_API_PORT}" 2>/dev/null \
      | awk '/LISTEN/ {match($0, /pid=([0-9]+)/, m); print m[1]}' | head -1 || true)
    if [[ -n "${stale_pid}" ]]; then
      log_info "Killing stale process (PID ${stale_pid}) on port ${KIND_API_PORT}..."
      sudo kill "${stale_pid}" 2>/dev/null || true
      sleep 2
    fi

    # Delete existing cluster if present
    if [[ "${cluster_exists}" == "true" ]]; then
      echo "👉 Deleting existing KIND cluster: ${KIND_CLUSTER_NAME}..."
      kind delete cluster --name "${KIND_CLUSTER_NAME}"
    fi

    # Create fresh cluster
    create_kind_config "${kind_config}"

    echo "👉 Creating KIND cluster: ${KIND_CLUSTER_NAME} (API @ 127.0.0.1:${KIND_API_PORT})"
    kind create cluster --name "${KIND_CLUSTER_NAME}" --config "${kind_config}"

    rm -f "${kind_config}"

    echo "✅ KIND cluster recreated"
    kubectl cluster-info --context "${context}"

    wait_for_k8s_ready "${context}"
  fi

  log_info "=== KIND Cluster Upgrade Complete ==="
}

################################
# Main Dispatch
################################

# Flexible argument parsing:
#   ./pre-orch-upgrade.sh                          -> auto-detect provider, action=upgrade
#   ./pre-orch-upgrade.sh upgrade                  -> auto-detect provider, action=upgrade
#   ./pre-orch-upgrade.sh rke2                     -> provider=rke2, action=upgrade
#   ./pre-orch-upgrade.sh rke2 upgrade             -> provider=rke2, action=upgrade
#   ./pre-orch-upgrade.sh --skip-os-config         -> auto-detect, options start immediately

PROVIDER=""
ACTION="upgrade"

# Check if first arg is a known provider, 'upgrade', a flag, or missing
if [[ $# -ge 1 ]]; then
  case "$1" in
    kind|k3s|rke2)
      PROVIDER="$1"
      shift
      # Check if next arg is 'upgrade' (optional, skip if flag or missing)
      if [[ $# -ge 1 && "$1" == "upgrade" ]]; then
        shift
      fi
      ;;
    upgrade)
      # No provider given, just the action
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -*)  # Flags — no provider, no action word, go straight to option parsing
      ;;
    *)
      echo "❌ Unknown argument: $1 (expected: kind, k3s, rke2, upgrade, or options)"
      usage
      exit 1
      ;;
  esac
fi

# Auto-detect provider if not explicitly specified
if [[ -z "${PROVIDER}" ]]; then
  echo "👉 No Kubernetes provider specified, auto-detecting..."
  if PROVIDER="$(detect_k8s_provider)"; then
    echo "✅ Detected Kubernetes provider: ${PROVIDER}"
  else
    echo "❌ Could not auto-detect Kubernetes provider."
    echo "   Please specify the provider explicitly: $(basename "$0") <kind|k3s|rke2> upgrade"
    exit 1
  fi
fi

log_info "Provider: ${PROVIDER} | Action: ${ACTION}"

# Parse long options
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;

    # Global
    --wait-timeout)
      WAIT_TIMEOUT_SECONDS="$2"
      shift 2
      ;;
    --wait-interval)
      WAIT_INTERVAL_SECONDS="$2"
      shift 2
      ;;
    --localpv-version)
      LOCALPV_VERSION="$2"
      shift 2
      ;;
    --helm-version)
      HELM_VERSION="$2"
      shift 2
      ;;
    --skip-os-config)
      SKIP_OS_CONFIG="true"
      shift
      ;;

    # KIND
    --cluster-name)
      KIND_CLUSTER_NAME="$2"
      shift 2
      ;;
    --api-port)
      KIND_API_PORT="$2"
      shift 2
      ;;
    --kind-version)
      KIND_VERSION="$2"
      shift 2
      ;;
    --force-recreate)
      KIND_FORCE_RECREATE="true"
      shift
      ;;

    # K3s
    --k3s-version)
      K3S_VERSION="$2"
      shift 2
      ;;

    # RKE2
    --rke2-target-version)
      RKE2_TARGET_VERSION="$2"
      shift 2
      ;;

    # Shared
    --docker-username)
      DOCKER_USERNAME="$2"
      shift 2
      ;;
    --docker-password)
      DOCKER_PASSWORD="$2"
      shift 2
      ;;

    *)
      echo "❌ Unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

# Step 1: OS Configuration (common for all providers)
if [[ "${SKIP_OS_CONFIG}" != "true" ]]; then
  upgrade_os_config
else
  log_info "Skipping OS configuration (--skip-os-config)"
fi

# Step 2: Provider-specific Kubernetes upgrade
case "${PROVIDER}" in
  kind)
    kind_upgrade
    # Step 3: OpenEBS LocalPV
    upgrade_openebs_localpv "kind-${KIND_CLUSTER_NAME}"
    ;;
  k3s)
    k3s_upgrade
    # Step 3: OpenEBS LocalPV
    upgrade_openebs_localpv
    ;;
  rke2)
    rke2_upgrade
    # Step 3: OpenEBS LocalPV
    upgrade_openebs_localpv
    ;;
  *)
    echo "❌ Unknown provider: ${PROVIDER} (expected: kind, k3s, or rke2)"
    usage
    exit 1
    ;;
esac

echo ""
log_info "========================================="
log_info "Pre-upgrade complete for provider: ${PROVIDER}"
log_info "========================================="
