#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# OnPrem Orchestrator Installer
# This script pushes Edge Manageability Framework to Gitea and deploys via ArgoCD

set -o errexit
set -o pipefail

cat << "EOF"

   ___           _         ___           _        _ _           
  / _ \ _ __ ___| |__     |_ _|_ __  ___| |_ __ _| | | ___ _ __ 
 | | | | '__/ __| '_ \     | || '_ \/ __| __/ _` | | |/ _ \ '__|
 | |_| | | | (__| | | |_   | || | | \__ \ || (_| | | |  __/ |   
  \___/|_|  \___|_| |_(_) |___|_| |_|___/\__\__,_|_|_|\___|_|   
                                                                 

EOF

# Add /usr/local/bin to the PATH
export PATH=$PATH:/usr/local/bin
export KUBECONFIG="/home/${USER}/.kube/config"

# Environment variables
GIT_REPOS="${GIT_REPOS:-}"
ORCH_INSTALLER_PROFILE="${ORCH_INSTALLER_PROFILE:-}"

# Constants
EDGE_MANAGEABILITY_FRAMEWORK_REPO="edge-manageability-framework"
GITEA_SVC_DOMAIN="gitea-http.gitea.svc.cluster.local"

# Validate required environment variables
if [ -z "$GIT_REPOS" ]; then
    echo "Error: GIT_REPOS environment variable is empty"
    exit 1
fi

if [ -z "$ORCH_INSTALLER_PROFILE" ]; then
    echo "Error: ORCH_INSTALLER_PROFILE environment variable is empty"
    exit 1
fi

echo "Starting installation of orch-installer using $ORCH_INSTALLER_PROFILE profile..."

#######################################
# Get Gitea service URL
#######################################
get_gitea_service_url() {
    local port
    port=$(kubectl get svc gitea-http -n gitea -o jsonpath='{.spec.ports[0].port}')
    
    if [ "$port" = "443" ]; then
        echo "$GITEA_SVC_DOMAIN"
    else
        echo "${GITEA_SVC_DOMAIN}:${port}"
    fi
}

#######################################
# Push artifact repo to Gitea
#######################################
push_artifact_repo_to_gitea() {
    local untared_path="$1"
    local artifact_path="$2"
    local repo_name="$3"
    local gitea_service_url="$4"
    
    echo "Extracting artifact: $artifact_path"
    tar -xf "$artifact_path" -C "$untared_path"
    
    local repo_dir="${untared_path}/${repo_name}"
    
    echo "Pushing $repo_name to Gitea..."
    
    # Create Kubernetes job to push repo to Gitea
    cat <<JOBEOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: gitea-init-${repo_name}
  namespace: gitea
  labels:
    managed-by: edge-manageability-framework
spec:
  template:
    spec:
      volumes:
      - name: tea
        hostPath:
          path: /usr/bin/tea
      - name: repo
        hostPath:
          path: ${repo_dir}
      - name: tls
        secret:
          secretName: gitea-tls-certs
      containers:
      - name: alpine
        image: alpine/git:2.49.1
        env:
        - name: GITEA_USERNAME
          valueFrom:
            secretKeyRef:
              name: argocd-gitea-credential
              key: username
        - name: GITEA_PASSWORD
          valueFrom:
            secretKeyRef:
              name: argocd-gitea-credential
              key: password
        command:
        - /bin/sh
        - -c
        args:
        - |
          git config --global credential.helper store
          git config --global user.email \$GITEA_USERNAME@orch-installer.com
          git config --global user.name \$GITEA_USERNAME
          git config --global http.sslCAInfo /usr/local/share/ca-certificates/tls.crt
          git config --global --add safe.directory /repo
          echo "https://\$GITEA_USERNAME:\$GITEA_PASSWORD@${gitea_service_url}" > /root/.git-credentials
          cd /repo
          git init
          git remote add gitea https://${gitea_service_url}/\$GITEA_USERNAME/${repo_name}.git
          git checkout -B main
          git add .
          git commit --allow-empty -m 'Recreate repo from artifact'
          git push --force gitea main
        volumeMounts:
        - name: tea
          mountPath: /usr/bin/tea
        - name: repo
          mountPath: /repo
        - name: tls
          mountPath: /usr/local/share/ca-certificates/
      restartPolicy: Never
  backoffLimit: 5
JOBEOF

    echo "Waiting for gitea-init-${repo_name} job to complete..."
    kubectl wait --for=condition=complete --timeout=300s -n gitea "job/gitea-init-${repo_name}" || {
        echo "Job failed or timed out. Checking job logs:"
        kubectl logs -n gitea "job/gitea-init-${repo_name}" || true
        return 1
    }
    
    echo "Repository $repo_name pushed to Gitea successfully"
}

#######################################
# Get artifact path from tar files location
#######################################
get_artifact_path() {
    local tar_files_location="$1"
    local repo_name="$2"
    local tar_suffix=".tgz"
    
    # Find the tar file matching the repo name
    local artifact_file
    artifact_file=$(find "$tar_files_location" -maxdepth 1 -type f -name "*${repo_name}*${tar_suffix}" | head -1)
    
    if [ -z "$artifact_file" ]; then
        echo "Error: Failed to find *${repo_name}*.tgz artifact in $tar_files_location"
        exit 1
    fi
    
    echo "$artifact_file"
}

#######################################
# Create Gitea credentials secret for ArgoCD
#######################################
create_gitea_creds_secret() {
    local repo_name="$1"
    local namespace="$2"
    local gitea_service_url="$3"
    
    echo "Creating Gitea credentials secret..."
    
    # Fetch Gitea credentials from secret
    local gitea_username_base64
    local gitea_password_base64
    
    gitea_username_base64=$(kubectl get secret argocd-gitea-credential -n gitea -o jsonpath='{.data.username}')
    gitea_password_base64=$(kubectl get secret argocd-gitea-credential -n gitea -o jsonpath='{.data.password}')
    
    # Decode credentials
    local gitea_username
    local gitea_password
    
    gitea_username=$(echo "$gitea_username_base64" | base64 -d)
    gitea_password=$(echo "$gitea_password_base64" | base64 -d)
    
    # Delete existing secret if it exists
    kubectl delete secret "$repo_name" -n argocd 2>/dev/null || true
    
    # Create ArgoCD repository secret
    cat <<SECEOF | kubectl create -f -
apiVersion: v1
kind: Secret
metadata:
  name: ${repo_name}
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: https://${gitea_service_url}/${gitea_username}/${repo_name}
  password: ${gitea_password}
  username: ${gitea_username}
SECEOF

    echo "Gitea credentials secret created"
}

#######################################
# Install root app via Helm
#######################################
install_root_app() {
    local emf_folder="$1"
    local orch_installer_profile="$2"
    local gitea_service_url="$3"
    
    local namespace="onprem"
    
    echo "Installing root-app..."
    
    # Create Gitea credentials secret
    create_gitea_creds_secret "$EDGE_MANAGEABILITY_FRAMEWORK_REPO" "$namespace" "$gitea_service_url"
    
    # Install root-app using Helm
    helm upgrade --install root-app \
        "${emf_folder}/${EDGE_MANAGEABILITY_FRAMEWORK_REPO}/argocd/root-app" \
        -f "${emf_folder}/${EDGE_MANAGEABILITY_FRAMEWORK_REPO}/orch-configs/clusters/${orch_installer_profile}.yaml" \
        -n "$namespace" \
        --create-namespace
    
    echo "root-app installed successfully"
}

#######################################
# Main installation flow
#######################################
main() {
    # Create temporary directory for Edge Manageability Framework
    local emf_folder
    emf_folder=$(mktemp -d -p "$HOME" "${EDGE_MANAGEABILITY_FRAMEWORK_REPO}.XXXXXX")
    
    # Ensure cleanup on exit
    trap "rm -rf '$emf_folder'" EXIT
    
    # Get Gitea service URL
    local gitea_service_url
    gitea_service_url=$(get_gitea_service_url)
    echo "Gitea service URL: $gitea_service_url"
    
    # Get artifact path
    local artifact_path
    artifact_path=$(get_artifact_path "$GIT_REPOS" "$EDGE_MANAGEABILITY_FRAMEWORK_REPO")
    echo "Artifact path: $artifact_path"
    
    # Push artifact repo to Gitea
    push_artifact_repo_to_gitea "$emf_folder" "$artifact_path" "$EDGE_MANAGEABILITY_FRAMEWORK_REPO" "$gitea_service_url"
    
    # Install root app
    install_root_app "$emf_folder" "$ORCH_INSTALLER_PROFILE" "$gitea_service_url"
    
    echo ""
    echo "✓ Installation of orch-installer completed successfully"
    echo ""
}

# Run main function
main
