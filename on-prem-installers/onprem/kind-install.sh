#!/usr/bin/env bash
set -e

CLUSTER_NAME="kind-cluster"
OS="$(uname | tr '[:upper:]' '[:lower:]')"
ARCH="amd64"
KIND_CONFIG="/tmp/kind-${CLUSTER_NAME}-6443.yaml"

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

  create_kind_config

  kind create cluster \
    --name "${CLUSTER_NAME}" \
    --config "${KIND_CONFIG}"

  echo "✅ Cluster created"
  kubectl cluster-info --context kind-${CLUSTER_NAME}
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

