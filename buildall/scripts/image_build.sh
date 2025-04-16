#!/usr/bin/env bash

# image_build.sh
# builds container images by invoking make docker-build

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

if [[ $# != 2 ]]; then
  echo "Usage: $0 <repo_path> <log_file>"
fi

REPO=$1

# touch and find the full path to the logfile
touch "$2"
LOGFILE=$(realpath "$2")

# bring in env vars
source env.sh

START_TS=$(date +"%s")

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

    echo "*** Starting docker build in: '${REPO}', tag: '${tag}', target: '${target}' ***"

    git switch --detach "${tag}"

    # this is a hack deal with issues in the docker-build target in EIM repos
    # regex matches all the end-of-string numbers/dots
    if [[ "${tag}" =~ ([0-9.]+)$ ]]
    then
      export DOCKER_VERSION="${BASH_REMATCH[1]}"
    else
      echo "Invalid tag format '${tag}'"
      exit 1
    fi

    make "${target}" >> "${LOGFILE}" 2>&1

    echo "*** Finished docker build in repo: '${REPO}', tag: '${tag}', target: '${target}' ***"
  done
popd

END_TS=$(date +"%s")
ELAPSED=$(( END_TS - START_TS ))

echo "### Docker build in: '${REPO}' took ${ELAPSED} seconds ###"
