#!/usr/bin/env bash

# environment file for all scripts

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

# docker configuration
export DOCKER_REGISTRY=open-registry.espdprod.infra-host.com
export DOCKER_REPOSITORY=edge-orch

# non-standard docker registry/repository env var configuration
# used in: app-orch-catalog repo
export PUBLISH_REGISTRY=${DOCKER_REGISTRY}
# used in: cluster, o11y repos
export REGISTRY=${DOCKER_REGISTRY}

# Make job parallelism - how many jobs to run at once
export MAKE_JOBS=4

# avoid printing directory entry/exit in recent (v4.x) versions of make
export MAKEFLAGS=--no-print-directory

# this makes Docker buildkit print progress in plain format, easier to capture in a logfile
export BUILDKIT_PROGRESS=plain

# Set this to any value (such as 'Y') if you have limited space and want
# to prune the docker buildx cache after every repo
export DOCKER_PRUNE=

# don't color output
export NO_COLOR=1

# avoid checking for tool versions/presence
export TOOL_VERSION_CHECK=0

# Github Token - used to build certain images
# Obtain from: https://github.com/settings/tokens
export GITHUB_TOKEN=
