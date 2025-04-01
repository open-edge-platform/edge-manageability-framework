#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

AZUREAD_SECRET_ID=${AZUREAD_SECRET_ID:-"release-service-token"}
AWS_REGION=${AWS_REGION:-"us-west-2"}

if [[ -z "$AZUREAD_REFRESH_TOKEN" ]]; then
  echo "AZUREAD_REFRESH_TOKEN is not set, trying to use AZUREAD_USER, AZUREAD_PASS, AZUREAD_CLIENTID to get refresh token..."
  if [[ -z "$AZUREAD_USER" ]] || [[ -z "$AZUREAD_PASS" ]] || [[ -z "$AZUREAD_CLIENTID" ]] ;then
    echo "Either AZUREAD_USER, AZUREAD_PASS, or AZUREAD_CLIENTID is not set, trying to fetch refresh token from AWS secrets manager..."
    AZUREAD_REFRESH_TOKEN=$(aws secretsmanager get-secret-value --region "$AWS_REGION" --secret-id "$AZUREAD_SECRET_ID" --query SecretString --output text | jq -r .refresh_token)
  else
    AZUREAD_REFRESH_TOKEN=$(curl -s -X POST -d "client_id=$AZUREAD_CLIENTID&scope=openid+offline_access+profile&username=$AZUREAD_USER&password=$AZUREAD_PASS&grant_type=password" https://login.microsoftonline.com/organizations/oauth2/v2.0/token | jq -r .refresh_token)
  fi
fi

if [[ -z "$AZUREAD_REFRESH_TOKEN" ]]; then
  echo "Unable to get refresh token, exiting..."
  exit 1
fi

namespace="orch-secret"

kubectl create namespace $namespace --dry-run=client -o yaml | kubectl apply -f -

kubectl -n $namespace delete secret azure-ad-creds --ignore-not-found

kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: azure-ad-creds
  namespace: $namespace
stringData:
  refresh_token: $AZUREAD_REFRESH_TOKEN
EOF
