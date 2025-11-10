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
TARGET_SERVICE="traefik.orch-gateway.svc.cluster.local"

echo "Configuring CoreDNS rewrite for Keycloak external URL..."
echo "  Domain: ${KEYCLOAK_DOMAIN}"
echo "  Target: ${TARGET_SERVICE}"

# Check if rewrite already exists
if kubectl get configmap coredns -n kube-system -o yaml | grep -q "rewrite.*${KEYCLOAK_DOMAIN}"; then
    echo "✓ CoreDNS rewrite rule already exists, skipping"
    exit 0
fi

echo "Adding CoreDNS rewrite rule..."
kubectl get configmap coredns -n kube-system -o yaml | \
    sed "/^    \.:53 {$/a\        rewrite name ${KEYCLOAK_DOMAIN} ${TARGET_SERVICE}" | \
    kubectl apply -f -

echo "Restarting CoreDNS to apply changes..."
kubectl rollout restart deployment/coredns -n kube-system

echo "Waiting for CoreDNS to be ready..."
kubectl rollout status deployment/coredns -n kube-system --timeout=60s

echo "✓ CoreDNS configured successfully"
echo
echo "Verifying configuration..."
if kubectl get configmap coredns -n kube-system -o yaml | grep -q "rewrite.*${KEYCLOAK_DOMAIN}"; then
    echo "✓ Rewrite rule verified in CoreDNS config"
else
    echo "✗ Warning: Rewrite rule not found in CoreDNS config"
    exit 1
fi

echo
echo "CoreDNS configuration complete!"
