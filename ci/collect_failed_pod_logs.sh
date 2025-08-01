#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
#
# Script Name: collect_failed_pod_logs.sh
# Description: This script performs the following actions:
#              - Collects a summary of all pods and applications in the cluster.
#              - Identifies pods that are not in a healthy state (i.e., not in Running or Completed phase).
#              - Captures descriptions and logs of only those failed or unhealthy pods.
#              - Saves all collected data in a directory and archives it into a tarball for debugging and support.

set -euo pipefail

OUTPUT_DIR="failed_pod_logs"
ARCHIVE_NAME="$OUTPUT_DIR.tar.gz"

# Clean and prepare directory
rm -rf failed_pod_logs* "$ARCHIVE_NAME"
mkdir -p "$OUTPUT_DIR"

echo "🔍 Collecting cluster pod summary..."
kubectl get pods -A > "$OUTPUT_DIR/summary-pods.txt" 2>&1

echo "📦 Collecting application summary..."
kubectl get application -A > "$OUTPUT_DIR/summary-applications.txt" 2>&1 || \
echo "No 'application' resource found or CRD not installed." >> "$OUTPUT_DIR/summary-applications.txt"

echo "❌ Collecting logs for failed/unhealthy pods only..."

# Get all pods and filter non-Running, non-Completed
kubectl get pods -A --no-headers | \
awk '$4 != "Running" && $4 != "Completed" {print $1, $2}' | \
while read -r NAMESPACE POD; do
    BASE_FILENAME="${NAMESPACE}-${POD}"

    echo "📄 Saving description for pod: $NAMESPACE/$POD"
    kubectl describe pod "$POD" -n "$NAMESPACE" > "$OUTPUT_DIR/${BASE_FILENAME}-describe.txt" 2>&1 || \
    echo "Failed to describe $POD" >> "$OUTPUT_DIR/${BASE_FILENAME}-error.log"

    echo "📋 Saving logs for pod: $NAMESPACE/$POD"
    kubectl logs "$POD" -n "$NAMESPACE" > "$OUTPUT_DIR/${BASE_FILENAME}-logs.txt" 2>&1 || \
    echo "Failed to get logs for $POD" >> "$OUTPUT_DIR/${BASE_FILENAME}-error.log"
done

echo "📦 Creating archive: $ARCHIVE_NAME"
tar -czf "$ARCHIVE_NAME" "$OUTPUT_DIR"

echo "✅ Done. Failed pod logs saved to: $ARCHIVE_NAME"
