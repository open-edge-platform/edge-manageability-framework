#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Configure CoreDNS to rewrite external Keycloak domain to internal Traefik service
# This enables internal pods to reach Keycloak via the external URL

set -e

CLUSTER_DOMAIN="${1:-}"

if [ -z "$CLUSTER_DOMAIN" ]; then
    echo "Error: CLUSTER_DOMAIN is required"
    echo "Usage: $0 <cluster-domain>"
    echo "Example: $0 orch-10-139-218-125.pid.infra-host.com"
    exit 1
fi

KEYCLOAK_DOMAIN="keycloak.${CLUSTER_DOMAIN}"
KEYCLOAK_INTERNAL="keycloak.kind.internal"
TARGET_SERVICE="traefik.orch-gateway.svc.cluster.local"

echo "Configuring CoreDNS rewrite for Keycloak external URL..."
echo "  External Domain: ${KEYCLOAK_DOMAIN}"
echo "  Internal Domain: ${KEYCLOAK_INTERNAL}"
echo "  Target: ${TARGET_SERVICE}"

# Check if rewrite already exists for external domain
REWRITE_COUNT=$(kubectl get configmap coredns -n kube-system -o yaml | grep -c "rewrite.*keycloak" || true)
if [ "$REWRITE_COUNT" -ge 2 ]; then
    echo "✓ CoreDNS rewrite rules already exist, skipping"
    exit 0
fi

echo "Adding CoreDNS rewrite rules..."
# Add external domain rewrite
if ! kubectl get configmap coredns -n kube-system -o yaml | grep -q "rewrite.*${KEYCLOAK_DOMAIN}"; then
    kubectl get configmap coredns -n kube-system -o yaml | \
        sed "/^    \.:53 {$/a\        rewrite name ${KEYCLOAK_DOMAIN} ${TARGET_SERVICE}" | \
        kubectl apply -f -
fi

# Add internal domain rewrite (for backward compatibility with kind.internal references)
if ! kubectl get configmap coredns -n kube-system -o yaml | grep -q "rewrite.*${KEYCLOAK_INTERNAL}"; then
    kubectl get configmap coredns -n kube-system -o yaml | \
        sed "/^    \.:53 {$/a\        rewrite name ${KEYCLOAK_INTERNAL} ${TARGET_SERVICE}" | \
        kubectl apply -f -
fi

echo "Restarting CoreDNS to apply changes..."
kubectl rollout restart deployment/coredns -n kube-system

echo "Waiting for CoreDNS to be ready..."
kubectl rollout status deployment/coredns -n kube-system --timeout=60s

echo "✓ CoreDNS configured successfully"
echo
echo "Verifying configuration..."
REWRITE_COUNT=$(kubectl get configmap coredns -n kube-system -o yaml | grep -c "rewrite.*keycloak" || true)
if [ "$REWRITE_COUNT" -ge 2 ]; then
    echo "✓ Rewrite rules verified in CoreDNS config (found $REWRITE_COUNT rules)"
else
    echo "✗ Warning: Expected 2 rewrite rules but found $REWRITE_COUNT"
    exit 1
fi

echo
echo "CoreDNS configuration complete!"
