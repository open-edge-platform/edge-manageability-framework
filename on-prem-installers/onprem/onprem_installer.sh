#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: onprem_installer.sh
# Description: This script:
#               Reads AZURE AD refresh_token credential from user input,
#               Downloads installer and repo artifacts,
#               Set's up OS level dependencies,
#               Installs RKE2 and basic cluster components,
#               Installs ArgoCD
#               Installs Gitea
#               Creates secrets (with user inputs where required)
#               Creates namespaces
#               Installs Edge Orchestrator SW:
#                   Untars and populates Gitea repos with Edge Orchestrator deployment code
#                   Kickstarts deployment via ArgoCD

# Usage: ./onprem_installer
#    -s:             Enables TLS for SRE Exporter. Private TLS CA cert may be provided for SRE destination as an additional argument - provide path to cert (optional)
#    -d:             Disable TLS verification for SMTP endpoint
#    -h:             help (optional)
#    -o:             Override production values with dev values
#    -u:             Set the Release Service URL

set -e
set -o pipefail

# Import shared functions
# shellcheck disable=SC1091
source "$(dirname "$0")/functions.sh"
wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
export PATH=${PATH}:`go env GOPATH`/bin:/home/ubuntu/.asdf/installs/mage/1.15.0/bin
cd ../
go install github.com/asdf-vm/asdf/cmd/asdf@v0.17.0
asdf plugin add mage
asdf install mage latest
# go mod download
# asdf global mage latest
cd -


### Constants

export RELEASE_SERVICE_URL="${RELEASE_SERVICE_URL:-registry-rs.edgeorchestration.intel.com}"
export ORCH_INSTALLER_PROFILE="${ORCH_INSTALLER_PROFILE:-onprem}"
export DEPLOY_VERSION="${DEPLOY_VERSION:-v3.1.0}"
export GITEA_IMAGE_REGISTRY="${GITEA_IMAGE_REGISTRY:-docker.io}"

### Variables
export cwd=$(pwd)

export deb_dir_name="installers"
export git_arch_name="repo_archives"
export argo_cd_ns=argocd
export gitea_ns=gitea
export archives_rs_path="edge-orch/common/files/orchestrator"
export si_config_repo="edge-manageability-framework"
export installer_rs_path="edge-orch/common/files"

export tmp_dir="$cwd/$git_arch_name/tmp"
export KUBECONFIG=/home/$USER/.kube/config

export ASSUME_YES=false
export SKIP_DOWNLOAD=false
export ENABLE_TRACE=false



# Variables that depend on the above and might require updating later, are placed in here
set_artifacts_version() {
  installer_list=(
    "onprem-config-installer:${DEPLOY_VERSION}"
    "onprem-ke-installer:${DEPLOY_VERSION}"
    "onprem-argocd-installer:${DEPLOY_VERSION}"
    "onprem-gitea-installer:${DEPLOY_VERSION}"
    "onprem-orch-installer:${DEPLOY_VERSION}"
  )

  git_archive_list=(
    "onpremfull:${DEPLOY_VERSION}"
  )
}

export GIT_REPOS=$cwd/$git_arch_name

### Functions

# create_namespaces() {
#   for ns in "${orch_namespace_list[@]}"; do
#     kubectl create ns "$ns" --dry-run=client -o yaml | kubectl apply -f -
#   done
# }

# create_azure_secret() {
#   namespace=orch-secret
#   kubectl -n $namespace delete secret azure-ad-creds --ignore-not-found

#   kubectl apply -f - <<EOF
# apiVersion: v1
# kind: Secret
# metadata:
#   name: azure-ad-creds
#   namespace: $namespace
# stringData:
#   refresh_token: $AZUREAD_REFRESH_TOKEN
# EOF
# }

# set_default_sre_env() {  ## moved to create secrets
#   if [[ -z ${SRE_USERNAME} ]]; then
#     export SRE_USERNAME=sre
#   fi
#   if [[ -z ${SRE_PASSWORD} ]]; then
#     if [[ -z ${ORCH_DEFAULT_PASSWORD} ]]; then
#       export SRE_PASSWORD=123
#     else
#       export SRE_PASSWORD=$ORCH_DEFAULT_PASSWORD
#     fi
#   fi
#   if [[ -z ${SRE_DEST_URL} ]]; then
#     export SRE_DEST_URL="http://sre-exporter-destination.orch-sre.svc.cluster.local:8428/api/v1/write"
#   fi
#   ## we don't create SRE_DEST_CA_CERT by default
# }

# set_default_smtp_env() {
#   if [[ -z ${SMTP_ADDRESS} ]]; then
#     export SMTP_ADDRESS="smtp.serveraddress.com"
#   fi
#   if [[ -z ${SMTP_PORT} ]]; then
#     export SMTP_PORT="587"
#   fi
#   # Firstname Lastname <email@example.com> format expected
#   if [[ -z ${SMTP_HEADER} ]]; then
#     export SMTP_HEADER="foo bar <foo@bar.com>"
#   fi
#   if [[ -z ${SMTP_USERNAME} ]]; then
#     export SMTP_USERNAME="uSeR"
#   fi
#   if [[ -z ${SMTP_PASSWORD} ]]; then
#     export SMTP_PASSWORD=T@123sfD
#   fi
# }

# create_smpt_secrets() {
#   namespace=orch-infra
#   kubectl -n $namespace delete secret smtp --ignore-not-found
#   kubectl -n $namespace delete secret smtp-auth --ignore-not-found

#   kubectl apply -f - <<EOF
# apiVersion: v1
# kind: Secret
# metadata:
#   name: smtp
#   namespace: $namespace
# type: Opaque
# stringData:
#   smartHost: $SMTP_ADDRESS
#   smartPort: "$SMTP_PORT"
#   from: $SMTP_HEADER
#   authUsername: $SMTP_USERNAME
# EOF

#   kubectl apply -f - <<EOF
# apiVersion: v1
# kind: Secret
# metadata:
#   name: smtp-auth
#   namespace: $namespace
# type: kubernetes.io/basic-auth
# stringData:
#   password: $SMTP_PASSWORD
# EOF
# }

# create_sre_secrets() {
#   namespace=orch-sre
#   kubectl -n $namespace delete secret basic-auth-username --ignore-not-found
#   kubectl -n $namespace delete secret basic-auth-password --ignore-not-found
#   kubectl -n $namespace delete secret destination-secret-url --ignore-not-found
#   kubectl -n $namespace delete secret destination-secret-ca --ignore-not-found

#   kubectl apply -f - <<EOF
# apiVersion: v1
# kind: Secret
# metadata:
#   name: basic-auth-username
#   namespace: $namespace
# stringData:
#   username: $SRE_USERNAME
# EOF

#   kubectl apply -f - <<EOF
# apiVersion: v1
# kind: Secret
# metadata:
#   name: basic-auth-password
#   namespace: $namespace
# stringData:
#   password: "$SRE_PASSWORD"
# EOF

#   kubectl apply -f - <<EOF
# apiVersion: v1
# kind: Secret
# metadata:
#   name: destination-secret-url
#   namespace: $namespace
# stringData:
#   url: $SRE_DEST_URL
# EOF

#   if [[ -n "${SRE_DEST_CA_CERT-}" ]]; then
#   kubectl apply -f - <<EOF
# apiVersion: v1
# kind: Secret
# metadata:
#   name: destination-secret-ca
#   namespace: $namespace
# stringData:
#   ca.crt: |
# $(printf "%s" "$SRE_DEST_CA_CERT" |sed -e $'s/^/    /')
# EOF
#   fi
# }

# This script allows making changes to the configuration during its runtime.
# The function performs the following steps:
# 1. Runs the on-premises installation.
# 2. Extracts the repository archive.
# 3. Prompts the user to edit the YAML configuration file and waits for their response.
# 4. Once the user has edited the file, they respond to the prompt with 'yes'.
# 5. Upon receiving a 'yes' response, the script re-archives the repository.
# 6. Continues with the installation of the orchestrator.
# Note: If the configuration already exists, the script will prompt the user to confirm if they want to overwrite it.
# allow_config_in_runtime() {
#   if [ "$ENABLE_TRACE" = true ]; then
#     echo "Tracing is enabled. Temporarily disabling tracing"
#     set +x
#   fi

#   export tmp_dir="$cwd/$git_arch_name/tmp"

#   if [ -d "$tmp_dir/$si_config_repo" ]; then
#     echo "Configuration already exists at $tmp_dir/$si_config_repo."
#     if [ "$ASSUME_YES" = true ]; then
#       echo "Assuming yes to use existing configuration."
#       return
#     fi
#     while true; do
#       read -rp "Do you want to overwrite the existing configuration? (yes/no): " yn
#       case $yn in
#         [Yy]* ) rm -rf "${tmp_dir:?}/${si_config_repo:?}"; break;;
#         [Nn]* ) echo "Using existing configuration."; return;;
#         * ) echo "Please answer yes or no.";;
#       esac
#     done
#   fi

#   ## Untar edge-manageability-framework repo
#   repo_file=$(find "$cwd/$git_arch_name" -name "*$si_config_repo*.tgz" -type f -printf "%f\n")

#   rm -rf "$tmp_dir"
#   mkdir -p "$tmp_dir"
#   tar -xf "$cwd/$git_arch_name/$repo_file" -C "$tmp_dir"

#   # Prompt for Docker.io credentials
#   ## Docker Hub usage and limits: https://docs.docker.com/docker-hub/usage/
#   while true; do
#     if [[ -z $DOCKER_USERNAME && -z $DOCKER_PASSWORD ]]; then
#       read -rp "Would you like to provide Docker credentials? (Y/n): " yn
#       case $yn in
#         [Yy]* ) echo "Enter Docker Username:"; read -r DOCKER_USERNAME; export DOCKER_USERNAME; echo "Enter Docker Password:"; read -r -s DOCKER_PASSWORD; export DOCKER_PASSWORD; break;;
#         [Nn]* ) echo "The installation will proceed without using Docker credentials."; unset DOCKER_USERNAME; unset DOCKER_PASSWORD; break;;
#         * ) echo "Please answer yes or no.";;
#       esac
#     else
#       echo "Setting Docker credentials."
#       export DOCKER_USERNAME
#       export DOCKER_PASSWORD
#       break
#     fi
#   done

#   if [[ -n $DOCKER_USERNAME && -n $DOCKER_PASSWORD ]]; then
#     echo "Docker credentials are set."
#   else
#     echo "Docker credentials are not valid. The installation will proceed without using Docker credentials."
#     unset DOCKER_USERNAME
#     unset DOCKER_PASSWORD
#   fi

#   # Prompt for IP addresses for Argo, Traefik and Nginx services
#   echo "Provide IP addresses for Argo, Traefik and Nginx services."
#   while true; do
#     if [[ -z ${ARGO_IP} ]]; then
#       echo "Enter Argo IP:"
#       read -r ARGO_IP
#       export ARGO_IP
#     fi

#     if [[ -z ${TRAEFIK_IP} ]]; then
#       echo "Enter Traefik IP:"
#       read -r TRAEFIK_IP
#       export TRAEFIK_IP
#     fi

#     if [[ -z ${NGINX_IP} ]]; then
#       echo "Enter Nginx IP:"
#       read -r NGINX_IP
#       export NGINX_IP
#     fi

#     if [[ $ARGO_IP =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ && $TRAEFIK_IP =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ && $NGINX_IP =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
#       echo "IP addresses are valid."
#       break
#     else
#       echo "Inputted values are not valid IPs. Please input correct IPs without any masks."
#       unset ARGO_IP
#       unset TRAEFIK_IP
#       unset NGINX_IP
#     fi
#   done

#   ## Wait for SI to confirm that they have made changes
#   while true; do
#     if [[ -n ${PROCEED} ]]; then
#       break
#     fi
#     read -rp "Edit config values.yaml files with custom configurations if necessary!!!
# The files are located at:
# $tmp_dir/$si_config_repo/orch-configs/profiles/<profile>.yaml
# $tmp_dir/$si_config_repo/orch-configs/clusters/$ORCH_INSTALLER_PROFILE.yaml
# Enter 'yes' to confirm that configuration is done in order to progress with installation
# ('no' will exit the script) !!!

# Ready to proceed with installation? " yn
#     case $yn in
#       [Yy]* ) break;;
#       [Nn]* ) exit 1;;
#       * ) echo "Please answer yes or no.";;
#     esac
#   done

#   if [ "$ENABLE_TRACE" = true ]; then
#     echo "Tracing is enabled. Re-enabling tracing"
#     set -x
#   fi
# }

usage() {
  cat >&2 <<EOF
Purpose:
Install OnPrem Edge Orchestrator.

Usage:
$(basename "$0") [option...] [argument]

ex:
./$(basename "$0")
./$(basename "$0") -c <certificate string>

Options:
    -h, --help         Print this help message and exit
    -c, --cert         Path to Release Service/ArgoCD certificate
    -s, --sre          Path to SRE destination CA certificate (enables TLS for SRE Exporter)
    --skip-download    Skip downloading installer packages 
    -d, --notls        Disable TLS verification for SMTP endpoint
    -o, --override     Override production values with dev values
    -u, --url          Set the Release Service URL
    -t, --trace        Enable tracing
    -w, --write-config Write configuration to disk and exit
    -y, --yes          Assume yes for using existing configuration if it exists

Environment Variables:
    DOCKER_USERNAME    Docker.io username
    DOCKER_PASSWORD    Docker.io password

EOF
}

# print_env_variables() {
#   echo; echo "========================================"
#   echo "         Environment Variables"
#   echo "========================================"
#   printf "%-25s: %s\n" "RELEASE_SERVICE_URL" "$RELEASE_SERVICE_URL"
#   printf "%-25s: %s\n" "ORCH_INSTALLER_PROFILE" "$ORCH_INSTALLER_PROFILE"
#   printf "%-25s: %s\n" "DEPLOY_VERSION" "$DEPLOY_VERSION"
#   echo "========================================"; echo
# }

# write_configs_using_overrides() {
#   ## Option to override clusterDomain in onprem yaml by setting env variable
#   if [[ -n ${CLUSTER_DOMAIN} ]]; then
#     echo "CLUSTER_DOMAIN is set. Updating clusterDomain in the YAML file..."
#     yq -i ".argo.clusterDomain=\"${CLUSTER_DOMAIN}\"" "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
#     echo "Update complete. clusterDomain is now set to: $CLUSTER_DOMAIN"
#   fi

#   ## Override TLS setting for SRE depending on user's input (presence of flag with SRE CA cert)
#   if [[ "${SRE_TLS_ENABLED-}" == "true" ]]; then
#     yq -i '.argo.o11y.sre.tls.enabled|=true' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
#     if [[ -n "${SRE_DEST_CA_CERT-}" ]]; then
#       yq -i '.argo.o11y.sre.tls.caSecretEnabled|=true' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
#     fi
#   else
#     yq -i '.argo.o11y.sre.tls.enabled|=false' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
#   fi

#   if [[ ${SMTP_SKIP_VERIFY} == "true" ]]; then
#     yq -i '.argo.o11y.alertingMonitor.smtp.insecureSkipVerify|=true' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
#   fi

#   # Override MetalLB address pools
#   yq -i '.postCustomTemplateOverwrite.metallb-config.ArgoIP|=strenv(ARGO_IP)' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
#   yq -i '.postCustomTemplateOverwrite.metallb-config.TraefikIP|=strenv(TRAEFIK_IP)' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
#   yq -i '.postCustomTemplateOverwrite.metallb-config.NginxIP|=strenv(NGINX_IP)' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml
# }

write_config_to_disk() {
  # export tmp_dir="$cwd/$git_arch_name/tmp"
  rm -rf "$tmp_dir"
  mkdir -p "$tmp_dir"
  export repo_file=$(find "$cwd/$git_arch_name" -name "*$si_config_repo*.tgz" -type f -printf "%f\n")
  tar -xf "$cwd/$git_arch_name/$repo_file" -C "$tmp_dir"

  # If overrides are set, ensure the written out configs are updated with them
  # write_configs_using_overrides
  mage onPrem:writeConfigsUsingOverrides

  echo "Configuration files have been written to disk at $tmp_dir/$si_config_repo"
  exit 0
}

# validate_and_set_ip() {
#   local yaml_path="$1"
#   local yaml_file="$2"
#   local ip_var_name="$3"
#   local ip_value

#   echo "Value at $yaml_path in $yaml_file: $(yq "$yaml_path" "$yaml_file")"

#   if [[ -z $(yq "$yaml_path" "$yaml_file") || $(yq "$yaml_path" "$yaml_file") == "null" ]]; then
#     echo "${ip_var_name} is not set to a valid value in the configuration file."
#     while true; do
#       read -rp "Please provide a value for ${ip_var_name}: " ip_value
#       if [[ $ip_value =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
#         export "$ip_var_name"="$ip_value"
#         yq -i "$yaml_path|=strenv($ip_var_name)" "$yaml_file"
#         echo "${ip_var_name} has been set to: $ip_value"
#         break
#       else
#         unset "$ip_var_name"
#         echo "Invalid IP address. Would you like to provide a valid value? (Y/n): "
#         read -r yn
#         case $yn in
#           [Nn]* ) echo "Exiting as a valid value for ${ip_var_name} has not been provided."; exit 1;;
#           * ) ;;
#         esac
#       fi
#     done
#   fi
# }

# validate_config() {

#   # Validate the IP addresses for Argo, Traefik and Nginx services
#   validate_and_set_ip '.postCustomTemplateOverwrite.metallb-config.ArgoIP' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml ARGO_IP
#   validate_and_set_ip '.postCustomTemplateOverwrite.metallb-config.TraefikIP' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml TRAEFIK_IP
#   validate_and_set_ip '.postCustomTemplateOverwrite.metallb-config.NginxIP' "$tmp_dir"/$si_config_repo/orch-configs/clusters/"$ORCH_INSTALLER_PROFILE".yaml NGINX_IP
# }

################################
##### INSTALL SCRIPT START #####
################################

# export ASSUME_YES=false
# export SKIP_DOWNLOAD=false
# export ENABLE_TRACE=false

if [ -n "${1-}" ]; then
  while :; do
    case "$1" in
      -h|--help)
        usage
        exit 0
      ;;
      -s|--sre_tls)
        export SRE_TLS_ENABLED="true"
        if [ "$2" ]; then
          SRE_DEST_CA_CERT="$(cat "$2")"
          shift
        fi
      ;;
      --skip-download)
        export SKIP_DOWNLOAD=true
      ;;
      -d|--notls)
        export SMTP_SKIP_VERIFY="true"
      ;;
      -o|--override)
        export ORCH_INSTALLER_PROFILE="onprem-dev"
      ;;
      -u|--url)
        if [ "$2" ]; then
          export RELEASE_SERVICE_URL="$2"
          shift
        else
          echo "ERROR: $1 requires an argument"
          exit 1
        fi
      ;;
      -t|--trace)
        set -x
        export ENABLE_TRACE=true
      ;;
      -w|--write-config)
        export WRITE_CONFIG="true"
      ;;
      -y|--yes)
        export ASSUME_YES=true
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

### Installer
echo "Running On Premise Edge Orchestrator installers"

# Print environment variables
# print_env_variables
mage onPrem:printEnvVariables

# Set the version of the artifacts to be downloaded
set_artifacts_version

# Check & install script dependencies
mage onPrem:checkOras
#check_oras
mage onPrem:installJq
mage onPrem:installYq
# install_jq
# install_yq
# download_packages () {
# if  [[ $SKIP_DOWNLOAD != true  ]]; then 
#   # Cleanup and download .deb packages
#   sudo rm -rf "${cwd:?}/${deb_dir_name:?}/"

#   retry_count=0
#   max_retries=10
#   retry_delay=15

#   until download_artifacts "$cwd" "$deb_dir_name" "$RELEASE_SERVICE_URL" "$installer_rs_path" "${installer_list[@]}"; do
#     ((retry_count++))
#     if [ "$retry_count" -ge "$max_retries" ]; then
#       echo "Failed to download deb artifacts after $max_retries attempts."
#       exit 1
#     fi
#     echo "Download failed. Retrying in $retry_delay seconds... ($retry_count/$max_retries)"
#     sleep "$retry_delay"
#   done

#   sudo chown -R _apt:root $deb_dir_name

#   ## Cleanup and download .git packages
#   sudo rm -rf  "${cwd:?}/${git_arch_name:?}/"

#   retry_count=0
#   max_retries=10
#   retry_delay=15

#   until download_artifacts "$cwd" "$git_arch_name" "$RELEASE_SERVICE_URL" "$archives_rs_path" "${git_archive_list[@]}"; do
#     ((retry_count++))
#     if [ "$retry_count" -ge "$max_retries" ]; then
#       echo "Failed to download git artifacts after $max_retries attempts."
#       exit 1
#     fi
#     echo "Download failed. Retrying in $retry_delay seconds... ($retry_count/$max_retries)"
#     sleep "$retry_delay"
#   done
# else 
#   echo "Skipping packages download"
#   sudo chown -R _apt:root $deb_dir_name
# fi
# }
# download_packages
mage onPrem:downloadPackages
# Write configuration to disk if the flag is set
export repo_file=$(find "$cwd/$git_arch_name" -name "*$si_config_repo*.tgz" -type f -printf "%f\n")
if [[ "$WRITE_CONFIG" == "true" ]]; then
  write_config_to_disk
  #mage onPrem:writeConfigToDisk
fi

# Config - interactive
# allow_config_in_runtime
mage onPrem:allowConfigInRuntime

# Write out the configs that have explicit overrides
#write_configs_using_overrides
mage onPrem:writeConfigsUsingOverrides
# exit 1
# Validate the configuration file, and set missing values
mage onPrem:validateConfig
# validate_config

## Tar back the edge-manageability-framework repo. This will be later pushed to Gitea repo in the Orchestrator Installer
# export tmp_dir="$cwd/$git_arch_name/tmp"
export repo_file=$(find "$cwd/$git_arch_name" -name "*$si_config_repo*.tgz" -type f -printf "%f\n")
cd "$tmp_dir"
tar -zcf "$repo_file" ./edge-manageability-framework
mv -f "$repo_file" "$cwd/$git_arch_name/$repo_file"
cd "$cwd"
rm -rf "$tmp_dir"

# Run OS Configuration installer
echo "Installing the OS level configuration..."
eval "sudo NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive apt-get install -y $cwd/$deb_dir_name/onprem-config-installer_*_amd64.deb"
echo "OS level configuration installed"

# Run K8s Installer
echo "Installing RKE2..."
if [[ -n "${DOCKER_USERNAME}" && -n "${DOCKER_PASSWORD}" ]]; then
  echo "Docker credentials provided. Installing RKE2 with Docker credentials"
  sudo DOCKER_USERNAME="${DOCKER_USERNAME}" DOCKER_PASSWORD="${DOCKER_PASSWORD}" NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive apt-get install -y "$cwd"/$deb_dir_name/onprem-ke-installer_*_amd64.deb
else
  sudo NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive apt-get install -y "$cwd"/$deb_dir_name/onprem-ke-installer_*_amd64.deb
fi
echo "RKE2 Installed"

mkdir -p /home/"$USER"/.kube
sudo cp  /etc/rancher/rke2/rke2.yaml /home/"$USER"/.kube/config
sudo chown -R "$USER":"$USER"  /home/"$USER"/.kube
sudo chmod 600 /home/"$USER"/.kube/config


# Run gitea installer
echo "Installing Gitea"
eval "sudo IMAGE_REGISTRY=${GITEA_IMAGE_REGISTRY} NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive apt-get install -y $cwd/$deb_dir_name/onprem-gitea-installer_*_amd64.deb"
# wait_for_namespace_creation $gitea_ns
mage onPrem:waitForNamespaceCreation $gitea_ns
sleep 30s
mage onPrem:waitForPodsRunning $gitea_ns
# wait_for_pods_running $gitea_ns
echo "Gitea Installed"

# Run argo CD installer
echo "Installing ArgoCD..."
eval "sudo NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive apt-get install -y $cwd/$deb_dir_name/onprem-argocd-installer_*_amd64.deb"
# wait_for_namespace_creation $argo_cd_ns
mage onPrem:waitForNamespaceCreation $argo_cd_ns
sleep 30s
mage onPrem:waitForPodsRunning $argo_cd_ns
# wait_for_pods_running $argo_cd_ns
echo "ArgoCD installed"

# Create namespaces for ArgoCD
mage onPrem:createNamespaces
# Create secret with azure credentials
#create_azure_secret
# mage onprem:CreateAzureSecret todo: this is not used anymore
# set_default_sre_env
# create_sre_secrets
mage onPrem:createSreSecrets
# set_default_smtp_env
# create_smpt_secrets
mage onPrem:createSmtpSecrets
harbor_password=$(mage onPrem:generateHarborPassword)
keycloak_password=$(mage onPrem:generatePassword)
postgres_password=$(mage onPrem:generatePassword)
# create_harbor_secret orch-harbor "$harbor_password"
mage onPrem:createHarborSecret orch-harbor "$harbor_password"
#create_harbor_password orch-harbor "$harbor_password"
mage onPrem:createHarborPassword orch-harbor "$harbor_password"
# create_keycloak_password orch-platform "$keycloak_password"
mage onPrem:createKeycloakPassword orch-platform "$keycloak_password"
# create_postgres_password orch-database "$postgres_password"
mage onPrem:createPostgresPassword orch-database "$postgres_password"


# Run orchestrator installer
#mage onPrem:installOrchestrator
echo "Installing Edge Orchestrator Packages"
eval "sudo NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive ORCH_INSTALLER_PROFILE=$ORCH_INSTALLER_PROFILE GIT_REPOS=$GIT_REPOS apt-get install -y $cwd/$deb_dir_name/onprem-orch-installer_*_amd64.deb"
echo "Edge Orchestrator getting installed, wait for SW to deploy... "

printf "\nEdge Orchestrator SW is being deployed, please wait for all applications to deploy...\n
To check the status of the deployment run 'kubectl get applications -A'.\n
Installation is completed when 'root-app' Application is in 'Healthy' and 'Synced' state.\n
Once it is completed, you might want to configure DNS for UI and other services by running generate_fqdn script and following instructions\n"
