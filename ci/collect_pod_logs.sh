#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
#
# Script Name: collect_pod_logs.sh
# Description: This script performs the following actions:
#              - Collects a summary of all pods and applications in the cluster.
#              - Separates pods based on their health status.
#              - Captures descriptions and logs for both healthy and unhealthy pods.
#              - Organizes logs into separate folders.
#              - Archives the results for troubleshooting or support use.

set -euo pipefail

OUTPUT_DIR="pod_logs"
ARCHIVE_NAME="$OUTPUT_DIR.tar.gz"
SUCCESS_DIR="$OUTPUT_DIR/success"
FAILED_DIR="$OUTPUT_DIR/failed"

# Clean up and create directories
rm -rf "$OUTPUT_DIR" "$ARCHIVE_NAME"
mkdir -p "$SUCCESS_DIR" "$FAILED_DIR"

echo "🔍 Collecting cluster pod summary..."
kubectl get pods -A > "$OUTPUT_DIR/summary-pods.txt" 2>&1

echo "📦 Collecting application summary..."
kubectl get application -A > "$OUTPUT_DIR/summary-applications.txt" 2>&1 || \
echo "No 'application' resource found or CRD not installed." >> "$OUTPUT_DIR/summary-applications.txt"

echo "🔎 Collecting logs and descriptions for all pods..."

kubectl get pods -A --no-headers | \
awk '{print $1, $2, $4}' | \
while read -r NAMESPACE POD STATUS; do
    BASE_FILENAME="${NAMESPACE}-${POD}"
    TARGET_DIR="$FAILED_DIR"

    if [[ "$STATUS" == "Running" || "$STATUS" == "Completed" ]]; then
        TARGET_DIR="$SUCCESS_DIR"
    fi

    echo "📄 Saving description for pod: $NAMESPACE/$POD"
    kubectl describe pod "$POD" -n "$NAMESPACE" > "$TARGET_DIR/${BASE_FILENAME}-describe.txt" 2>&1 || \
    echo "Failed to describe $POD" >> "$TARGET_DIR/${BASE_FILENAME}-error.log"

    echo "📋 Saving logs for pod: $NAMESPACE/$POD"
    kubectl logs "$POD" -n "$NAMESPACE" > "$TARGET_DIR/${BASE_FILENAME}-logs.txt" 2>&1 || \
    echo "Failed to get logs for $POD" >> "$TARGET_DIR/${BASE_FILENAME}-error.log"
done

echo "📦 Creating archive: $ARCHIVE_NAME"
tar -czf "$ARCHIVE_NAME" "$OUTPUT_DIR"

echo "✅ Done. Logs saved and archived at: $ARCHIVE_NAME"
