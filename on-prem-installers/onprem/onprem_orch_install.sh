#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: onprem_orch_install.sh
# Description: Main Edge Orchestrator installation script that:
#               - Loads configuration from onprem.env file
#               - Updates proxy configuration in deployment repository
#               - Installs Gitea Git repository server
#               - Installs ArgoCD for GitOps deployment
#               - Creates required Kubernetes namespaces
#               - Creates secrets (Harbor, Keycloak, Postgres, SRE, SMTP)
#               - Installs Edge Orchestrator software packages
#                 * Populates Gitea with Edge Orchestrator deployment code
#                 * Kickstarts deployment via ArgoCD
#
# Usage: ./onprem_orch_install.sh [OPTIONS]
#   Options:
#     -h, --help            Show help message
#     -s, --sre [PATH]      Enable SRE TLS with optional CA certificate path
#     -d, --notls           Disable SMTP TLS verification
#     -y, --yes             Assume 'yes' to all prompts and run non-interactively
#     --disable-co          Disable Cluster Orchestrator profile
#     --disable-ao          Disable Application Orchestrator profile
#     --disable-o11y        Disable Observability profile
#     -st, --single_tenancy Enable single tenancy mode
#     -t, --trace           Enable debug tracing
#
# Prerequisites: 
#   - onprem_pre_install.sh must have completed successfully
#   - onprem.env file must exist with proper configuration

set -e
set -o pipefail

# Import shared functions
# shellcheck disable=SC1091
source "$(dirname "$0")/functions.sh"

### Variables
cwd=$(pwd)

ASSUME_YES=false
ENABLE_TRACE=false
SINGLE_TENANCY_PROFILE=false
INSTALL_GITEA="true"
deb_dir_name="installers"
git_arch_name="repo_archives"
argo_cd_ns="argocd"
gitea_ns="gitea"
si_config_repo="edge-manageability-framework"
export GIT_REPOS=$cwd/$git_arch_name

# Source main environment configuration if it exists
MAIN_ENV_CONFIG="$(dirname "$0")/onprem.env"

create_smtp_secrets() {
  # Check if SMTP variables are set
  if [[ -z "${SMTP_ADDRESS:-}" || -z "${SMTP_PORT:-}" || -z "${SMTP_HEADER:-}" || -z "${SMTP_USERNAME:-}" || -z "${SMTP_PASSWORD:-}" ]]; then
    echo "Warning: SMTP configuration variables not set. Skipping SMTP secrets creation."
    echo "To enable SMTP, uncomment and configure SMTP variables in onprem.env file."
    return
  fi

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
  # Check if SRE variables are set
  if [[ -z "${SRE_USERNAME:-}" || -z "${SRE_PASSWORD:-}" || -z "${SRE_DEST_URL:-}" ]]; then
    echo "Warning: SRE configuration variables not set. Skipping SRE secrets creation."
    echo "To enable SRE, uncomment and configure SRE variables in onprem.env file."
    return
  fi

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

reset_runtime_variables() {
  local config_file="$cwd/onprem.env"
  
  echo "Cleaning up runtime variables from previous runs..."
  
  local temp_file="${config_file}.tmp"
  local in_multiline=0
  
  while IFS= read -r line || [[ -n "$line" ]]; do
    # Skip lines while inside a multi-line variable
    if [[ $in_multiline -eq 1 ]]; then
      [[ "$line" =~ [\'\"][[:space:]]*$ ]] && in_multiline=0
      continue
    fi
    
    # Check if line is a runtime variable
    if [[ "$line" =~ ^export\ (SRE_TLS_ENABLED|SRE_DEST_CA_CERT|SMTP_SKIP_VERIFY|DISABLE_CO_PROFILE|DISABLE_AO_PROFILE|DISABLE_O11Y_PROFILE)= ]]; then
      # Check if it's multi-line (opening quote without closing quote on same line)
      if [[ "$line" =~ =[\'\"]. ]] && ! [[ "$line" =~ =[\'\"].*[\'\"][[:space:]]*$ ]]; then
        in_multiline=1
      fi
      continue
    fi
    
    # Keep non-runtime variable lines
    echo "$line" >> "$temp_file"
  done < "$config_file"
  
  mv "$temp_file" "$config_file"
  
  # Unset variables in current shell
  unset SRE_TLS_ENABLED SRE_DEST_CA_CERT SMTP_SKIP_VERIFY
  unset DISABLE_CO_PROFILE DISABLE_AO_PROFILE DISABLE_O11Y_PROFILE
  
  echo "Runtime variables cleaned successfully."
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

usage() {
  cat >&2 <<EOF
Purpose:
Install OnPrem Edge Orchestrator main components including Gitea, ArgoCD, and all orchestrator services.
This script should be run after onprem_pre_install.sh has completed successfully.

Prerequisites:
- onprem_pre_install.sh must have been run successfully
- onprem.env file must exist with proper configuration
- RKE2 Kubernetes cluster must be running
- Root/sudo access for package installation

Usage:
$(basename "$0") [OPTIONS]

Examples:
./$(basename "$0")                    # Basic installation with onprem.env config
./$(basename "$0") -s /path/to/ca.crt # Enable SRE TLS with custom CA certificate
./$(basename "$0") -d                 # Disable SMTP TLS verification
./$(basename "$0") -st                # Enable single tenancy mode
./$(basename "$0") -t                 # Enable debug tracing

Options:
    -h, --help                 Show this help message and exit
    
    -s, --sre [CA_CERT_PATH]   Enable TLS for SRE (Site Reliability Engineering) Exporter
                               Optionally provide path to SRE destination CA certificate
                               Example: -s /path/to/sre-ca.crt
    
    -d, --notls                Disable TLS verification for SMTP endpoint
                               Use when SMTP server has self-signed certificates
    
    -y, --yes                  Assume 'yes' to all prompts and run non-interactively
                               Skips configuration review prompt
    
    --disable-co               Disable Cluster Orchestrator profile
                               Skips AO and CO related component installation
    
    --disable-ao               Disable Application Orchestrator profile
                               Skips AO related component installation
    
    --disable-o11y             Disable Observability (O11y) profile
                               Skips monitoring and observability component installation
    
    -st, --single_tenancy      Enable single tenancy mode
                               Configures the system for single tenant deployment
    
    -t, --trace                Enable bash debug tracing (set -x)
                               Shows detailed command execution for troubleshooting

Configuration:
    All configuration is read from onprem.env file. Key variables include:
    - RELEASE_SERVICE_URL: Registry for packages and images
    - DEPLOY_VERSION: Version of Edge Orchestrator to deploy
    - ORCH_INSTALLER_PROFILE: Deployment profile (onprem/onprem-dev)
    - GITEA_IMAGE_REGISTRY: Registry for Gitea images
    - SRE and SMTP credentials: For monitoring and email notifications

Output:
    After installation, monitor deployment status with: kubectl get applications -A
    Installation is complete when 'root-app' is 'Healthy' and 'Synced'

EOF
}
write_shared_variables() {
  local config_file="$cwd/onprem.env"
  
  # Update SRE TLS configuration using common function
  if [[ -n "${SRE_TLS_ENABLED:-}" && "${SRE_TLS_ENABLED}" != "false" ]]; then
    update_config_variable "$config_file" "SRE_TLS_ENABLED" "${SRE_TLS_ENABLED}"
    update_config_variable "$config_file" "SRE_DEST_CA_CERT" "${SRE_DEST_CA_CERT}"
  fi

  # Update SMTP configuration using common function
  if [[ -n "${SMTP_SKIP_VERIFY:-}" && "${SMTP_SKIP_VERIFY}" == "true" ]]; then
    update_config_variable "$config_file" "SMTP_SKIP_VERIFY" "${SMTP_SKIP_VERIFY}"
  fi
  
  # Update profile disable flags using common function
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
################################
##### INSTALL SCRIPT START #####
################################

### Installer
echo "Running On Premise Edge Orchestrator installers"

if [ "$(dpkg -l | grep -ci onprem-ke-installer)"  -eq 0 ]; then
    echo "Please run pre-install script first"
    exit 1
fi

# Remove runtime variables from previous runs
reset_runtime_variables

# Re-source the config file after cleanup to get fresh values
if [[ -f "$MAIN_ENV_CONFIG" ]]; then
  # shellcheck disable=SC1090
  source "$MAIN_ENV_CONFIG"
fi

if [ -n "${1-}" ]; then
  while :; do
    case "$1" in
      -h|--help)
        usage
        exit 0
      ;;
      -s|--sre)
        SRE_TLS_ENABLED="true"
        if [ "$2" ]; then
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

if [ "$ENABLE_TRACE" = true ]; then
    set -x
fi

# Print environment variables
print_env_variables

# Update env file with runtime configuration from command-line arguments
write_shared_variables

# Generate Cluster Config
./generate_cluster_yaml.sh onprem

# Check if enable-app-orch.yaml is present and uncommented in the cluster config file
if grep -q "^[[:space:]]*[^#]*-[[:space:]]*.*enable-app-orch\.yaml" "$ORCH_INSTALLER_PROFILE".yaml; then
  INSTALL_GITEA="true"
else
  INSTALL_GITEA="false"
fi

# cp changes to tmp repo
tmp_dir="$cwd/$git_arch_name/tmp"
cp "$ORCH_INSTALLER_PROFILE".yaml "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml

if [ "$ASSUME_YES" = false ]; then
  while true; do
      if [[ -n ${PROCEED} ]]; then
          break
      fi
      read -rp "Edit config values.yaml files with custom configurations if necessary!!!
  The files are located at:
  $tmp_dir/$si_config_repo/orch-configs/profiles/<profile>.yaml
  $tmp_dir/$si_config_repo/orch-configs/clusters/$ORCH_INSTALLER_PROFILE.yaml
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

## Tar back the edge-manageability-framework repo. This will be later pushed to Gitea repo in the Orchestrator Installer
repo_file=$(find "$cwd/$git_arch_name" -name "*$si_config_repo*.tgz" -type f -printf "%f\n")
cd "$tmp_dir"
tar -zcf "$repo_file" ./edge-manageability-framework
mv -f "$repo_file" "$cwd/$git_arch_name/$repo_file"
cd "$cwd"
rm -rf "$tmp_dir"

if find "$cwd/$deb_dir_name" -name "onprem-gitea-installer_*_amd64.deb" -type f | grep -q .; then
    # Run gitea installer
    echo "Installing Gitea"
    # INSTALL_GITEA=$([[ "${DISABLE_CO_PROFILE:-}" == "true" || "${DISABLE_AO_PROFILE:-}" == "true" ]] && echo "false" || echo "true")
    eval "sudo IMAGE_REGISTRY=${GITEA_IMAGE_REGISTRY} INSTALL_GITEA=${INSTALL_GITEA} NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive apt-get install -y $cwd/$deb_dir_name/onprem-gitea-installer_*_amd64.deb"
    wait_for_namespace_creation $gitea_ns
    sleep 30s
    wait_for_pods_running $gitea_ns
    echo "Gitea Installed"
else
    echo "❌ Package file NOT found: $cwd/$deb_dir_name/onprem-gitea-installer_*_amd64.deb"
    echo "Please ensure the package file exists and the path is correct."
    exit 1
fi
if find "$cwd/$deb_dir_name" -name "onprem-argocd-installer_*_amd64.deb" -type f | grep -q .; then
    # Run argo CD installer
    echo "Installing ArgoCD..."
    eval "sudo NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive apt-get install -y $cwd/$deb_dir_name/onprem-argocd-installer_*_amd64.deb"
    wait_for_namespace_creation $argo_cd_ns
    sleep 30s
    wait_for_pods_running $argo_cd_ns
    echo "ArgoCD installed"
else
    echo "❌ Package file NOT found: $cwd/$deb_dir_name/onprem-argocd-installer_*_amd64.deb"
    echo "Please ensure the package file exists and the path is correct."
    exit 1
fi

# Create required namespaces
create_namespaces

# create sre and smtp secrets
create_sre_secrets
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
    eval "sudo INSTALL_GITEA=${INSTALL_GITEA} NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive ORCH_INSTALLER_PROFILE=$ORCH_INSTALLER_PROFILE GIT_REPOS=$GIT_REPOS apt-get install -y $cwd/$deb_dir_name/onprem-orch-installer_*_amd64.deb"
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
