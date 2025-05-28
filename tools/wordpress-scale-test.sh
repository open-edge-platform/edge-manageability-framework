#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Check if ORCH_DEFAULT_PASSWORD is set and not empty
if [ -z "$ORCH_DEFAULT_PASSWORD" ]; then
  echo "Error: ORCH_DEFAULT_PASSWORD is not set or is empty"
  exit 1
fi

VERSION=$1
APPS=$2
SLEEP=$3
CLI=catalog
USER=edge-operator-example-user
PASSWORD=${ORCH_DEFAULT_PASSWORD}

[ -n "$4" ] && ORCHESTRATOR_DOMAIN=$4 || ORCHESTRATOR_DOMAIN=kind.internal

CATALOG_ENDPOINT="https://app-orch.${ORCHESTRATOR_DOMAIN}"
DEPLOYMENT_ENDPOINT="https://app-orch.${ORCHESTRATOR_DOMAIN}"

[ -n "$4" ] && CATALOG_ARGS="--deployment-endpoint ${DEPLOYMENT_ENDPOINT} --catalog-endpoint ${CATALOG_ENDPOINT}" || CATALOG_ARGS=""

${CLI} "${CATALOG_ARGS}" logout
${CLI} "${CATALOG_ARGS}" login --client-id=system-client --trust-cert=true --keycloak https://keycloak."${ORCHESTRATOR_DOMAIN}"/realms/master "${USER}" "${PASSWORD}"
for _ in $(seq 1 "$APPS")
do
    ${CLI} "${CATALOG_ARGS}" create deployment wordpress "${VERSION}" --application-label wordpress.color=blue \
        --application-set wordpress.service.type=NodePort --publisher intel
    sleep "$SLEEP"
done

while (true)
do
    COUNT=$(kubectl -n fleet-default get deployments.app.orchestrator.io --no-headers|grep -c " Running")
    echo "Waiting for $COUNT Deployments"
    if [ "$COUNT" -gt 0 ]
    then
        sleep 5
    else
        COUNT=$(kubectl -n fleet-default get deployments.app.orchestrator.io --no-headers|grep -c " Running")
        echo "All $COUNT Deployments are Running!"
        exit 0
    fi
done
