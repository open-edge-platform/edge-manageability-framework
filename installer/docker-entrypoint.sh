#!/bin/sh

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e

cp -R /tmp/.aws /root/.aws
chmod -f 700 /root/.aws || true

# clear any proxy settings not specifically imported by the docker run command
unset HTTP_PROXY;unset HTTPS_PROXY;unset NO_PROXY;

exec "$@"
