#!/usr/bin/env bash

# environment file for all scripts

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

# docker configuration
export DOCKER_REGISTRY=open-registry.espdprod.infra-host.com
export DOCKER_REPOSITORY=edge-orch

# Make job parallelism - how many jobs to run at once
export MAKE_JOBS=4

export MAKEFLAGS=--no-print-directory

# this makes Docker buildkit print progress in plain format, easier to capture in a logfile
export BUILDKIT_PROGRESS=plain

# don't color output
export NO_COLOR=1
