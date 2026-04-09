#!/bin/bash
# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
# Adopt existing metrics-server resources into Helm management.
# K3s bundles metrics-server by default without Helm labels, which causes
# "invalid ownership metadata" errors when helmfile tries to install the chart.
# This hook labels/annotates all known metrics-server resources so Helm can adopt them.
set -euo pipefail
RELEASE="k8s-metrics-server"
NAMESPACE="kube-system"
adopt_resource() {
  local resource="$1"
  local ns_flag="${2:-}"
  if kubectl $ns_flag get "$resource" &>/dev/null; then
    kubectl $ns_flag annotate "$resource" \
      meta.helm.sh/release-name="$RELEASE" \
      meta.helm.sh/release-namespace="$NAMESPACE" \
      --overwrite 2>/dev/null || true
    kubectl $ns_flag label "$resource" \
      app.kubernetes.io/managed-by=Helm \
      --overwrite 2>/dev/null || true
  fi
}
# Namespace-scoped resources
for res in serviceaccount/metrics-server deployment.apps/metrics-server service/metrics-server; do
  adopt_resource "$res" "-n $NAMESPACE"
done
# RoleBinding
adopt_resource "rolebinding/metrics-server-auth-reader" "-n $NAMESPACE"
# Cluster-scoped resources
for res in \
  "clusterrole/system:metrics-server" \
  "clusterrole/system:aggregated-metrics-reader" \
  "clusterrolebinding/metrics-server:system:auth-delegator" \
  "clusterrolebinding/system:metrics-server" \
  "apiservice/v1beta1.metrics.k8s.io"; do
  adopt_resource "$res" ""
done
