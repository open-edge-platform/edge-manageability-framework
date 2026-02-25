#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# Prefer binaries installed to /usr/local/bin (e.g., avoid asdf shims).
export PATH="/usr/local/bin:${PATH}"
# shellcheck disable=SC1091
source onprem.env

################################
# Defaults / Configuration
################################

WAIT_TIMEOUT_SECONDS="${WAIT_TIMEOUT_SECONDS:-300}"
WAIT_INTERVAL_SECONDS="${WAIT_INTERVAL_SECONDS:-5}"
LOCALPV_VERSION="${LOCALPV_VERSION:-4.3.0}"

# KIND
KIND_CLUSTER_NAME_DEFAULT="kind-cluster"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-$KIND_CLUSTER_NAME_DEFAULT}"
KIND_API_PORT="${KIND_API_PORT:-6443}"
KIND_VERSION="${KIND_VERSION:-}"

# K3s
K3S_VERSION_DEFAULT="v1.34.3+k3s1"
K3S_VERSION="${K3S_VERSION:-$K3S_VERSION_DEFAULT}"

# RKE2
RKE2_VERSION_DEFAULT="v1.34.3+rke2r1"
RKE2_VERSION="${RKE2_VERSION:-$RKE2_VERSION_DEFAULT}"
DOCKER_USERNAME="${DOCKER_USERNAME:-}"
DOCKER_PASSWORD="${DOCKER_PASSWORD:-}"

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
  trap 'rm -rf "$tmp"' RETURN

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

usage() {
  cat >&2 <<EOF
Usage:
  $(basename "$0") <kind|k3s|rke2> <install|uninstall> [options]

Global options:
  --wait-timeout <seconds>     Default: ${WAIT_TIMEOUT_SECONDS}
  --wait-interval <seconds>    Default: ${WAIT_INTERVAL_SECONDS}
  --localpv-version <version>  Default: ${LOCALPV_VERSION}

KIND options:
  --cluster-name <name>        Default: ${KIND_CLUSTER_NAME_DEFAULT}
  --api-port <port>            Default: ${KIND_API_PORT}
  --kind-version <version>     Default: latest

K3s options:
  --k3s-version <version>      Default: ${K3S_VERSION_DEFAULT}

RKE2 options:
  --rke2-version <version>     Default: ${RKE2_VERSION_DEFAULT}
  --docker-username <user>     Optional (for Docker Hub auth)
  --docker-password <pass>     Optional (for Docker Hub auth)

Examples:
  $(basename "$0") kind install
  $(basename "$0") kind uninstall
  $(basename "$0") k3s install --k3s-version v1.34.3+k3s1
  $(basename "$0") rke2 install --rke2-version v1.34.3+rke2r1
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

  echo "👉 Waiting for all nodes to be Ready (timeout: ${WAIT_TIMEOUT_SECONDS}s)..."
  if ! kubectl "${kubectl_ctx_args[@]}" wait --for=condition=Ready node --all --timeout="${WAIT_TIMEOUT_SECONDS}s"; then
    echo "❌ Timed out waiting for nodes to become Ready"
    kubectl "${kubectl_ctx_args[@]}" get nodes -o wide || true
    kubectl "${kubectl_ctx_args[@]}" get pods -A || true
    exit 1
  fi
  echo "✅ All nodes are Ready"
}

install_openebs_localpv() {
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

  echo "🚀 Installing/Upgrading OpenEBS LocalPV..."
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
    maxPods: 250
    serializeImagePulls: false
EOF
}

kind_install() {
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
    install_openebs_localpv "${context}"
    return 0
  fi

  create_kind_config "${kind_config}"

  kind create cluster --name "${KIND_CLUSTER_NAME}" --config "${kind_config}"

  echo "✅ Cluster created"
  kubectl cluster-info --context "${context}"

  wait_for_k8s_ready "${context}"
  install_openebs_localpv "${context}"
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

k3s_install() {
  require_cmd sudo
  require_cmd curl

  if [[ "$(uname -s)" != "Linux" ]]; then
    echo "❌ K3s install currently supports Linux only"
    exit 1
  fi

  if systemctl is-active --quiet k3s.service 2>/dev/null; then
    echo "⚠️  k3s.service is already active; reusing it"
  else
    echo "👉 Installing K3s (${K3S_VERSION})..."
    curl -sfL https://get.k3s.io | sudo INSTALL_K3S_VERSION="${K3S_VERSION}" sh -s - server \
      --write-kubeconfig-mode=0644 \
      --disable traefik \
      --disable local-storage \
      --kubelet-arg=max-pods=200
  fi

  echo "👉 Setting up kubeconfig (~/.kube/config)..."
  k3s_setup_kubeconfig /etc/rancher/k3s/k3s.yaml

  wait_for_k8s_ready
  install_openebs_localpv
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
  - "max-pods=200"
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
      - /var/openebs/local:/var/openebs/local
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
  install_openebs_localpv
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

if [[ $# -lt 2 ]]; then
  usage
  exit 1
fi

PROVIDER="$1"
ACTION="$2"
shift 2

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
    --localpv-version)
      LOCALPV_VERSION="$2"
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

    *)
      echo "❌ Unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

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
