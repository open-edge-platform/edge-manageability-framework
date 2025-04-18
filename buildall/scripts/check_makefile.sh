#!/usr/bin/env bash

# check_makefile.sh
# checks for valid makefile targets

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

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

pushd "${REPO_PATH}"

  # find Dockerfiles in repo
  DOCKERFILES=$(find . -name "Dockerfile*" -print )

  if [ "${DOCKERFILES}" ]; then
    echo "** Has Dockerfiles **"
  fi

  # check if docker-build target exists in Makefile
  set +eu
  DOCKER_BUILD_TARGET=$(grep ^docker-build Makefile)
  set -eu

  if [ "${DOCKER_BUILD_TARGET}" ]; then
    echo "** Has docker-build target **"
  fi

  # find Helm Charts in repo
  HELMCHARTS=$(find . -name "Chart.yaml" -print )

  if [ "${HELMCHARTS}" ]; then
    echo "** Has Helm Charts **"
  fi

  set +eu
  HELM_BUILD_TARGET=$(grep ^helm-build Makefile)
  set -eu

  if [ "${HELM_BUILD_TARGET}" ]; then
    echo "** Has helm-build target **"
  fi

#  HELP=$(make help)
#  echo "${HELP}"

popd
