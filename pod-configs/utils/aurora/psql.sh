#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e

psqlInit() {
  # We use a k8s pod to run psql commands
  echo "*** Creating psql client pod..."
  psqlPodName="psql-client-$RANDOM"
  kubectl run $psqlPodName --image=postgres:14.6-alpine --restart=Never -i -q --rm -- sleep infinity >/dev/null 2>&1 &

  # Wait for the psql pod to be running
  sleep 1
  while [[ $(kubectl get pod "$psqlPodName" -o jsonpath='{.status.phase}') != "Running" ]]; do
    echo "*** Waiting for $psqlPodName to be running..."
    sleep 1
  done
}

# Make sure the psq pod is removed when the script terminates, even in the event of errors
cleanup() {
  if [[ ! -z ${psqlPodName} ]]; then
    echo "*** Cleaning up psql client..."
    kubectl delete pod --now --wait=false "$psqlPodName"
  fi
}
trap cleanup EXIT

if [[ $1 == "--admin" && $# == 1 ]]; then
  psqlInit
  echo
  echo "*** Retrieving Aurora admin endpoint info from Terraform..."
  tfOutput=$(terraform output -json)
  awsRegion=$(echo "$tfOutput" | jq -e --raw-output .aurora_postgresql_region.value)
  PGHOST=$(echo "$tfOutput" | jq -e --raw-output .aurora_postgresql_host.value)
  PGPORT=5432
  PGUSER=postgres
  awsMasterPasswordSecretName=$(echo "$tfOutput" | jq -e --raw-output .aurora_postgresql_master_password_secret_name.value)

  # Force aws cli to pick up AWS_PROFILE.
  # Access key and token could be for a different account hosting the terraform state.
  export AWS_ACCESS_KEY_ID=
  export AWS_SESSION_TOKEN=
  PGPASSWORD=$(aws secretsmanager get-secret-value --region "$awsRegion" --secret-id "$awsMasterPasswordSecretName" --query SecretString --output text | jq -e --raw-output '.password' )

  kubectl exec -it $psqlPodName -- env PGPASSWORD="$PGPASSWORD" psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER"
elif [[ $# == 2 ]]; then
  psqlInit
  # Namespace and secret name are passed via command line
  namespace="$1"
  secretName="$2"

  # Retrieve secret values
  echo
  echo "*** Retrieving secret $namespace/$secretName..."
  secretJson=$(kubectl get secret "$secretName" --namespace="$namespace" -o jsonpath="{.data}")
  PGHOST=$(echo "$secretJson" | jq -r '.PGHOST' | base64 --decode)
  PGPORT=$(echo "$secretJson" | jq -r '.PGPORT' | base64 --decode)
  PGUSER=$(echo "$secretJson" | jq -r '.PGUSER' | base64 --decode)
  PGPASSWORD=$(echo "$secretJson" | jq -r '.PGPASSWORD' | base64 --decode)
  PGDATABASE=$(echo "$secretJson" | jq -r '.PGDATABASE' | base64 --decode)

  echo "  - PGHOST=$PGHOST"
  echo "  - PGPORT=$PGPORT"
  echo "  - PGUSER=$PGUSER"
  echo "  - PGPASSWORD=(${#PGPASSWORD} characters)"
  echo "  - PGDATABASE=$PGDATABASE"
  echo

  kubectl exec -it $psqlPodName -- env PGPASSWORD="$PGPASSWORD" psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE"
else
  echo "Usage: $0 --admin"
  echo "Usage: $0 <NAMESPACE> <SECRET_NAME>"
  echo -e "  NAMESPACE    Kubernetes namespace (e.g. orch-platform)"
  echo -e "  SECRET_NAME  Secret name without -aurora-postgresql suffix (e.g. vault)"
  exit 1
fi