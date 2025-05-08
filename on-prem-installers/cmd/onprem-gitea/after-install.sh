#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

cat << "EOF"

   ____ _ _
  / ___(_) |_ ___  __ _
 | |  _| | __/ _ \/ _` |
 | |_| | | ||  __/ (_| |
  \____|_|\__\___|\__,_|


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
  $openssl req -key "$tmpDir/infra-tls.key" -new -x509 -days 365 -out "$tmpDir/infra-tls.crt" -subj "/C=US/O=Orch Deploy/OU=Orchestrator" -addext "$san"
  cp "${tmpDir}"/infra-tls.crt /usr/local/share/ca-certificates/gitea_cert.crt
  update-ca-certificates

  # Create a tls secret with custom key names
  kubectl create secret tls gitea-tls-certs -n gitea \
    --cert="$tmpDir/infra-tls.crt" \
    --key="$tmpDir/infra-tls.key"

  # Create a tls secret with custom key names for orch-platform namespace
  # This is needed to access the gitea service from the orch-platform namespace  
  kubectl create secret tls gitea-tls-certs -n orch-platform \
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
    --from-literal=username="$accountName" \
    --from-literal=password="$password" \
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
  if ! kubectl exec -n gitea "$giteaPod" -c "gitea" -- gitea admin user list | grep -q "$accountName"; then
    echo "Creating Gitea account for $accountName"
    kubectl exec -n gitea "$giteaPod" -c "gitea" -- gitea admin user create --username "$accountName" --password "$password" --email "$email" --must-change-password=false
  else
    echo "Gitea account for $accountName already exists, updating password"
    kubectl exec -n gitea "$giteaPod" -c "gitea" -- gitea admin user change-password --username "$accountName" --password "$password" --must-change-password=false
  fi

  userToken=$(kubectl exec -n gitea "$giteaPod" -c gitea -- gitea admin user generate-access-token --scopes write:repository,write:user --username $accountName --token-name "${accountName}-$(date +%s)")
  token=$(echo $userToken | awk '{print $NF}')
  kubectl create secret generic gitea-$accountName-token -n gitea --from-literal=token=$token
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
createGiteaSecret "argocd-gitea-credential" "argocd" "$argocdGiteaPassword" "orch-platform"
createGiteaSecret "app-gitea-credential" "apporch" "$appGiteaPassword" "orch-platform"
createGiteaSecret "cluster-gitea-credential" "clusterorch" "$clusterGiteaPassword" "orch-platform"

# More helm values are set in ../assets/gitea/values.yaml
helm install gitea /tmp/gitea/gitea --values /tmp/gitea/values.yaml --set gitea.admin.existingSecret=gitea-cred --set image.registry="${IMAGE_REGISTRY}" -n gitea --timeout 15m0s --wait

# Create Gitea accounts for ArgoCD, AppOrch and ClusterOrch
createGiteaAccount "argocd-gitea-credential" "argocd" "$argocdGiteaPassword" "argocd@orch-installer.com"
createGiteaAccount "app-gitea-credential" "apporch" "$appGiteaPassword" "apporch@orch-installer.com"
createGiteaAccount "cluster-gitea-credential" "clusterorch" "$clusterGiteaPassword" "clusterorch@orch-installer.com"
