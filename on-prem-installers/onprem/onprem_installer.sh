#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: onprem_installer.sh
# Description: This script:
#               Creates secrets (with user inputs where required)
#               Creates namespaces
#               Installs Edge Orchestrator SW:
#                   Untars and populates Gitea repos with Edge Orchestrator deployment code
#                   Kickstarts deployment via ArgoCD

set -e
set -o pipefail

# Import shared functions
# shellcheck disable=SC1091
source "$(dirname "$0")/functions.sh"

### Constants

RELEASE_SERVICE_URL="${RELEASE_SERVICE_URL:-registry-rs.edgeorchestration.intel.com}"
ORCH_INSTALLER_PROFILE="${ORCH_INSTALLER_PROFILE:-onprem}"
DEPLOY_VERSION="${DEPLOY_VERSION:-v2025.2.0}"

### Variables
cwd=$(pwd)

deb_dir_name="installers"
git_arch_name="repo_archives"
export GIT_REPOS=$cwd/$git_arch_name
export KUBECONFIG="${KUBECONFIG:-/home/$USER/.kube/config}"

set_default_sre_env() {
  if [[ -z ${SRE_USERNAME} ]]; then
    export SRE_USERNAME=sre
  fi
  if [[ -z ${SRE_PASSWORD} ]]; then
    if [[ -z ${ORCH_DEFAULT_PASSWORD} ]]; then
      export SRE_PASSWORD=123
    else
      export SRE_PASSWORD=$ORCH_DEFAULT_PASSWORD
    fi
  fi
  if [[ -z ${SRE_DEST_URL} ]]; then
    export SRE_DEST_URL="http://sre-exporter-destination.orch-sre.svc.cluster.local:8428/api/v1/write"
  fi
  ## we don't create SRE_DEST_CA_CERT by default
}

set_default_smtp_env() {
  if [[ -z ${SMTP_ADDRESS} ]]; then
    export SMTP_ADDRESS="smtp.serveraddress.com"
  fi
  if [[ -z ${SMTP_PORT} ]]; then
    export SMTP_PORT="587"
  fi
  # Firstname Lastname <email@example.com> format expected
  if [[ -z ${SMTP_HEADER} ]]; then
    export SMTP_HEADER="foo bar <foo@bar.com>"
  fi
  if [[ -z ${SMTP_USERNAME} ]]; then
    export SMTP_USERNAME="uSeR"
  fi
  if [[ -z ${SMTP_PASSWORD} ]]; then
    export SMTP_PASSWORD=T@123sfD
  fi
}

create_smtp_secrets() {
  namespace=orch-infra
  kubectl -n $namespace delete secret smtp --ignore-not-found
  kubectl -n $namespace delete secret smtp-auth --ignore-not-found

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
  namespace=orch-sre
  kubectl -n $namespace delete secret basic-auth-username --ignore-not-found
  kubectl -n $namespace delete secret basic-auth-password --ignore-not-found
  kubectl -n $namespace delete secret destination-secret-url --ignore-not-found
  kubectl -n $namespace delete secret destination-secret-ca --ignore-not-found

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
$(printf "%s" "$SRE_DEST_CA_CERT" |sed -e $'s/^/    /')
EOF
  fi
}


print_env_variables() {
  echo; echo "========================================"
  echo "         Environment Variables"
  echo "========================================"
  printf "%-25s: %s\n" "RELEASE_SERVICE_URL" "$RELEASE_SERVICE_URL"
  printf "%-25s: %s\n" "ORCH_INSTALLER_PROFILE" "$ORCH_INSTALLER_PROFILE"
  printf "%-25s: %s\n" "DEPLOY_VERSION" "$DEPLOY_VERSION"
  echo "========================================"; echo
}

create_namespaces() {
  orch_namespace_list=(
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

  userToken=$(kubectl exec -n gitea "$giteaPod" -c gitea -- gitea admin user generate-access-token --scopes write:repository,write:user --username "$accountName" --token-name "${accountName}-$(date +%s)")
  token=$(echo "$userToken" | awk '{print $NF}')
  kubectl create secret generic gitea-"$accountName"-token -n gitea --from-literal=token="$token"
}

randomPassword() {
  head -c 512 /dev/urandom | tr -dc A-Za-z0-9 | head -c 16
}

################################
##### INSTALL SCRIPT START #####
################################

### Installer
echo "Running On Premise Edge Orchestrator installers"

if [ $(dpkg -l | grep -i onprem-ke-installer| wc -l) -eq 0 ]; then
    echo "Please run pre-installer script first"
    exit 1
fi

# Print environment variables
print_env_variables

# Create required namespaces
create_namespaces

# Create secret for Gitea
appGiteaPassword=$(randomPassword)
clusterGiteaPassword=$(randomPassword)

# Create user credential secrets for  AppOrch and ClusterOrch
createGiteaSecret "app-gitea-credential" "apporch" "$appGiteaPassword" "orch-platform"
createGiteaSecret "cluster-gitea-credential" "clusterorch" "$clusterGiteaPassword" "orch-platform"

# Create Gitea accounts for AppOrch and ClusterOrch
createGiteaAccount "app-gitea-credential" "apporch" "$appGiteaPassword" "apporch@orch-installer.com"
createGiteaAccount "cluster-gitea-credential" "clusterorch" "$clusterGiteaPassword" "clusterorch@orch-installer.com"

# create sre and smtp secrets
set_default_sre_env
create_sre_secrets
set_default_smtp_env
create_smtp_secrets
# Create secrets for Harbor, Keycloak and Postgres
harbor_password=$(head -c 512 /dev/urandom | tr -dc A-Za-z0-9 | cut -c1-100)
keycloak_password=$(generate_password)
postgres_password=$(generate_password)
create_harbor_secret orch-harbor "$harbor_password"
create_harbor_password orch-harbor "$harbor_password"
create_keycloak_password orch-platform "$keycloak_password"
create_postgres_password orch-database "$postgres_password"

if find "$cwd/$deb_dir_name" -name "onprem-orch-installer_*_amd64.deb" -type f | grep -q .; then
    # Run orchestrator installer
    echo "Installing Edge Orchestrator Packages"
    eval "sudo NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive ORCH_INSTALLER_PROFILE=$ORCH_INSTALLER_PROFILE GIT_REPOS=$GIT_REPOS apt-get install -y $cwd/$deb_dir_name/onprem-orch-installer_*_amd64.deb"
    echo "Edge Orchestrator getting installed, wait for SW to deploy... "
else
    echo "❌ Package file NOT found: $cwd/$deb_dir_name/onprem-orch-installer_*_amd64.deb"
    echo "Please ensure the package file exists and the path is correct."
    exit 1
fi

printf "\nEdge Orchestrator SW is being deployed, please wait for all applications to deploy...\n
To check the status of the deployment run 'kubectl get applications -A'.\n
Installation is completed when 'root-app' Application is in 'Healthy' and 'Synced' state.\n
Once it is completed, you might want to configure DNS for UI and other services by running generate_fqdn script and following instructions\n"
