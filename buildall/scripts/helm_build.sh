#!/usr/bin/env bash

# helm_build.sh
# builds docker containers by invoking make helm-build

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

set -xeu -o pipefail

if [[ $# != 1 ]]; then
  echo "Usage: $0 <repo>"
fi

REPO=$1

# bring in env vars
source env.sh

REPO_PATH="repos/${REPO}"

if [ ! -d "${REPO_PATH}" ]; then
  echo "'${REPO_PATH}' is not a directory"
  exit 1
fi

# this file won't exist if no helm charts are built by the repo
TAGS_PATH="scratch/htags_${REPO}"

if [ ! -f "${TAGS_PATH}" ]; then
  echo "'${TAGS_PATH}' does not exist, so repo does not create any helm charts"
  exit 0
fi

TAG_FILE=$(cat "${TAGS_PATH}")

pushd "${REPO_PATH}"
  for tline in $TAG_FILE; do

    tag=$(cut -d '|' -f 1 <<< "$tline")
    outDir=$(cut -d '|' -f 2 <<< "$tline")

    if [ "$tag" == "$outDir" ]
    then
      outDir="."
    fi

    echo "*** Building helm chart for tag: '${tag}' placed in: '${outDir}' ***"

    git switch --detach "${tag}"

    make helm-build

    cp "${outDir}"/*.tgz ../../charts
  done
popd
