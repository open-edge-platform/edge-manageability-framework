#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

source utils.sh

SAVE_DIR=~/pod-configs/SAVEME

load_cluster_state_env
if ! load_scm_auth; then
    exit 1
fi
save_scm_auth
