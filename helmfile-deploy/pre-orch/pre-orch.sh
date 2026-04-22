#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# Prefer binaries installed to /usr/local/bin (e.g., avoid asdf shims).
export PATH="/usr/local/bin:${PATH}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ─── Timing helpers ──────────────────────────────────────────────────────────
SCRIPT_START_TIME=$SECONDS
SCRIPT_START_TS=$(date '+%Y-%m-%d %H:%M:%S')
declare -a STEP_NAMES=()
declare -a STEP_DURATIONS=()

step_start() {
  _STEP_NAME="$1"
  _STEP_START=$SECONDS
  echo ""
  echo "⏱️  [$_STEP_NAME] started at $(date '+%H:%M:%S')"
}

step_done() {
  local dur=$(( SECONDS - _STEP_START ))
  STEP_NAMES+=("$_STEP_NAME")
  STEP_DURATIONS+=($dur)
  echo "⏱️  [$_STEP_NAME] completed in $(( dur / 60 ))m $(( dur % 60 ))s"
}

print_timing_summary() {
  local total=$(( SECONDS - SCRIPT_START_TIME ))
  local end_ts
  end_ts=$(date '+%Y-%m-%d %H:%M:%S')
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  PRE-ORCH TIMING SUMMARY"
  echo "═══════════════════════════════════════════════════════════════"
  for i in "${!STEP_NAMES[@]}"; do
    local d=${STEP_DURATIONS[$i]}
    printf "  %-35s %dm %ds\n" "${STEP_NAMES[$i]}" $(( d / 60 )) $(( d % 60 ))
  done
  echo "  ─────────────────────────────────────────────────────────────"
  echo "  Start: $SCRIPT_START_TS"
  echo "  End:   $end_ts"
  printf "  Total: %dm %ds\n" $(( total / 60 )) $(( total % 60 ))
  echo "═══════════════════════════════════════════════════════════════"
}

trap print_timing_summary EXIT

# Source config file if present
if [[ -f "${SCRIPT_DIR}/pre-orch.env" ]]; then
  # shellcheck disable=SC1091
  set -a; source "${SCRIPT_DIR}/pre-orch.env"; set +a
fi

# ─── Logging: tee all output to timestamped log file ────────────────────────
LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"
LOG_TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
LOG_FILE="$LOG_DIR/pre-orch_${LOG_TIMESTAMP}.log"
exec > >(tee -a "$LOG_FILE") 2>&1
echo "═══ Log started: $(date -Iseconds) ═══"
echo "═══ Command: $0 $* ═══"
echo ""

################################
# Defaults / Configuration
################################

WAIT_TIMEOUT_SECONDS="${WAIT_TIMEOUT_SECONDS:-300}"
WAIT_INTERVAL_SECONDS="${WAIT_INTERVAL_SECONDS:-5}"

# KIND
KIND_CLUSTER_NAME_DEFAULT="kind-cluster"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-$KIND_CLUSTER_NAME_DEFAULT}"
KIND_API_PORT="${KIND_API_PORT:-6443}"
KIND_VERSION="${KIND_VERSION:-}"

# K3s
K3S_VERSION_DEFAULT="v1.34.3+k3s1"
K3S_VERSION="${K3S_VERSION:-$K3S_VERSION_DEFAULT}"

# RKE2
RKE2_VERSION_DEFAULT="v1.34.4+rke2r1"
RKE2_VERSION="${RKE2_VERSION:-$RKE2_VERSION_DEFAULT}"
DOCKER_USERNAME="${DOCKER_USERNAME:-}"
DOCKER_PASSWORD="${DOCKER_PASSWORD:-}"

# Kubelet
MAX_PODS="${MAX_PODS:-500}"

# Pre-orch components (helmfile-based)
INSTALL_OPENEBS="${INSTALL_OPENEBS:-true}"
INSTALL_METALLB="${INSTALL_METALLB:-true}"
INSTALL_PRE_CONFIG="${INSTALL_PRE_CONFIG:-true}"

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

install_helm() {
  if cmd_exists helm; then
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

  # Prefer system install if possible, otherwise fall back to a user install.
  if [[ -w /usr/local/bin ]]; then
    if [[ -n "${HELM_VERSION:-}" ]]; then
      HELM_INSTALL_DIR="/usr/local/bin" DESIRED_VERSION="${HELM_VERSION}" "$installer_path"
    else
      HELM_INSTALL_DIR="/usr/local/bin" "$installer_path"
    fi
  elif cmd_exists sudo; then
    if [[ -n "${HELM_VERSION:-}" ]]; then
      sudo -E env DESIRED_VERSION="${HELM_VERSION}" "$installer_path"
    else
      sudo -E "$installer_path"
    fi
  else
    mkdir -p "${HOME}/.local/bin"
    if [[ -n "${HELM_VERSION:-}" ]]; then
      HELM_INSTALL_DIR="${HOME}/.local/bin" DESIRED_VERSION="${HELM_VERSION}" "$installer_path"
    else
      HELM_INSTALL_DIR="${HOME}/.local/bin" "$installer_path"
    fi
  fi

  if ! cmd_exists helm; then
    echo "❌ helm installation did not succeed; please install helm manually and retry."
    exit 1
  fi
}

install_kubectl() {
  if cmd_exists kubectl; then
    return 0
  fi

  require_cmd curl

  echo "👉 kubectl not found; installing latest stable kubectl..."

  local version
  version="$(curl -fsSL https://dl.k8s.io/release/stable.txt)"

  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "${arch}" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
  esac

  curl -fsSLo /tmp/kubectl "https://dl.k8s.io/release/${version}/bin/${os}/${arch}/kubectl"
  chmod +x /tmp/kubectl

  if [[ -w /usr/local/bin ]]; then
    mv /tmp/kubectl /usr/local/bin/kubectl
  elif cmd_exists sudo; then
    sudo mv /tmp/kubectl /usr/local/bin/kubectl
  else
    mkdir -p "${HOME}/.local/bin"
    mv /tmp/kubectl "${HOME}/.local/bin/kubectl"
  fi

  if ! cmd_exists kubectl; then
    echo "❌ kubectl installation did not succeed; please install kubectl manually and retry."
    exit 1
  fi
  echo "✅ kubectl ${version} installed"
}

install_helmfile() {
  if cmd_exists helmfile; then
    return 0
  fi

  require_cmd curl

  echo "👉 helmfile not found; installing latest helmfile..."

  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "${arch}" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
  esac

  local version
  version="$(curl -fsSL https://api.github.com/repos/helmfile/helmfile/releases/latest | grep '"tag_name"' | cut -d '"' -f 4)"

  local tarball="helmfile_${version#v}_${os}_${arch}.tar.gz"
  local url="https://github.com/helmfile/helmfile/releases/download/${version}/${tarball}"

  local tmp
  tmp="$(mktemp -d)"

  curl -fsSL "${url}" -o "${tmp}/${tarball}"
  tar -xzf "${tmp}/${tarball}" -C "${tmp}" helmfile

  if [[ -w /usr/local/bin ]]; then
    mv "${tmp}/helmfile" /usr/local/bin/helmfile
  elif cmd_exists sudo; then
    sudo mv "${tmp}/helmfile" /usr/local/bin/helmfile
  else
    mkdir -p "${HOME}/.local/bin"
    mv "${tmp}/helmfile" "${HOME}/.local/bin/helmfile"
  fi
  chmod +x "$(command -v helmfile)"
  rm -rf "${tmp}"

  if ! cmd_exists helmfile; then
    echo "❌ helmfile installation did not succeed; please install helmfile manually and retry."
    exit 1
  fi
  echo "✅ helmfile ${version} installed"
}

install_dependencies() {
  echo "🔍 Checking dependencies..."
  install_kubectl
  install_helm
  install_helmfile
  echo "✅ All dependencies ready"
}

usage() {
  cat >&2 <<EOF
Usage:
  $(basename "$0") [kind|k3s|rke2] [install|uninstall|upgrade] [options]

  Provider and action can also be set via PROVIDER and ACTION in pre-orch.env.
  CLI arguments override config file values.

Global options:
  --wait-timeout <seconds>     Default: ${WAIT_TIMEOUT_SECONDS}
  --wait-interval <seconds>    Default: ${WAIT_INTERVAL_SECONDS}
  --no-openebs                 Skip OpenEBS LocalPV install
  --no-metallb                 Skip MetalLB install

KIND options:
  --cluster-name <name>        Default: ${KIND_CLUSTER_NAME_DEFAULT}
  --api-port <port>            Default: ${KIND_API_PORT}
  --kind-version <version>     Default: latest

K3s options:
  --k3s-version <version>      Default: ${K3S_VERSION_DEFAULT}
  --docker-username <user>     Optional (for Docker Hub auth)
  --docker-password <pass>     Optional (for Docker Hub auth)

RKE2 options:
  --rke2-version <version>     Default: ${RKE2_VERSION_DEFAULT}
  --docker-username <user>     Optional (for Docker Hub auth)
  --docker-password <pass>     Optional (for Docker Hub auth)

Examples:
  $(basename "$0")                                     # Uses PROVIDER from pre-orch.env
  $(basename "$0") install                             # Uses PROVIDER from pre-orch.env
  $(basename "$0") k3s install
  $(basename "$0") kind install
  $(basename "$0") k3s upgrade                          # Upgrade pre-orch components
  $(basename "$0") k3s install --no-metallb
  $(basename "$0") k3s install --no-openebs --no-metallb
  $(basename "$0") rke2 install --docker-username user --docker-password pass
EOF
}

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

  echo "👉 Waiting for at least one node to register (timeout: ${WAIT_TIMEOUT_SECONDS}s)..."
  deadline=$((SECONDS + WAIT_TIMEOUT_SECONDS))
  until [[ $(kubectl "${kubectl_ctx_args[@]}" get nodes --no-headers 2>/dev/null | wc -l) -gt 0 ]]; do
    if (( SECONDS >= deadline )); then
      echo "❌ Timed out waiting for node to register after ${WAIT_TIMEOUT_SECONDS}s"
      exit 1
    fi
    sleep "${WAIT_INTERVAL_SECONDS}"
  done
  echo "✅ Node registered"

  echo "👉 Waiting for all nodes to be Ready (timeout: ${WAIT_TIMEOUT_SECONDS}s)..."
  if ! kubectl "${kubectl_ctx_args[@]}" wait --for=condition=Ready node --all --timeout="${WAIT_TIMEOUT_SECONDS}s"; then
    echo "❌ Timed out waiting for nodes to become Ready"
    kubectl "${kubectl_ctx_args[@]}" get nodes -o wide || true
    kubectl "${kubectl_ctx_args[@]}" get pods -A || true
    exit 1
  fi
  echo "✅ All nodes are Ready"
}

install_pre_orch_components() {
  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

  echo "👉 Waiting for all system pods to be ready (timeout: ${WAIT_TIMEOUT_SECONDS}s)..."
  local deadline=$((SECONDS + WAIT_TIMEOUT_SECONDS))
  while true; do
    # Count non-completed pods that are not fully ready (READY x/y where x!=y, or status != Running)
    local not_ready
    not_ready=$(kubectl get pods -A --no-headers 2>/dev/null \
      | awk '$4 != "Completed" && $4 != "Succeeded" {
               split($3, a, "/");
               if (a[1] != a[2] || $4 != "Running") count++
             }
             END { print count+0 }')
    if [[ "${not_ready}" -eq 0 ]]; then
      break
    fi
    if (( SECONDS >= deadline )); then
      echo "❌ Timed out waiting for system pods to be ready (${not_ready} pod(s) not ready)"
      kubectl get pods -A || true
      exit 1
    fi
    sleep "${WAIT_INTERVAL_SECONDS}"
  done
  echo "✅ All system pods are ready"

  # ─── Install OpenEBS and MetalLB in parallel via helmfile ────────────────
  if [[ "${INSTALL_OPENEBS}" == "true" || "${INSTALL_METALLB}" == "true" ]]; then
    step_start "OpenEBS + MetalLB"

    if [[ "${INSTALL_METALLB}" == "true" ]]; then
      # Validate IP configuration
      if [[ -n "${EMF_ORCH_IP:-}" ]]; then
        echo "✅ Single-IP mode: all services will share ${EMF_ORCH_IP}"
        echo "   Traefik port: 443, HAProxy port: 9443"
        export EMF_TRAEFIK_IP="${EMF_ORCH_IP}"
        export EMF_HAPROXY_IP="${EMF_ORCH_IP}"
      elif [[ -z "${EMF_TRAEFIK_IP:-}" || -z "${EMF_HAPROXY_IP:-}" ]]; then
        echo "❌ Either EMF_ORCH_IP (single-IP) or both EMF_TRAEFIK_IP and EMF_HAPROXY_IP must be set"
        exit 1
      fi
    fi

    echo "🚀 Installing via helmfile (OpenEBS=${INSTALL_OPENEBS}, MetalLB=${INSTALL_METALLB})..."
    (cd "${script_dir}" && helmfile -f helmfile.yaml.gotmpl apply --skip-diff-on-install --concurrency 3 2>&1) || {
      echo "❌ helmfile apply failed"
      exit 1
    }
    echo "✅ Pre-orch components installed"
    step_done
  else
    echo "⏭️  Skipping OpenEBS and MetalLB (both disabled)"
  fi

  if [[ "${INSTALL_PRE_CONFIG}" == "true" ]]; then
    local config_action="install"
    if [[ "${ACTION:-install}" == "upgrade" ]]; then
      config_action="upgrade"
    fi
    step_start "pre-orch-config"
    echo "🚀 Running pre-orch-config ($config_action)..."
    "${script_dir}/pre-orch-config.sh" "$config_action" || {
      echo "❌ pre-orch-config $config_action failed"
      exit 1
    }
    step_done
  else
    echo "⏭️  Skipping pre-orch-config (--no-pre-config)"
  fi
}

################################
# KIND
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
    | cut -d '"' -f 4
}

install_kind_bin() {
  require_cmd curl
  require_cmd sudo

  local os arch version
  os="$(kind_os)"
  arch="$(kind_arch)"

  if [[ -z "${KIND_VERSION}" ]]; then
    version="$(get_latest_kind)"
  else
    version="${KIND_VERSION}"
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
    maxPods: ${MAX_PODS}
    serializeImagePulls: false
EOF
}

kind_install() {
  install_dependencies
  require_cmd kubectl

  local kind_config
  kind_config="/tmp/kind-${KIND_CLUSTER_NAME}-${KIND_API_PORT}.yaml"
  local context="kind-${KIND_CLUSTER_NAME}"

  install_kind_bin

  echo "👉 Creating KIND cluster: ${KIND_CLUSTER_NAME} (API @ 127.0.0.1:${KIND_API_PORT})"

  if kind get clusters 2>/dev/null | grep -qx "${KIND_CLUSTER_NAME}"; then
    echo "⚠️  KIND cluster '${KIND_CLUSTER_NAME}' already exists; reusing it"
    kind export kubeconfig --name "${KIND_CLUSTER_NAME}" >/dev/null 2>&1 || true
    kubectl cluster-info --context "${context}" || true
    wait_for_k8s_ready "${context}"
    install_pre_orch_components
    return 0
  fi

  create_kind_config "${kind_config}"

  step_start "KIND cluster create"
  kind create cluster --name "${KIND_CLUSTER_NAME}" --config "${kind_config}"

  echo "✅ Cluster created"
  kubectl cluster-info --context "${context}"

  wait_for_k8s_ready "${context}"
  step_done
  install_pre_orch_components
}

kind_uninstall() {
  require_cmd sudo

  echo "👉 Deleting KIND cluster: ${KIND_CLUSTER_NAME}"
  kind delete cluster --name "${KIND_CLUSTER_NAME}" || true
  rm -f "/tmp/kind-${KIND_CLUSTER_NAME}-${KIND_API_PORT}.yaml" || true

  echo "👉 Uninstalling KIND binary"
  sudo rm -f /usr/local/bin/kind

  echo "✅ KIND removed"
}

################################
# K3s
################################

k3s_setup_kubeconfig() {
  local src="$1"

  mkdir -p "${HOME}/.kube"
  sudo cp "${src}" "${HOME}/.kube/config"
  sudo chown "${USER}:${USER}" "${HOME}/.kube/config"
  chmod 600 "${HOME}/.kube/config"

  export KUBECONFIG="${HOME}/.kube/config"
}

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

k3s_install() {
  install_dependencies
  require_cmd sudo
  require_cmd curl

  if [[ "$(uname -s)" != "Linux" ]]; then
    echo "❌ K3s install currently supports Linux only"
    exit 1
  fi

  local registries_written="false"
  if k3s_configure_registries; then
    registries_written="true"
  fi

  if systemctl is-active --quiet k3s.service 2>/dev/null; then
    echo "⚠️  k3s.service is already active; reusing it"
  else
    step_start "K3s install"
    echo "👉 Installing K3s (${K3S_VERSION})..."
    curl -sfL https://get.k3s.io | sudo INSTALL_K3S_VERSION="${K3S_VERSION}" sh -s - server \
      --write-kubeconfig-mode=0644 \
      --disable traefik \
      --disable local-storage \
      --disable servicelb \
      --kubelet-arg=max-pods=${MAX_PODS}
  fi

  if [[ "${registries_written}" == "true" ]]; then
    echo "👉 Restarting k3s.service to apply /etc/rancher/k3s/registries.yaml..."
    sudo systemctl restart k3s.service
  fi

  echo "👉 Setting up kubeconfig (~/.kube/config)..."
  k3s_setup_kubeconfig /etc/rancher/k3s/k3s.yaml

  wait_for_k8s_ready
  [[ -n "${_STEP_START:-}" ]] && step_done
  install_pre_orch_components
}

k3s_uninstall() {
  require_cmd sudo

  echo "👉 Uninstalling K3s..."

  if [[ -x /usr/local/bin/k3s-uninstall.sh ]]; then
    sudo /usr/local/bin/k3s-uninstall.sh
  else
    echo "⚠️  /usr/local/bin/k3s-uninstall.sh not found; doing best-effort cleanup"
    sudo systemctl disable --now k3s.service || true
    sudo rm -rf /var/lib/rancher/k3s /etc/rancher/k3s || true
  fi

  echo "✅ K3s uninstall complete"
  echo "Note: kubeconfig at ${HOME}/.kube/config was not removed."
}

################################
# RKE2
################################

rke2_write_audit_policy() {
  sudo mkdir -p /etc/rancher/rke2

  sudo tee /etc/rancher/rke2/audit-policy.yaml >/dev/null <<'EOF'
apiVersion: audit.k8s.io/v1
kind: Policy
omitStages:
  - "RequestReceived"
rules:
  - level: RequestResponse
    resources:
    - group: ""
      resources: ["pods"]
  - level: Metadata
    resources:
    - group: ""
      resources: ["pods/log", "pods/status"]
  - level: None
    resources:
    - group: ""
      resources: ["configmaps"]
      resourceNames: ["controller-leader"]
  - level: None
    users: ["system:kube-proxy"]
    verbs: ["watch"]
    resources:
    - group: ""
      resources: ["endpoints", "services"]
  - level: None
    userGroups: ["system:authenticated"]
    nonResourceURLs:
    - "/api*"
    - "/version"
  - level: Request
    resources:
    - group: ""
      resources: ["configmaps"]
    namespaces: ["kube-system"]
  - level: Metadata
    resources:
    - group: ""
      resources: ["secrets", "configmaps"]
  - level: Request
    resources:
    - group: ""
    - group: "extensions"
  - level: Metadata
    omitStages:
      - "RequestReceived"
EOF
}

rke2_write_config() {
  sudo mkdir -p /etc/rancher/rke2

  sudo tee /etc/rancher/rke2/config.yaml >/dev/null <<'EOF'
write-kubeconfig-mode: "0644"
audit-policy-file: "/etc/rancher/rke2/audit-policy.yaml"
cni:
  - calico
disable:
  - rke2-canal
  - rke2-ingress-nginx
  - rke2-snapshot-controller
  - rke2-snapshot-validation-webhook
disable-cloud-controller: true
kubelet-arg:
  - "max-pods=${MAX_PODS}"
etcd-arg:
  - --debug=false
  - --log-package-levels=INFO
  - --config-file=/var/lib/rancher/rke2/server/db/etcd/config
  - --quota-backend-bytes=8589934592
  - --auto-compaction-mode=periodic
  - --auto-compaction-retention=1h
services:
  kubelet:
    extra_binds:
EOF
}

rke2_write_coredns_chart_config() {
  sudo mkdir -p /var/lib/rancher/rke2/server/manifests/

  sudo tee /var/lib/rancher/rke2/server/manifests/rke2-coredns-config.yaml >/dev/null <<'EOF'
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-coredns
  namespace: kube-system
spec:
  valuesContent: |-
    global:
      clusterCIDR: 10.42.0.0/16
      clusterCIDRv4: 10.42.0.0/16
      clusterDNS: 10.43.0.10
      rke2DataDir: /var/lib/rancher/rke2
      serviceCIDR: 10.43.0.0/16
    service:
      name: "kube-dns"
EOF
}

rke2_configure_proxy_env() {
  # Best-effort: if proxy env vars exist, configure systemd env file
  if [[ -n "${http_proxy:-}" || -n "${https_proxy:-}" || -n "${no_proxy:-}" ]]; then
    sudo tee /etc/default/rke2-server >/dev/null <<EOF
HTTP_PROXY=${http_proxy:-}
HTTPS_PROXY=${https_proxy:-}
NO_PROXY=${no_proxy:-}
EOF
  fi
}

rke2_setup_kubeconfig_and_tools() {
  mkdir -p "${HOME}/.kube"
  sudo cp /etc/rancher/rke2/rke2.yaml "${HOME}/.kube/config"
  sudo chown -R "${USER}:${USER}" "${HOME}/.kube"
  chmod 600 "${HOME}/.kube/config"
  export KUBECONFIG="${HOME}/.kube/config"

  # Copy binaries for post install operations
  if [[ -f /var/lib/rancher/rke2/bin/ctr ]]; then
    sudo cp /var/lib/rancher/rke2/bin/ctr /usr/local/bin/ || true
  fi
  if [[ -f /var/lib/rancher/rke2/bin/kubectl ]]; then
    sudo cp /var/lib/rancher/rke2/bin/kubectl /usr/local/bin/ || true
  fi
}

rke2_configure_registries() {
  if [[ -n "${DOCKER_USERNAME}" && -n "${DOCKER_PASSWORD}" ]]; then
    sudo mkdir -p /etc/rancher/rke2
    sudo tee /etc/rancher/rke2/registries.yaml >/dev/null <<EOF
configs:
  "registry-1.docker.io":
    auth:
      username: "${DOCKER_USERNAME}"
      password: "${DOCKER_PASSWORD}"
EOF
  fi
}

rke2_install() {
  install_dependencies
  require_cmd sudo
  require_cmd curl

  if [[ "$(uname -s)" != "Linux" ]]; then
    echo "❌ RKE2 install currently supports Linux only"
    exit 1
  fi

  if systemctl is-active --quiet rke2-server.service 2>/dev/null; then
    echo "⚠️  rke2-server.service is already active; reusing it"
  else
    echo "👉 Installing RKE2 (${RKE2_VERSION})..."
    curl -sfL https://get.rke2.io | sudo INSTALL_RKE2_VERSION="${RKE2_VERSION}" sh -
  fi

  rke2_configure_proxy_env
  rke2_write_audit_policy
  rke2_write_config
  rke2_write_coredns_chart_config
  rke2_configure_registries

  step_start "RKE2 install"
  echo "👉 Enabling and starting rke2-server.service..."
  sudo systemctl enable --now rke2-server.service

  sleep 5
  if ! systemctl is-active --quiet rke2-server.service; then
    echo "❌ rke2-server.service is not active"
    sudo systemctl status rke2-server.service || true
    exit 1
  fi

  rke2_setup_kubeconfig_and_tools

  echo "👉 Restarting rke2-server.service to apply config changes..."
  sudo systemctl restart rke2-server.service

  wait_for_k8s_ready
  step_done
  install_pre_orch_components
}

rke2_uninstall() {
  require_cmd sudo

  echo "👉 Uninstalling RKE2..."

  if [[ -x /usr/local/bin/rke2-uninstall.sh ]]; then
    sudo /usr/local/bin/rke2-uninstall.sh
  else
    echo "⚠️  /usr/local/bin/rke2-uninstall.sh not found; doing best-effort cleanup"
    sudo systemctl disable --now rke2-server.service || true
    sudo rm -rf /var/lib/rancher/rke2 /etc/rancher/rke2 || true
  fi

  echo "✅ RKE2 uninstall complete"
  echo "Note: kubeconfig at ${HOME}/.kube/config was not removed."
}

################################
# Argument parsing / Dispatch
################################

if [[ $# -lt 1 ]]; then
  echo "❌ Action required: install, uninstall, or upgrade"
  usage
  exit 1
fi

ACTION="${1}"
shift

if [[ "${ACTION}" != "install" && "${ACTION}" != "uninstall" && "${ACTION}" != "upgrade" ]]; then
  # First arg might be provider, second must be action
  PROVIDER="${ACTION}"
  if [[ $# -lt 1 ]]; then
    echo "❌ Action required: install, uninstall, or upgrade"
    usage
    exit 1
  fi
  ACTION="${1}"
  shift
fi

if [[ "${ACTION}" != "install" && "${ACTION}" != "uninstall" && "${ACTION}" != "upgrade" ]]; then
  echo "❌ Invalid action: ${ACTION}. Must be 'install', 'uninstall', or 'upgrade'"
  usage
  exit 1
fi

if [[ "${ACTION}" != "upgrade" && -z "${PROVIDER:-}" ]]; then
  echo "❌ Provider required: set PROVIDER in pre-orch.env or pass as argument"
  usage
  exit 1
fi

# Manual parsing for long options
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;

    --wait-timeout)
      WAIT_TIMEOUT_SECONDS="$2"
      shift 2
      ;;
    --wait-interval)
      WAIT_INTERVAL_SECONDS="$2"
      shift 2
      ;;
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

    --k3s-version)
      K3S_VERSION="$2"
      shift 2
      ;;

    --rke2-version)
      RKE2_VERSION="$2"
      shift 2
      ;;
    --docker-username)
      DOCKER_USERNAME="$2"
      shift 2
      ;;
    --docker-password)
      DOCKER_PASSWORD="$2"
      shift 2
      ;;

    --no-openebs)
      INSTALL_OPENEBS="false"
      shift
      ;;
    --no-metallb)
      INSTALL_METALLB="false"
      shift
      ;;
    --no-pre-config)
      INSTALL_PRE_CONFIG="false"
      shift
      ;;

    *)
      echo "❌ Unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

# For upgrade: skip cluster creation, just re-apply pre-orch components (idempotent)
if [[ "${ACTION}" == "upgrade" ]]; then
  echo "🔄 Upgrade mode: re-applying pre-orch components..."
  install_dependencies
  wait_for_k8s_ready
  install_pre_orch_components
else
  case "${PROVIDER}" in
    kind)
      case "${ACTION}" in
        install) kind_install ;;
        uninstall) kind_uninstall ;;
        *) usage; exit 1 ;;
      esac
      ;;
    k3s)
      case "${ACTION}" in
        install) k3s_install ;;
        uninstall) k3s_uninstall ;;
        *) usage; exit 1 ;;
      esac
      ;;
    rke2)
      case "${ACTION}" in
        install) rke2_install ;;
        uninstall) rke2_uninstall ;;
        *) usage; exit 1 ;;
      esac
      ;;
    *)
      echo "❌ Unknown provider: ${PROVIDER}"
      usage
      exit 1
      ;;
  esac
fi
