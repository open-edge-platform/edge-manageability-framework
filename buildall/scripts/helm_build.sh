#!/usr/bin/env bash

# helm_build.sh
# builds docker containers by invoking make helm-build

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

if [[ $# != 2 ]]; then
  echo "Usage: $0 <repo> <log_file>"
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

    git switch --detach "${tag}"

    # check if helm-build target exists in Makefile
    set +eu
    HELM_BUILD_TARGET=$(grep ^helm-build Makefile)
    set -eu

    if [ ! "${HELM_BUILD_TARGET}" ]; then
      echo "### Copying Makefile from main branch as helm-build target doesn't exist ###"
      git checkout main Makefile
    fi

    echo "### Starting make helm-build in: '${REPO}', tag: '${tag}' ###"

    make helm-build >> "${LOGFILE}" 2>&1

    mv "${outDir}"/*.tgz ../../charts

    # clean up repo
    git checkout HEAD .

    echo "### Finished make helm-build in: '${REPO}', tag: '${tag}', output in: '${outDir}' ###"

  done
popd

END_TS=$(date +"%s")
ELAPSED=$(( END_TS - START_TS ))

echo "### helm-build took ${ELAPSED} seconds in repo: '${REPO}' ###"
