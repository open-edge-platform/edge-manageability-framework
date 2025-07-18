# Design Proposal: On-premises EMF storage backup and restore

Author(s): Andrei Palade

Last updated: 2025-07-17

## Abstract

During upgrade of EMF, various failures can occur which may leave the
orchestrator in an unusable state. To overcome this limitation, this
document proposes a tool that enables storage backup and restore of the
On-premises EMF. The document captures an overview of the architecture and
how it will used.

## EMF Storage

Currently, EMF uses `openebs-hostpath` as its storage solution.
This means that persistent data is stored on the local disks of the
Kubernetes nodes using the OpenEBS HostPath provisioner. This approach is
simple to set up and is suitable for development or testing environments,
but it may not provide the durability or scalability required for
production workloads, since data is tied to the specific node where it was
created. 

Furthermore, in case that the node that runs the orchestrator fails during the upgrade, currently it becomes difficult to rollback to a previous version.

## Proposal

### Using Velero with MinIO for Orchestrator Backup

Velero is an open-source tool designed for backing up and restoring Kubernetes cluster resources and persistent volumes. In this proposal, Velero is used in conjunction with MinIO, an S3-compatible object storage, to provide reliable backup and restore capabilities for the On-premises EMF orchestrator.

#### How it Works

1. **MinIO Deployment**: MinIO is deployed within the on-premises environment to serve as the backup storage backend. It exposes an S3-compatible API for object storage.
2. **Velero Installation**: Velero is installed in the Kubernetes cluster where the orchestrator runs. It is configured to use MinIO as its backup storage provider.
3. **Backup Process**:
    - Velero creates backups of Kubernetes resources (deployments, services, configmaps, etc.) and persistent volumes.
    - The backup data is stored in MinIO buckets.
4. **Restore Process**:
    - In case of failure or rollback, Velero can restore the orchestrator state from the backups stored in MinIO.


#### Benefits

- **Reliability**: Backups are stored off the cluster, reducing risk of data loss.
- **Flexibility**: MinIO can be deployed anywhere, providing S3 compatibility without relying on public cloud.
- **Automation**: Velero supports scheduled backups and automated restores.

#### Example Velero Configuration

```yaml
velero install \
  --provider aws \
  --bucket emf-backups \
  --secret-file ./minio-credentials \
  --use-volume-snapshots=false \
  --backup-location-config region=minio,s3ForcePathStyle="true",s3Url=http://minio-service:9000
```

This setup ensures that orchestrator state and data can be reliably backed up and restored using familiar, open-source tools.


## Implementation

This tool will be implemented as a Bash script that users can execute to trigger backup and restore operations. The script will provide commands to initiate a backup of the orchestrator state and data, as well as restore from a previous backup. 

Additionally, it will include options to enable or disable syncing of ArgoCD applications, allowing users to control application reconciliation during backup or restore processes. This approach ensures a simple, command-line driven workflow for managing EMF storage backups and restores.
