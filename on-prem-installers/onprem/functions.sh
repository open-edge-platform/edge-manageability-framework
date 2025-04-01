#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Description: Collection of functions shared between onprem scripts.

### Functions

create_harbor_secret() {
    kubectl -n "$1" delete secret harbor-admin-credential --ignore-not-found

    kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: harbor-admin-credential
  namespace: $1
stringData:
  credential: "admin:$2"
EOF
}

create_harbor_password() {
    kubectl -n "$1" delete secret harbor-admin-password --ignore-not-found

    kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: harbor-admin-password
  namespace: $1
stringData:
  HARBOR_ADMIN_PASSWORD: "$2"
EOF
}

create_keycloak_password() {
    kubectl -n "$1" delete secret platform-keycloak --ignore-not-found

    kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: platform-keycloak
  namespace: $1
stringData:
  admin-password: "$2"
EOF
}

# Generates a random password for Keycloak with the following requirements:
# - At least one lowercase letter
# - At least one uppercase letter
# - At least one digit
# - At least one special character from the set !@#$%^&*()_+{}|:<>?
# - Total length of 25 characters
# The password is created by generating random characters for each category,
# filling the rest with additional random characters, and shuffling the result.
generate_keycloak_password() {
    # Generate random characters for each category
    # shellcheck disable=SC2018
    lowercase=$(tr -dc 'a-z' < /dev/urandom | head -c 1)
    # shellcheck disable=SC2019
    uppercase=$(tr -dc 'A-Z' < /dev/urandom | head -c 1)
    digit=$(tr -dc '0-9' < /dev/urandom | head -c 1)
    special=$(tr -dc '!@#$%^&*()_+{}|:<>?' < /dev/urandom | head -c 1)
    
    # Generate additional random characters to fill the rest of the password
    remaining=$(tr -dc 'a-zA-Z0-9!@#$%^&*()_+{}|:<>?' < /dev/urandom | head -c 21)
    
    # Combine all parts and shuffle them to create the final password
    password=$(echo "$lowercase$uppercase$digit$special$remaining" | fold -w1 | shuf | tr -d '\n')
    
    echo "$password"
}

# Checks if oras tool is installed
check_oras() {
    if ! command -v oras &>/dev/null; then
        echo "Oras is not installed, install oras, exiting..."
        exit 1
    fi
}

# Install jq tool
install_jq() {
    if ! command -v jq &>/dev/null; then
        sudo NEEDRESTART_MODE=a apt-get install -y jq
    else
        echo jq tool found in the path
    fi
}

# Install yq tool
install_yq() {
    if ! command -v yq &>/dev/null; then
        curl -jL https://github.com/mikefarah/yq/releases/download/v4.42.1/yq_linux_amd64 -o /tmp/yq
        sudo mv /tmp/yq /usr/bin/yq
        sudo chmod 755 /usr/bin/yq
    else
        echo yq tool found in the path
    fi
}

# Downloads artifacts from OCI registry in Release Service
# download_artifacts <cwd> <directory> <release service URL> <path in release service> <array[@] of package names>
download_artifacts() {
    cwd=$1
    dir_name=$2
    rs_url=$3
    rs_path=$4
    shift 4
    download_list=("$@")

    mkdir -p "$cwd/$dir_name"
    cd "$cwd/$dir_name" || exit 1
    for artifact in "${download_list[@]}"; do
        sudo oras pull -v "$rs_url/$rs_path/$artifact"
    done
    cd "$cwd" || exit 1
}

# Gets JWT token from Azure
# get_JWT_token <refresh token> <release service URL>
get_JWT_token() {
    curl -X POST -d "refresh_token=$1&grant_type=refresh_token" "https://$2/oauth/token" | jq -r .id_token
}

# Waits for pods in namespace to be in Ready state
# wait_for_pods_running <namespace>
wait_for_pods_running() {
    kubectl wait pod --selector='!job-name' --all --for=condition=Ready --namespace="$1" --timeout=600s
}

# Waits for deployment to be in Ready state
# wait_for_deploy <deployment_name> <namespace>
wait_for_deploy() {
    kubectl rollout status deploy/"$1" -n "$2" --timeout=30m
}

# Waits for pods in namespace to be created
# wait_for_namespace_creation <namespace>
wait_for_namespace_creation() {
    while [ "$(kubectl get ns "$1" -o json | jq .status.phase -r)" != "Active" ]
    do
        sleep 5
    done
}
