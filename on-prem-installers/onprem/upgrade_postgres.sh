#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

podname="postgresql-0"
postgres_namespace=orch-database
POSTGRES_LOCAL_BACKUP_PATH="./" 
local_backup_file="${postgres_namespace}_${podname}_backup.sql"
local_backup_path="${POSTGRES_LOCAL_BACKUP_PATH}${local_backup_file}"
POSTGRES_USERNAME="postgres"  
application_namespace=onprem

check_postgres() {
  if [ -f "$local_backup_path" ]; then
    echo "Backup file already exists. Please remove/rename it before proceeding."
    echo "If you want to restore an already created file please comment out the prechecks and backup functions"
    exit 1
  fi

  # Check if the PostgreSQL pod is running
  pod_status=$(kubectl get pods -n $postgres_namespace $podname -o jsonpath='{.status.phase}')

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
  echo "Backing up databases from pod $podname in namespace $postgres_namespace..."

  remote_backup_path="/tmp/${postgres_namespace}_${podname}_backup.sql"
    
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
  # backgrounbd as pvc will not be deleted until app deletion
  kubectl delete pvc -n $postgres_namespace data-postgresql-0 &
  # patch ensures cascade delete
  kubectl patch application -n $application_namespace postgresql  -p '{"metadata": {"finalizers": ["resources-finalizer.argocd.argoproj.io"]}}' --type merge
  kubectl delete application -n $application_namespace postgresql --cascade=background


  kubectl delete secret --ignore-not-found=true -n $postgres_namespace postgresql
}

restore_postgres() {
  kubectl exec -n $postgres_namespace $podname -- /bin/bash -c "$(typeset -f disable_security); disable_security"
  remote_backup_path="/tmp/${postgres_namespace}_${podname}_backup.sql"
  kubectl cp "$local_backup_path" "$postgres_namespace/$podname:$remote_backup_path"

  echo "Restoring backup databases from pod $podname in namespace $postgres_namespace..."

  kubectl exec -n $postgres_namespace $podname -- /bin/bash -c "psql -U $POSTGRES_USERNAME <  $remote_backup_path "
  kubectl exec -n $postgres_namespace $podname -- /bin/bash -c "$(typeset -f enable_security); enable_security"
}

#### Check if the PostgreSQL pod is running
# prechecks

## Backup secret
# kubectl get secret -n $postgres_namespace postgresql -o yaml > postgres_secret.yaml

# backup_postgres
# delete_postgres
## Delete secret
# kubectl delete secret -n $postgres_namespace postgresql

# echo "upgrade argo chart HERE"
# # Restore secret after app delete but before postgress restored
# yq e 'del(.metadata.labels, .metadata.annotations, .metadata.uid, .metadata.creationTimestamp)' postgres_secret.yaml | kubectl apply -f -
# sleep 30

# restore_postgres