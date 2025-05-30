#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

#set -x
set -e
set -o pipefail

# Check if ORCH_DEFAULT_PASSWORD is set and not empty
if [ -z "$ORCH_DEFAULT_PASSWORD" ]; then
  echo "Error: ORCH_DEFAULT_PASSWORD is not set or is empty"
  exit 1
fi

USER=edge-operator-example-user
PASSWORD=${ORCH_DEFAULT_PASSWORD}
CLI=catalog

[ -n "$1" ] && ORCHESTRATOR_DOMAIN=$1 || ORCHESTRATOR_DOMAIN=kind.internal

CATALOG_ENDPOINT="https://app-orch.${ORCHESTRATOR_DOMAIN}"
DEPLOYMENT_ENDPOINT="https://app-orch.${ORCHESTRATOR_DOMAIN}"

[ -n "$1" ] && CATALOG_ARGS="--deployment-endpoint ${DEPLOYMENT_ENDPOINT} --catalog-endpoint ${CATALOG_ENDPOINT}" || CATALOG_ARGS=""


${CLI} "${CATALOG_ARGS}" logout
${CLI} "${CATALOG_ARGS}" login --client-id=system-client --trust-cert=true --keycloak "https://keycloak.${ORCHESTRATOR_DOMAIN}/realms/master" ${USER} "${PASSWORD}"

REFRESH_TOKEN=$(${CLI} "${CATALOG_ARGS}" config get refresh-token)
ACCESS_TOKEN=$(curl -s --location --request POST "https://keycloak.${ORCHESTRATOR_DOMAIN}/realms/master/protocol/openid-connect/token" \
    --header 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode 'grant_type=refresh_token' \
    --data-urlencode 'client_id=system-client' \
    --data-urlencode "refresh_token=${REFRESH_TOKEN}" | jq -r ".access_token")
AUTH_HEADER="Authorization: Bearer ${ACCESS_TOKEN}"

curl "https://app-orch.${ORCHESTRATOR_DOMAIN}/deployment.orchestrator.apis/v1/deployments?offset=0&orderBy=displayName%20desc&pageSize=2" --header "${AUTH_HEADER}"