#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Check if ORCH_DEFAULT_PASSWORD is set and not empty
if [ -z "$ORCH_DEFAULT_PASSWORD" ]; then
  echo "Error: ORCH_DEFAULT_PASSWORD is not set or is empty"
  exit 1
fi

CLI=catalog
USER=edge-operator-example-user
PASSWORD=${ORCH_DEFAULT_PASSWORD}

[ ! -z "$1" ] && ORCHESTRATOR_DOMAIN=$1 || ORCHESTRATOR_DOMAIN=kind.internal

CATALOG_ENDPOINT="https://app-orch.${ORCHESTRATOR_DOMAIN}"
DEPLOYMENT_ENDPOINT="https://app-orch.${ORCHESTRATOR_DOMAIN}"

[ ! -z "$1" ] && CATALOG_ARGS="--deployment-endpoint ${DEPLOYMENT_ENDPOINT} --catalog-endpoint ${CATALOG_ENDPOINT}" || CATALOG_ARGS=""

${CLI} ${CATALOG_ARGS} logout
${CLI} ${CATALOG_ARGS} login --client-id=system-client --trust-cert=true --keycloak https://keycloak.${ORCHESTRATOR_DOMAIN}/realms/master ${USER} ${PASSWORD}
${CLI} ${CATALOG_ARGS} list deployments|tail -n +2|cut -f 1 -d ' '|xargs -n 1 ${CLI} delete deployment