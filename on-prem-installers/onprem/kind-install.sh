#!/usr/bin/env bash
set -e

CLUSTER_NAME="kind-cluster"
OS="$(uname | tr '[:upper:]' '[:lower:]')"
ARCH="amd64"
KIND_CONFIG="/tmp/kind-${CLUSTER_NAME}-6443.yaml"
WAIT_TIMEOUT_SECONDS="${WAIT_TIMEOUT_SECONDS:-300}"
WAIT_INTERVAL_SECONDS="${WAIT_INTERVAL_SECONDS:-5}"

# Prefer binaries installed to /usr/local/bin (e.g., avoid asdf shims).
export PATH="/usr/local/bin:${PATH}"

wait_for_kind_ready() {
  local context="kind-${CLUSTER_NAME}"

  echo "👉 Waiting for Kubernetes API to be reachable (context: ${context})..."
  local deadline=$((SECONDS + WAIT_TIMEOUT_SECONDS))
  until kubectl --context "${context}" get --raw='/readyz' >/dev/null 2>&1; do
    if (( SECONDS >= deadline )); then
      echo "❌ Timed out waiting for API server to be ready after ${WAIT_TIMEOUT_SECONDS}s"
      kubectl --context "${context}" cluster-info || true
      exit 1
    fi
    sleep "${WAIT_INTERVAL_SECONDS}"
  done
  echo "✅ API server is ready"

  echo "👉 Waiting for all nodes to be Ready (timeout: ${WAIT_TIMEOUT_SECONDS}s)..."
  if ! kubectl --context "${context}" wait --for=condition=Ready node --all --timeout="${WAIT_TIMEOUT_SECONDS}s"; then
    echo "❌ Timed out waiting for nodes to become Ready"
    kubectl --context "${context}" get nodes -o wide || true
    kubectl --context "${context}" get pods -A || true
    exit 1
  fi
  echo "✅ All nodes are Ready"
}

install_openebs_localpv() {
  local context="kind-${CLUSTER_NAME}"

  if ! command -v helm >/dev/null 2>&1; then
    echo "❌ helm not found. Install helm v3 and retry."
    exit 1
  fi

  # Hardcoded version (change it here when needed)
  local LOCALPV_VERSION="4.3.0"

  echo "👉 Using OpenEBS LocalPV version: ${LOCALPV_VERSION}"

  echo "👉 Adding OpenEBS LocalPV Helm repo..."
  helm repo add openebs-localpv https://openebs.github.io/dynamic-localpv-provisioner >/dev/null

  echo "🔄 Updating Helm repos..."
  helm repo update >/dev/null

  echo "🚀 Installing/Upgrading OpenEBS LocalPV..."
  helm upgrade --install openebs-localpv openebs-localpv/localpv-provisioner \
    --kube-context "${context}" \
    --version "${LOCALPV_VERSION}" \
    --namespace openebs-system --create-namespace \
    --set hostpathClass.enabled=true \
    --set hostpathClass.name=openebs-hostpath \
    --set hostpathClass.isDefaultClass=true \
    --set deviceClass.enabled=false \
    --wait --timeout 10m0s

  echo "📦 OpenEBS Pods in openebs-system namespace:"
  kubectl --context "${context}" get pods -n openebs-system
}

get_latest_kind() {
  curl -s https://api.github.com/repos/kubernetes-sigs/kind/releases/latest \
    | grep '"tag_name"' \
    | cut -d '"' -f 4
}

install_kind() {
  KIND_VERSION=$(get_latest_kind)
  echo "👉 Installing KIND ${KIND_VERSION}..."

  curl -Lo kind https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${OS}-${ARCH}
  chmod +x kind
  sudo mv kind /usr/local/bin/kind

  echo "✅ KIND ${KIND_VERSION} installed"
}

create_kind_config() {
  cat <<EOF > "${KIND_CONFIG}"
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "127.0.0.1"
  apiServerPort: 6443
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: KubeletConfiguration
    maxPods: 250
    serializeImagePulls: false
EOF
}

create_cluster() {
  echo "👉 Creating KIND cluster: ${CLUSTER_NAME} (API @ 127.0.0.1:6443)"

  if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
    echo "⚠️  KIND cluster '${CLUSTER_NAME}' already exists; reusing it"
    kind export kubeconfig --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
    kubectl cluster-info --context kind-${CLUSTER_NAME} || true
    wait_for_kind_ready
    install_openebs_localpv
    return 0
  fi

  create_kind_config

  kind create cluster \
    --name "${CLUSTER_NAME}" \
    --config "${KIND_CONFIG}"

  echo "✅ Cluster created"
  kubectl cluster-info --context kind-${CLUSTER_NAME}

  wait_for_kind_ready

  install_openebs_localpv
}

delete_cluster() {
  echo "👉 Deleting KIND cluster: ${CLUSTER_NAME}"
  kind delete cluster --name "${CLUSTER_NAME}" || true
  rm -f "${KIND_CONFIG}"
  echo "✅ Cluster deleted"
}

uninstall_kind() {
  echo "👉 Uninstalling KIND..."
  sudo rm -f /usr/local/bin/kind
  echo "✅ KIND removed"
}

usage() {
  echo "Usage: $0 {install|uninstall}"
}

case "$1" in
  install)
    install_kind
    create_cluster
    ;;
  uninstall)
    delete_cluster
    uninstall_kind
    ;;
  *)
    usage
    exit 1
    ;;
esac

