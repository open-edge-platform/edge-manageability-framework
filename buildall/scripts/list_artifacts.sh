#!/usr/bin/env bash

# list_artifacts.sh
# list all charts and container images by calling makefile targets in each repo

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

# import env
source env.sh

if [[ $# != 1 ]]; then
  echo "Usage: $0 <repo_path>"
fi

REPO_PATH=$1

if [ ! -d "${REPO_PATH}" ]; then
  echo "'${REPO_PATH}' is not a directory"
  exit 1
fi

if [ ! -f "${REPO_PATH}/Makefile" ]; then
  echo "No Makefile found in '${REPO_PATH}'"
  exit 0
fi

# suppress cd printing directory name
cd "${REPO_PATH}" > /dev/null

# check if helm-list target exists in Makefile
set +eu
HELM_LIST_TARGET=$(grep ^helm-list Makefile)
set -eu

if [ "${HELM_LIST_TARGET}" ]; then
  make helm-list
fi

# check if docker-list target exists in Makefile
set +eu
DOCKER_LIST_TARGET=$(grep ^docker-list Makefile)
set -eu

if [ "${DOCKER_LIST_TARGET}" ]; then
  make docker-list
fi
