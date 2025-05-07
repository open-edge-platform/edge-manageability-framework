#!/usr/bin/env bash

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

# buildall.sh

set -eu -o pipefail

# bring in environmental variables. Where MAKE_JOBS is set
source env.sh

# checkout all repos - populates repos/
make checkout-repos

# create list of all artifacts provided by each repo, both charts and images
make -j "${MAKE_JOBS}" list-artifacts

# create manifest of charts with versions required by edge-manageability-framework repo
make chart-manifest

# create per-repo lists of helm charts to build, and the tag on those repos
make sort-charts

# build all helm charts given the versions
make -j "${MAKE_JOBS}" helm-build

# create manifest of docker images with edge-manageability-framework repo and built charts
make image-manifest

# create per-repo lists of container images to build, and the tags on those repos
make sort-images

# build all imgaes given the versions
make -j "${MAKE_JOBS}" image-build
