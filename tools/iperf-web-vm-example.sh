#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# set -x
set -e
set -o pipefail

VERSION=$1

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
"${SCRIPT_DIR}"/iperf-web-vm-scale-test.sh "${VERSION}" 1 0
