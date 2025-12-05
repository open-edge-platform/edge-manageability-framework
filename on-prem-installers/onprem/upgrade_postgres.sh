#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

postgres_namespace=orch-database
POSTGRES_LOCAL_BACKUP_PATH="./"
local_backup_file="${postgres_namespace}_backup.sql"
local_backup_path="${POSTGRES_LOCAL_BACKUP_PATH}${local_backup_file}"
POSTGRES_USERNAME="postgres"
application_namespace=onprem

# Detect upgrade path by checking which PostgreSQL pod exists in orch-database namespace
# If postgresql-cluster-1 exists, this is upgrade FROM 2025.02 to latest
# If postgresql-0 exists in orch-database, this is upgrade FROM 3.1.3 to latest
# Note: gitea-postgresql-0 in gitea namespace is ignored
get_postgres_pod() {
  # Get all pod names in orch-database namespace
  local pod_names=$(kubectl get pods -n orch-database -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
  
  # Check for postgresql-cluster-1 (2025.02 CloudNativePG)
  if echo "$pod_names" | grep -qw "postgresql-cluster-1"; then
    echo "postgresql-cluster-1"
    return
  fi
  
  # Check for postgresql-0 (3.1.3)
  if echo "$pod_names" | grep -qw "postgresql-0"; then
    echo "postgresql-0"
    return
  fi
  
  # Default fallback
  echo "postgresql-0"
}

# Detect upgrade path based on pod name
detect_upgrade_path() {
  local pod=$(get_postgres_pod)
  if [[ "$pod" == "postgresql-cluster-1" ]]; then
    UPGRADE_FROM_3_1_X="false"
    echo "Detected upgrade FROM v2025.02 (pod: $pod)"
  elif [[ "$pod" == "postgresql-0" ]]; then
    UPGRADE_FROM_3_1_X="true"
    echo "Detected upgrade FROM v3.1.3 (pod: $pod)"
  else
    echo "Unknown PostgreSQL pod: $pod, defaulting to v3.1.3 upgrade path"
    UPGRADE_FROM_3_1_X="true"
  fi
}

# Auto-detect if not already set by onprem_upgrade.sh
if [[ -z "$UPGRADE_FROM_3_1_X" ]]; then
  detect_upgrade_path
else
  echo "Using upgrade path from parent script: UPGRADE_FROM_3_1_X=$UPGRADE_FROM_3_1_X"
fi

podname=$(get_postgres_pod)

check_postgres() {
  if [[ -f "$local_backup_path" ]]; then
    read -rp "A backfile file already exists.
    If you would like to continue using this backup file type Continue :
    " confirm && [[ $confirm == [cC][oO][nN][tT][iI][nN][uU][eE] ]] || exit 1
    # avoid the rest of the check function as this could be a recovery from a failed update
    return
  fi

  # Dynamically get the correct PostgreSQL pod name
  local current_podname=$(get_postgres_pod)

  # Check if the PostgreSQL pod is running
  pod_status=$(kubectl get pods -n $postgres_namespace $current_podname -o jsonpath='{.status.phase}')

  if [ "$pod_status" != "Running" ]; then
    echo "PostgreSQL pod is not running. Current status: $pod_status"
    exit 1
  fi

  echo "PostgreSQL pod running."
}

disable_security() {
  sed -ibak 's/^\([^#]*\)md5/\1trust/g' /opt/bitnami/postgresql/conf/pg_hba.conf
  pg_ctl reload
}

enable_security() {
  sed -ibak 's/^\([^#]*\)trust/\1md5/g' /opt/bitnami/postgresql/conf/pg_hba.conf
  pg_ctl reload
}

backup_postgres() {
  if [[ -f "$local_backup_path" ]]; then
  echo "Backup file detected skipping backup"
  return
  fi
  echo "Backing up databases from pod $podname in namespace $postgres_namespace..."

  if [[ "$UPGRADE_FROM_3_1_X" == "true" ]]; then
        remote_backup_path="/tmp/${postgres_namespace}_backup.sql"
  else
        remote_backup_path="/var/lib/postgresql/data/${postgres_namespace}_backup.sql"
  fi

  kubectl exec -n $postgres_namespace $podname -- /bin/bash -c "$(typeset -f disable_security); disable_security"

  if kubectl exec -n $postgres_namespace $podname -- /bin/bash -c "pg_dumpall -U $POSTGRES_USERNAME -f '$remote_backup_path'"; then
    echo "Backup completed successfully for pod $podname in namespace $postgres_namespace."
    kubectl cp "$postgres_namespace/$podname:$remote_backup_path" "$local_backup_path"
    cp "$local_backup_path" "${local_backup_path}.bak"
    sed -ni '1,/-- Roles/p;/-- User Configurations/,$p' "$local_backup_path"
  else
    echo "Backup failed for pod $podname in namespace $postgres_namespace."
  fi
}

delete_postgres() {
  kubectl patch application -n $application_namespace postgresql-secrets  -p '{"metadata": {"finalizers": ["resources-finalizer.argocd.argoproj.io"]}}' --type merge
  kubectl delete application -n $application_namespace postgresql-secrets --cascade=background
  # background as pvc will not be deleted until app deletion
  kubectl delete pvc -n $postgres_namespace data-postgresql-0 --ignore-not-found=true &

  # patch ensures cascade delete
  if kubectl get application postgresql -n "$application_namespace" --no-headers >/dev/null 2>&1; then
    echo "Found postgresql application, applying finalizer patch..."
    if kubectl patch application postgresql -n "$application_namespace" \
        -p '{"metadata": {"finalizers": ["resources-finalizer.argocd.argoproj.io"]}}' --type merge; then
        echo "✅ Finalizer patch applied successfully"
        kubectl delete application -n $application_namespace postgresql --cascade=background
    else
        echo "❌ Failed to apply finalizer patch"
    fi
  else
      echo "postgresql application not found in namespace $application_namespace, skipping patch"
  fi

  kubectl delete secret --ignore-not-found=true -n $postgres_namespace postgresql
}

restore_postgres() {
  podname=$(get_postgres_pod)

  remote_backup_path="/var/lib/postgresql/data/${postgres_namespace}_backup.sql"

  kubectl cp "$local_backup_path" "$postgres_namespace/$podname:$remote_backup_path" -c postgres

  echo "Restoring backup databases from pod $podname in namespace $postgres_namespace..."

  # Get postgres password from secret
  if [[ "$UPGRADE_FROM_3_1_X" == "true" ]]; then
        PGPASSWORD=$(kubectl get secret -n $postgres_namespace postgresql -o jsonpath='{.data.postgres-password}' | base64 -d)
else
        PGPASSWORD=$(kubectl get secret -n $postgres_namespace orch-database-postgresql -o jsonpath='{.data.password}' | base64 -d)
fi

  # CloudNativePG doesn't need security disable/enable, just use credentials
  # Use the remote backup file that was copied to the pod
  kubectl exec -n $postgres_namespace "$podname" -c postgres -- env PGPASSWORD="$PGPASSWORD" psql -U $POSTGRES_USERNAME -f "$remote_backup_path"

  echo "Restore completed successfully."
}
