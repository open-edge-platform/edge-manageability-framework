#!/usr/bin/env bash

# image_build.sh
# builds container images by invoking make docker-build

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

if [[ $# != 1 ]]; then
  echo "Usage: $0 <repo_path>"
fi

REPO=$1

# bring in env vars
source env.sh

REPO_PATH="repos/${REPO}"

if [ ! -d "${REPO_PATH}" ]; then
  echo "'${REPO_PATH}' is not a directory"
  exit 1
fi

# this file won't exist if no container images are built by the repo
TAGS_PATH="scratch/itags_${REPO}"

if [ ! -f "${TAGS_PATH}" ]; then
  echo "'${TAGS_PATH}' does not exist, so repo does not create any container images"
  exit 0
fi

TAG_FILE=$(cat "${TAGS_PATH}")

pushd "${REPO_PATH}"
  for tline in $TAG_FILE; do

    tag=$(cut -d '|' -f 1 <<< "$tline")
    target=$(cut -d '|' -f 2 <<< "$tline")

    if [ "$tag" == "$target" ]
    then
      target='docker-build'
    fi

    echo "*** Building docker image for tag: '${tag}' with target ${target} ***"

    git switch --detach "${tag}"

    # this is a hack deal with issues in the docker-build target in EIM repos
    # regex matches all the end-of-string numbers/dots
    if [[ "${tag}" =~ ([0-9.]+)$ ]]
    then
      echo "Bare Version: ${BASH_REMATCH[1]}"
      export DOCKER_VERSION="${BASH_REMATCH[1]}"
    else
      echo "Invalid tag format '${tag}'"
      exit 1
    fi

    make "${target}"
  done
popd
