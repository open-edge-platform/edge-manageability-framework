#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e -o pipefail

# Namespace and secret name are passed via command line
namespace="$1"
secretName="$2"
database="$3"

# Get cluster name from Terraform output and set up EKS config base on this
echo "*** Getting cluster name from Terraform..."
clusterName=$(terraform output -json | jq .cluster_name.value -r)

if [ -z "$clusterName" ]; then
  echo "Unable to find cluster name from Terraform output"
  exit 1
fi

echo "*** Setting up kubeconfig for cluster $clusterName..."
export KUBECONFIG=$(mktemp)
aws eks update-kubeconfig --name "$clusterName" --kubeconfig "$KUBECONFIG"

# Retrieve secret values using kubectl
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

if [ -n "$database" ]
then
  echo
  echo "*** Performing test on database '$database'"
  PGDATABASE=$database
fi

# We use a k8s pod to run psql commands
echo
echo "*** Creating psql client pod..."
psqlPodName="psql-client-$RANDOM"
kubectl run $psqlPodName --image=postgres:14.6-alpine --restart=Never -i -q --rm -- sleep 120 >/dev/null 2>&1 &

# Make sure the psq pod is removed when the script terminates, even in the event of errors
cleanup() {
  echo "*** Cleaning up psql client..."
  kubectl delete pod --now --wait=false "$psqlPodName"
  if [ -f "$KUBECONFIG" ]; then
    rm -rf "$KUBECONFIG"
  fi
}
trap cleanup EXIT

# Wait for the psql pod to be running
sleep 1
while [[ $(kubectl get pod "$psqlPodName" -o jsonpath='{.status.phase}') != "Running" ]]; do
  echo "*** Waiting for $psqlPodName to be running..."
  sleep 1
done

psqlAlias="kubectl exec -i $psqlPodName -- env PGPASSWORD=$PGPASSWORD psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDATABASE"

echo
echo "*** Checking write permissions..."
$psqlAlias -c "CREATE TABLE test_table (id serial primary key, data text);"
$psqlAlias -c "INSERT INTO test_table (data) VALUES ('test');"

echo
echo "*** Checking read permissions..."
$psqlAlias -c "SELECT * FROM test_table;"

echo
echo "*** Cleaning up tables..."
$psqlAlias -c "DROP TABLE test_table"

echo
echo "*** All tests passed"