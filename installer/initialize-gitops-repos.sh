#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# shellcheck source=installer/utils.sh
. "${HOME}"/utils.sh

# Consts
export BUCKET_REGION="us-west-2"
SAVE_DIR="${SAVE_DIR:-$HOME/pod-configs/SAVEME}"

load_provision_env

if ! load_scm_auth; then
    exit 1
fi
save_scm_auth

# Clone and init main branch for Code Commit Repos
ORCH_DEPLOY="https://gitea.${CLUSTER_FQDN}/argocd/edge-manageability-framework.git"

mkdir -p ~/src

clone_repo "$ORCH_DEPLOY" edge-manageability-framework

# Extract build contents to repo
cp -R edge-manageability-framework/* src/edge-manageability-framework

commit_repo edge-manageability-framework

# Exit with success code
echo Success. Ready to deploy.
exit 0
