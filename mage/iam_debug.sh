#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

DT=$(date -u +"%Y%m%dt%H%M")

mkdir -p "${DT}_mt_debug"

kubectl get orgs.org.infra-host.com -o json > "${DT}_mt_debug/orgs.json"
kubectl get orgactivewatchers.orgactivewatcher.infra-host.com -o json > "${DT}_mt_debug/orgs_aw.json"
kubectl get projects.project.infra-host.com -o json > "${DT}_mt_debug/projects.json"
kubectl get projectactivewatchers.projectactivewatcher.infra-host.com -o json > "${DT}_mt_debug/projects_aw.json"

TM_PODNAME=$(kubectl get pods -n orch-iam | cut -d " " -f 1 | grep tenancy-manager)

kubectl logs -n orch-iam "${TM_PODNAME}" > "${DT}_mt_debug/tenancy-manager.logs"

tar -czvf "${DT}_mt_debug.tgz" "${DT}_mt_debug"
