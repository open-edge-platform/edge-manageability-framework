#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2024 Intel Corp.
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Shows how all the Applications will be rendered for a specific cluster definition.
# Example: tools/render-apps.sh dev-coder-minimal

set -x
set -e
set -o pipefail

CLUSTER=$1

EDGE_MANAGEABILITY_FRAMEWORK_DIR=${EDGE_MANAGEABILITY_FRAMEWORK_DIR:-~/edge-manageability-framework}
ORCH_CONFIGS_DIR=${ORCH_CONFIGS_DIR:-~/orch-configs}
CLUSTER_PATH=${ORCH_CONFIGS_DIR}/clusters
CLUSTER_DEF=${CLUSTER_PATH}/${CLUSTER}.yaml

VALUES_FILES=$(yq '.root.clusterValues[]' "${CLUSTER_DEF}" | awk "{print \"-f ${ORCH_CONFIGS_DIR}/\" \$1}" | xargs)

helm template "${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/argocd/applications" "${VALUES_FILES}"
helm template "${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/argocd-internal/applications" "${VALUES_FILES}"
