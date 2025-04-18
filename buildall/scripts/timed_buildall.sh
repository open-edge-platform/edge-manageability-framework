#!/usr/bin/env bash

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

# buildall.sh

set -xeu -o pipefail

# bring in environmental variables. Where MAKE_JOBS is set
source env.sh

# otherwise the shell builtin time may be used
TIMECMD="command time"
TIMEFILE=scratch/times_$(date -u +"%Y%m%d_%H%M%S")

START_TS=$(date +"%s")

touch "${TIMEFILE}"

# checkout all repos - populates repos/
echo "checkout-repos" >> "${TIMEFILE}"
${TIMECMD} -a -o "${TIMEFILE}" make checkout-repos

# create list of all artifacts provided by each repo, both charts and images
echo "list-artifacts" >> "${TIMEFILE}"
${TIMECMD} -a -o "${TIMEFILE}" make -j "${MAKE_JOBS}" list-artifacts

# create manifest of charts with versions required by edge-manageability-framework repo
echo "chart-manifest" >> "${TIMEFILE}"
${TIMECMD} -a -o "${TIMEFILE}" make chart-manifest

# create per-repo lists of helm charts to build, and the tag on those repos
echo "sort-charts" >> "${TIMEFILE}"
${TIMECMD} -a -o "${TIMEFILE}" make sort-charts

# build all helm charts given the versions
echo "helm-build" >> "${TIMEFILE}"
${TIMECMD} -a -o "${TIMEFILE}" make -j "${MAKE_JOBS}" helm-build

# create manifest of docker images with edge-manageability-framework repo and built charts
echo "image-manifest" >> "${TIMEFILE}"
${TIMECMD} -a -o "${TIMEFILE}" make image-manifest

# create per-repo lists of container images to build, and the tags on those repos
echo "sort-images" >> "${TIMEFILE}"
${TIMECMD} -a -o "${TIMEFILE}" time make sort-images

# build all imgaes given the versions
echo "image-build" >> "${TIMEFILE}"
${TIMECMD} -a -o "${TIMEFILE}" time make -j "${MAKE_JOBS}" image-build

END_TS=$(date +"%s")
ELAPSED=$(( END_TS - START_TS ))
echo "Total elapsed time: ${ELAPSED} seconds" | tee -a "${TIMEFILE}"
