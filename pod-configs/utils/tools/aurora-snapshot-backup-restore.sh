#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

DATE=$(date '+%Y%m%d_%H%M')

# Define parameters
ACCOUNT_NUMBER=$1
CLUSTER_NAME=$2
ACTION=$3
REGION=$4
SNAPSHOT_IDENTIFIER=$5

# Validate parameters
if [ -z "$ACCOUNT_NUMBER" ] || [ -z "$CLUSTER_NAME" ] || [ -z "$ACTION" ] || [ -z "$REGION" ]; then
    echo "Usage: $0 <ACCOUNT_NUMBER> <CLUSTER_NAME> <ACTION> <REGION> [SNAPSHOT_IDENTIFIER]"
    echo "Parameters:"
    echo "  ACCOUNT_NUMBER: AWS account number (e.g., 123456789012)"
    echo "  CLUSTER_NAME: The name of your DB cluster"
    echo "  ACTION: 'backup' to create a snapshot or 'restore' to restore a DB cluster from a snapshot"
    echo "  REGION: The AWS region (e.g., us-west-2)"
    echo "  SNAPSHOT_IDENTIFIER: (Optional for 'backup') The name of the snapshot to restore from (e.g., db-cluster-name-snapshot-20230901_1100)"
    echo ""
    echo "Examples:"
    echo "  Create a snapshot: $0 123456789012 db-cluster-name backup us-west-2"
    echo "  Restore from a snapshot: $0 123456789012 db-cluster-name restore us-west-2 db-cluster-name-snapshot-20230901_1100"
    echo "  List available snapshots: $0 123456789012 db-cluster-name list us-west-2"
    exit 1
fi

# Validate action
if [ "${ACTION}" != "backup" ] && [ "${ACTION}" != "restore" ] && [ "${ACTION}" != "list" ]; then
    echo "Invalid action. Use 'backup', 'restore' or 'list'."
    exit 1
fi

# Check if cluster exists
CLUSTER_EXIST=$(aws rds describe-db-clusters --db-cluster-identifier "${CLUSTER_NAME}" --region "${REGION}" 2>/dev/null)
if [ -z "${CLUSTER_EXIST}" ]; then
    echo "Cluster ${CLUSTER_NAME} does not exist in region ${REGION}."
    exit 1
fi

if [ "${ACTION}" == "backup" ]; then
    # Create a sanitized snapshot name
    SNAPSHOT_NAME=$(echo "${CLUSTER_NAME}-${ACCOUNT_NUMBER}-${DATE}" | tr -cd 'a-zA-Z0-9-')
    SNAPSHOT_NAME=$(echo "${SNAPSHOT_NAME}" | tr '[:upper:]' '[:lower:]')
    SNAPSHOT_NAME="${SNAPSHOT_NAME:0:63}"

    echo "Creating snapshot ${SNAPSHOT_NAME} for cluster ${CLUSTER_NAME}..."

    # Create the snapshot
    aws rds create-db-cluster-snapshot --db-cluster-snapshot-identifier "${SNAPSHOT_NAME}" --db-cluster-identifier "${CLUSTER_NAME}" --region "${REGION}"

    echo "Waiting for snapshot ${SNAPSHOT_NAME} to become available..."
    while true; do
        STATUS=$(aws rds describe-db-cluster-snapshots --db-cluster-snapshot-identifier "${SNAPSHOT_NAME}" --region "${REGION}" --query "DBClusterSnapshots[0].Status" --output text)
        if [ "${STATUS}" == "available" ]; then
            echo "Snapshot ${SNAPSHOT_NAME} is now available."
            break
        elif [ "${STATUS}" == "failed" ]; then
            echo "Snapshot ${SNAPSHOT_NAME} creation failed."
            exit 1
        else
            echo "Snapshot status: ${STATUS}. Waiting..."
            sleep 30
        fi
    done
fi

if [ "${ACTION}" == "restore" ]; then
    if [ -z "${SNAPSHOT_NAME}" ]; then
        echo "Usage for restore: $0 <ACCOUNT_NUMBER> restore <REGION> <CLUSTER_NAME> <SNAPSHOT_NAME>"
        exit 1
    fi

    # Restore the snapshot
    STATUS=$(aws rds describe-db-cluster-snapshots --db-cluster-snapshot-identifier "${SNAPSHOT_NAME}" --region "${REGION}" --query "DBClusterSnapshots[0].Status" --output text)
    if [ "${STATUS}" == "available" ]; then
        echo "Snapshot ${SNAPSHOT_NAME} is available to restore. Proceeding..."

        aws rds restore-db-cluster-from-snapshot --snapshot-identifier "${SNAPSHOT_NAME}" --db-cluster-identifier "${CLUSTER_NAME}" --region "${REGION}" --engine aurora-postgresql
        while true; do
            CLUSTER_STATUS=$(aws rds describe-db-clusters --db-cluster-identifier "${CLUSTER_NAME}" --region "${REGION}" --query "DBClusters[0].Status" --output text)
            if [ "${CLUSTER_STATUS}" == "available"]; then
                echo "Snapshot has been successfully restored."
                break
            elif [ "${CLUSTER_STATUS}" == "deleting" ] || [ "${CLUSTER_STATUS}" == "migration-failed" ] || [ "${CLUSTER_STATUS}" == "stopped" ] || [ "${CLUSTER_STATUS}" == "stopping" ]; then
                echo "Restore from ${SNAPSHOT_NAME} of DB cluster ${CLUSTER_NAME} FAILED! Exiting"
                exit 1
            else
                echo "Snapshot still being restored. Waiting..."
                sleep 30
            fi
        done
    else
        echo "An error occured. Exiting..."
        exit 1
    fi
fi

if [ "${ACTION}" == "list" ]; then
    # List all available snapshots for the cluster
    echo "Listing available snapshots for cluster: ${CLUSTER_NAME} in region: ${REGION}"
    aws rds describe-db-cluster-snapshots \
        --db-cluster-identifier "${CLUSTER_NAME}" \
        --region "${REGION}" \
        --query 'DBClusterSnapshots[*].{Snapshot:DBClusterSnapshotIdentifier,Status:Status,Created:SnapshotCreateTime}' \
        --output table
    exit 0
fi
