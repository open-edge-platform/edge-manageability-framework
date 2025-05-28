#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

#
# This script requires an environment variable ORCH_CONFIGS_DIR set to absolute path
# where the source of `orch-configs` repository resides on local filesystem
#
# This script requires an environment variable EDGE_MANAGEABILITY_FRAMEWORK_DIR set to absolute path
# where the source of `edge-manageability-framework` repository resides on local filesystem
#
# The script relies on `render-apps.sh` script to render output for a single cluster
#
# Output produced in OUTPUT_DIR (default dirname 'render-output') contains folders
# per cluster with applications rendered inside.
#
# Recommended usage:
# - render output for `edge-manageability-framework` and `orch-configs` revisions before applied changes (e.g. current main)
# - render output for `edge-manageability-framework` and `orch-configs` revisions after applied changes (e.g. branch version)
# - use diff between folders to inspect the impact of changes across clusters
#

set -euo pipefail

if [[ "${EDGE_MANAGEABILITY_FRAMEWORK_DIR-}" == "" || "${ORCH_CONFIGS_DIR-}" == "" ]]; then
  echo Error: both EDGE_MANAGEABILITY_FRAMEWORK_DIR and ORCH_CONFIGS_DIR environment variables must be defined!
  exit 1
fi

OUTPUT_DIR=render-output
mkdir -p $OUTPUT_DIR
OUTPUT_DIR_PATH=$(realpath $OUTPUT_DIR)

pushd $PWD

cd $EDGE_MANAGEABILITY_FRAMEWORK_DIR
EDGE_MANAGEABILITY_FRAMEWORK_REV=$(git rev-parse --short HEAD)
EDGE_MANAGEABILITY_FRAMEWORK_BRANCH=$(git rev-parse --abbrev-ref HEAD)

cd $ORCH_CONFIGS_DIR
ORCH_CONFIGS_REV=$(git rev-parse --short HEAD)
ORCH_CONFIGS_BRANCH=$(git rev-parse --abbrev-ref HEAD)

CLUSTERS=$(ls $ORCH_CONFIGS_DIR/clusters | sed 's/\.yaml//')
RENDER_OUT_DIR=$OUTPUT_DIR_PATH/$EDGE_MANAGEABILITY_FRAMEWORK_BRANCH-$EDGE_MANAGEABILITY_FRAMEWORK_REV-$ORCH_CONFIGS_BRANCH-$ORCH_CONFIGS_REV
mkdir -p $RENDER_OUT_DIR

for cluster in $CLUSTERS; do
  echo Rendering apps for cluster $cluster...
  mkdir -p $RENDER_OUT_DIR/$cluster
  cd $RENDER_OUT_DIR/$cluster
  $EDGE_MANAGEABILITY_FRAMEWORK_DIR/tools/render-apps.sh $cluster | yq -s '.metadata.name'
done

popd
