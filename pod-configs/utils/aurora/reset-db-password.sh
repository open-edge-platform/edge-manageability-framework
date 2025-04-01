#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e -o pipefail

# Usage ./reset-db-password.sh <k8s-namespace> <database> <user>

# New database information
k8sNamespace="$1"
k8sSecretName="$2"
database="$2"
# Random password
newUserPassword=$(openssl rand -base64 32 | tr -dc 'a-zA-Z0-9' | head -c 16)

# Force aws cli to pick up AWS_PROFILE.
# Access key and token could be for a different account hosting the terraform state.
export AWS_ACCESS_KEY_ID=
export AWS_SESSION_TOKEN=

# Get cluster name from Terraform output and set up EKS config base on this
echo "*** Getting cluster name from Terraform..."
tfOutput=$(terraform output -json)
clusterName=$(echo "$tfOutput" | jq .cluster_name.value -r)
if [ -z "$clusterName" ]; then
  echo "Unable to find cluster name from Terraform output"
  exit 1
fi

echo "*** Setting up kubeconfig for cluster $clusterName..."
export KUBECONFIG=$(mktemp)
aws eks update-kubeconfig --name "$clusterName" --kubeconfig "$KUBECONFIG"

# Retrieve AWS Aurora endpoint information
echo
echo "*** Retrieving Aurora endpoint info..."
rdsClusterInfo=$(aws rds describe-db-clusters --db-cluster-identifier "$clusterName-aurora-postgresql" --output json | jq '.DBClusters[0]')
rdsClusterArn=$(echo "$rdsClusterInfo" | jq '.DBClusterArn' -r)
rdsSecretId=$(aws secretsmanager list-secrets --filter Key=tag-key,Values=aws:rds:primaryDBClusterArn --filter Key=tag-value,Values=$rdsClusterArn --output json | jq '.SecretList[0].Name' -r)

pgHost=$(echo "$rdsClusterInfo" | jq -r .Endpoint)
pgPort=5432
pgAdminUser=postgres
pgAdminPassword=$(aws secretsmanager get-secret-value --secret-id "$rdsSecretId" --query SecretString --output text | jq -e --raw-output '.password' )

if [ -z "$pgAdminPassword" ]; then
  echo "Unable to find password for RDS cluster $rdsClusterArn"
  exit 1
fi

# We use a k8s pod to run psql commands. Actual psql invocations are done through
# kubectl exec to avoid exposing the admin password in the deployment resource.
# The pod is automatically removed at the end of the script, or after 120 seconds
# (hard timeout) if the cleanup exit trap should fail for some reason.
echo
echo "*** Creating psql client pod..."
psqlPodName="psql-client-$RANDOM"
kubectl run $psqlPodName --image=postgres:14.6-alpine --restart=Never -i -q --rm -- sleep 120 >/dev/null 2>&1 &

# Make sure the psql pod is removed when the script terminates, even in the event of errors
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

# Get user information from kubernetes secret
databaseUser=$(kubectl get secret -n "$k8sNamespace" "$k8sSecretName" -o json | jq -r '.data.PGUSER' | base64 -d)

if [ -z "$databaseUser" ]; then
  echo "Unable to find database uesr from secret $k8sNamespace/$k8sSecretName"
  exit 1
fi

# Reset password
echo
echo "*** Altering user '$databaseUser'..."
kubectl exec -i $psqlPodName -- env PGPASSWORD="$pgAdminPassword" psql -h "$pgHost" -p "$pgPort" -U "$pgAdminUser" << EOF
  ALTER USER "$databaseUser" WITH PASSWORD '$newUserPassword';
EOF

# Removes old secret and creates a new one with the new password
echo
echo "*** Removing old secret $k8sSecretName in namespace $k8sNamespace (if exists)..."
kubectl delete secret -n "$k8sNamespace" "$k8sSecretName" || true

# Format needs to be compatible with multiple helm charts, hence password showing up twice.
echo
echo "*** Creating new secret $k8sSecretName in namespace $k8sNamespace..."
kubectl create secret generic "$k8sSecretName" \
  --from-literal=PGHOST="$pgHost" \
  --from-literal=PGPORT="$pgPort" \
  --from-literal=PGUSER="$databaseUser" \
  --from-literal=PGPASSWORD="$newUserPassword" \
  --from-literal=password="$newUserPassword" \
  --from-literal=PGDATABASE="$database" \
  --namespace="$k8sNamespace"

echo
echo "*** DONE!"
