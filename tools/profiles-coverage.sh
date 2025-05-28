#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

#
# This script requires an environment variable ORCH_CONFIGS_DIR set to absolute path
# where the source of `orch-configs` repository resides on local filesystem
#
# If invoked without parameters:
# - it displays all profile names and count of clusters where it is used
#
# If invoked with $1 parameter:
# - it displays names of all clusters that use the profile name that matches $1
#

set -euo pipefail

if [[ "${ORCH_CONFIGS_DIR-}" == "" ]]; then
  echo Error: ORCH_CONFIGS_DIR environment variable must be defined!
  exit 1
fi

pushd "$PWD" > /dev/null

cd "$ORCH_CONFIGS_DIR"

if [ -n "${1-}" ]; then
  profile=$1
  echo "$profile" covered in clusters:
  grep -E "^[ ]*- orch-configs/profiles/$profile" ./orch-configs/clusters/*
else
  PROFILES=$(ls profiles)
  # produces output formatted: <times covered>: <profile name>
  # so the script output can be easily sorted with 'sort -n'
  for profile in $PROFILES; do
    count=$(grep -cE "^[ ]*- orch-configs/profiles/$profile" ./orch-configs/clusters/*)
    echo "$count: $profile"  
  done
fi

popd > /dev/null
