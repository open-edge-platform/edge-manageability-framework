#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

for cluster in `kind get clusters`
do
    echo Cluster: $cluster
    kubectl --context kind-$cluster get pods -A
    echo ""
done
