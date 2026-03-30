#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
#
# Creates tls-autocert secret from tls-orch in orch-gateway namespace.
# On-prem self-signed deployments create tls-orch but cert-synchronizer
# expects tls-autocert (which is only created by platform-autocert on AWS).

NS="orch-gateway"
SOURCE="tls-orch"
TARGET="tls-autocert"

if kubectl get secret "$TARGET" -n "$NS" &>/dev/null; then
  echo "Secret $TARGET already exists in $NS — skipping"
  exit 0
fi

if ! kubectl get secret "$SOURCE" -n "$NS" &>/dev/null; then
  echo "Source secret $SOURCE not found in $NS — skipping"
  exit 0
fi

echo "Creating $TARGET from $SOURCE in $NS..."
kubectl get secret "$SOURCE" -n "$NS" -o json \
  | jq --arg name "$TARGET" '.metadata.name = $name | del(.metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp, .metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"])' \
  | kubectl apply -f - -n "$NS"

echo "Secret $TARGET created in $NS"
