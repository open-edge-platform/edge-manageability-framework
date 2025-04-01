#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

for deb in *_*.deb; do
  if [[ "$deb" =~ (.*)_([0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]+)?)_amd64.deb ]]; then
    mv "$deb" "${BASH_REMATCH[1]}_v${BASH_REMATCH[2]}_amd64.deb"
  fi
done