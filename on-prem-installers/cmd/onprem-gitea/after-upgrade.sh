#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

cat << "EOF"

   ____ _ _               _   _                           _
  / ___(_) |_ ___  __ _  | | | |_ __   __ _ _ __ __ _  __| | ___
 | |  _| | __/ _ \/ _` | | | | | '_ \ / _` | '__/ _` |/ _` |/ _ \
 | |_| | | ||  __/ (_| | | |_| | |_) | (_| | | | (_| | (_| |  __/
  \____|_|\__\___|\__,_|  \___/| .__/ \__, |_|  \__,_|\__,_|\___|
                               |_|    |___/

EOF

IMAGE_REGISTRY="${IMAGE_REGISTRY:-docker.io}"

export KUBECONFIG=/home/$USER/.kube/config

# Add /usr/local/bin to the PATH as some utilities, like kubectl, could be installed there
export PATH="$PATH:/usr/local/bin"

processSAN() {
  local result="subjectAltName=DNS:localhost"
  for domain in "$@"; do
    result="${result},DNS:${domain}"
  done
  echo "${result}"
}

processCerts() {
  echo "Generating key..."

  if ! openssl version; then
    echo "OpenSSL not found!"
    exit 1
  fi

  openssl="openssl"

  tmpDir=$(mktemp -d)
  $openssl genrsa -out "$tmpDir/infra-tls.key" 4096

  echo "Generating certificate..."
  san=$(processSAN "$@")
  # Generate the certificate with the name infra-tls.crt
  $openssl req -key "$tmpDir/infra-tls.key" -new -x509 -days 365 -out "$tmpDir/infra-tls.crt" -subj "/C=US/O=Orch Deploy/OU=Open Edge Platform" -addext "$san"
  cp -f "${tmpDir}"/infra-tls.crt /usr/local/share/ca-certificates/gitea_cert.crt

  update-ca-certificates -f

  # Create a tls secret with custom key names
  kubectl create secret tls gitea-tls-certs -n gitea \
    --cert="$tmpDir/infra-tls.crt" \
    --key="$tmpDir/infra-tls.key"

  # Clean up the temporary directory
  rm -rf "$tmpDir"
}

randomPassword() {
  tr -dc A-Za-z0-9 </dev/urandom | head -c 16
}

createGiteaSecret() {
  local secretName=$1
  local accountName=$2
  local password=$3
  local namespace=$4

  kubectl create secret generic "$secretName" -n "$namespace" \
    --from-literal=username='$accountName' \
    --from-literal=password='$password' \
    --dry-run=client -o yaml | kubectl apply -f -
}

createGiteaAccount() {
  local secretName=$1
  local accountName=$2
  local password=$3
  local email=$4
  local giteaPods=""
  local giteaPod=""

  giteaPods=$(kubectl get pods -n gitea -l app=gitea -o jsonpath="{.items[*].metadata.name}")
  if [ -z "$giteaPods" ]; then
    echo "No Gitea pods found. Exiting."
    exit 1
  fi

  giteaPod=$(echo "$giteaPods" | cut -d ' ' -f1)
  if ! kubectl exec -n gitea "$giteaPod" -c gitea -- gitea admin user list | grep -q "$accountName"; then
    echo "Creating Gitea account for $accountName"
    kubectl exec -n gitea "$giteaPod" -c gitea -- gitea admin user create --username "$accountName" --password "$password" --email "$email" --must-change-password=false
  else
    echo "Gitea account for $accountName already exists, updating password"
    kubectl exec -n gitea "$giteaPod" -c gitea -- gitea admin user change-password --username "$accountName" --password "$password" --must-change-password=false
  fi

  userToken=$(kubectl exec -n gitea "$giteaPod" -c gitea -- gitea admin user generate-access-token --scopes write:repository,write:user --username "$accountName" --token-name "${accountName}-$(date +%s)")
  token=$(echo "$userToken" | awk '{print $NF}')
  kubectl create secret generic gitea-"$accountName"-token -n gitea --from-literal=token='$token' --dry-run=client -o yaml | kubectl apply -f -
}

kubectl create ns gitea >/dev/null 2>&1 || true
kubectl create ns orch-platform >/dev/null 2>&1 || true
kubectl -n gitea get secret gitea-tls-certs >/dev/null 2>&1 || processCerts gitea-http.gitea.svc.cluster.local

adminGiteaPassword=$(randomPassword)
argocdGiteaPassword=$(randomPassword)
appGiteaPassword=$(randomPassword)
clusterGiteaPassword=$(randomPassword)

# Create secret for Gitea admin user but should not be used for normal operations
createGiteaSecret "gitea-cred" "gitea_admin" "$adminGiteaPassword" "gitea"

# Create user credential secrets for ArgoCD, AppOrch and ClusterOrch
createGiteaSecret "argocd-gitea-credential" "argocd" "$argocdGiteaPassword" "gitea"
createGiteaSecret "app-gitea-credential" "apporch" "$appGiteaPassword" "orch-platform"
createGiteaSecret "cluster-gitea-credential" "clusterorch" "$clusterGiteaPassword" "orch-platform"

# Need to scale down the pod
kubectl scale deployment gitea -n gitea --replicas=0

# More helm values are set in ../assets/gitea/values.yaml
echo "Starting Gitea Helm upgrade..."
echo "Docker registry: ${IMAGE_REGISTRY}"
echo "Checking storage class availability before upgrade..."
kubectl get storageclass -o wide || true
kubectl get pvc -n gitea || true

# Wait for Kubernetes to be ready by checking for metrics API availability and basic apiserver readiness
echo "Waiting for Kubernetes system to be ready..."
max_retries=30
retry_count=0
while [ $retry_count -lt $max_retries ]; do
  # Check if basic kubectl commands work and metrics are available
  if kubectl get nodes >/dev/null 2>&1 && kubectl api-resources 2>/dev/null | grep -q "metrics"; then
    echo "Kubernetes system is ready"
    break
  fi
  retry_count=$((retry_count + 1))
  if [ $retry_count -eq $max_retries ]; then
    echo "Warning: Kubernetes system may not be fully ready, continuing anyway..."
  else
    echo "Kubernetes not ready yet, waiting... ($retry_count/$max_retries)"
    sleep 10
  fi
done

echo "Starting helm upgrade (without --wait due to infrastructure phase)..."

# Upgrade Gitea without --wait to avoid hanging on pod readiness checks
# Pod readiness will be verified with manual wait loop below
helm upgrade --install gitea /tmp/gitea/gitea --values /tmp/gitea/values.yaml --set gitea.admin.existingSecret=gitea-cred --set image.registry="${IMAGE_REGISTRY}" -n gitea --timeout 15m0s

# Wait for Gitea pod to be ready with manual polling
echo "Waiting for Gitea pod to be ready..."
max_retries=180  # 30 minutes with 10s intervals
retry_count=0
while [ $retry_count -lt $max_retries ]; do
  ready=$(kubectl get deployment -n gitea gitea -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' 2>/dev/null)
  if [ "$ready" = "True" ]; then
    echo "Gitea pod is ready"
    break
  fi
  retry_count=$((retry_count + 1))
  if [ $retry_count -eq $max_retries ]; then
    echo "Error: Gitea pod failed to become ready after 30 minutes"
    kubectl describe deployment -n gitea gitea || true
    kubectl logs -n gitea -l app=gitea --tail=50 || true
    exit 1
  fi
  if [ $((retry_count % 6)) -eq 0 ]; then
    echo "Still waiting for Gitea pod... ($((retry_count * 10))s elapsed)"
    kubectl get pods -n gitea || true
  fi
  sleep 10
done

echo "Gitea Helm upgrade completed"

# Create Gitea accounts for ArgoCD, AppOrch and ClusterOrch
createGiteaAccount "argocd-gitea-credential" "argocd" "$argocdGiteaPassword" "argocd@orch-installer.com"
createGiteaAccount "app-gitea-credential" "apporch" "$appGiteaPassword" "test@test.com"
createGiteaAccount "cluster-gitea-credential" "clusterorch" "$clusterGiteaPassword" "test@test2.com"
