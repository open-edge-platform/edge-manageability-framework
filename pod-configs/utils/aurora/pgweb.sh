#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "*** Creating pgweb pod..."
pgwebPodName="pgweb-$RANDOM"

# Make sure the psq pod is removed when the script terminates, even in the event of errors
cleanup() {
  if [[ ! -z ${pgwebPodName} ]]; then
    echo "*** Cleaning up pgweb..."
    kubectl delete pod --now --wait=false "$pgwebPodName"
  fi

  echo "*** Cleaning up kubectl port-forward..."
  pkill -f "kubectl port-forward $pgwebPodName"
}
trap cleanup EXIT

if [[ $1 == "--admin" && $# == 3 ]]; then
  set -x
  echo "*** Retrieving Aurora admin endpoint info from AWS..."
  clusterName="$2"
  region="$3"
  PGPORT=5432
  PGUSER=postgres
  PGHOST=$(aws rds describe-db-clusters --db-cluster-identifier "$clusterName-aurora-postgresql" --region "$region" | jq -r '.DBClusters[0].Endpoint')
  awsMasterPasswordSecretName=$(aws secretsmanager list-secrets --filters "Key=tag-key,Values=environment" "Key=tag-value,Values=$clusterName" "Key=owning-service,Values=rds" | jq -r '.SecretList[0].Name')

  # Force aws cli to pick up AWS_PROFILE.
  # Access key and token could be for a different account hosting the terraform state.
  export AWS_ACCESS_KEY_ID=
  export AWS_SESSION_TOKEN=
  PGPASSWORD=$(aws secretsmanager get-secret-value --secret-id "$awsMasterPasswordSecretName" --query SecretString --output text | jq -e --raw-output '.password' )
  PGDATABASE=postgres
elif [[ $# == 2 ]]; then
  # Namespace and secret name are passed via command line
  namespace="$1"
  secretName="$2"

  # Retrieve secret values
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
else
  echo "Usage: $0 --admin <CLUSTER_NAME> <AWS_REGION>"
  echo "Usage: $0 <NAMESPACE> <SECRET_NAME>"
  echo -e "  CLUSTER_NAME    The cluster name (e.g. demo1)"
  echo -e "  AWS_REGION      The AWS region of the deployment (e.g. us-west-2)"
  echo -e "  NAMESPACE       Kubernetes namespace (e.g. orch-platform)"
  echo -e "  SECRET_NAME     Secret name without -aurora-postgresql suffix (e.g. vault)"
  exit 1
fi

# URL encode in case there are special characters
PGHOST=$(echo "$PGHOST" | tr -d '\n' | jq -sRr @uri)
PGPORT=$(echo "$PGPORT" | tr -d '\n' | jq -sRr @uri)
PGUSER=$(echo "$PGUSER" | tr -d '\n' | jq -sRr @uri)
PGPASSWORD=$(echo "$PGPASSWORD" | tr -d '\n' | jq -sRr @uri)
PGDATABASE=$(echo "$PGDATABASE" | tr -d '\n' | jq -sRr @uri)

kubectl run $pgwebPodName --image=sosedoff/pgweb --port=8081 --env="PGWEB_DATABASE_URL=postgres://$PGUSER:$PGPASSWORD@$PGHOST:$PGPORT/$PGDATABASE?sslmode=require" -i -q --rm >/dev/null 2>&1 &

# Wait for the psql pod to be running
sleep 1
while [[ $(kubectl get pod "$pgwebPodName" -o jsonpath='{.status.phase}') != "Running" ]]; do
  echo "*** Waiting for $pgwebPodName to be running..."
  sleep 1
done

echo -e "\033[32m*** Start kubectl port-forward. pgweb will be available at http://localhost:8081\033[m"

kubectl port-forward $pgwebPodName 8081