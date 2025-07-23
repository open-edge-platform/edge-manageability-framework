# EMF Cloud Upgrade Guide

**Upgrade Path:** EMF Cloud v3.0 → v3.1  
**Document Version:** 1.0

## Overview

This document provides step-by-step instructions to upgrade
Cloud Edge Manageability Framework (EMF) from version 3.0 to 3.1.

### Important Notes

> **⚠️ DISRUPTIVE UPGRADE WARNING**  
> This upgrade requires edge node re-onboarding due to architecture changes (RKE2 → K3s).  
> Plan for edge nodes service downtime and
manual data backup/restore procedures in edge nodes.

## Prerequisites

### System Requirements

- Current EMF Cloud installation version 3.0
- Root/sudo privileges on orchestrator node
- PostgreSQL service running and accessible
- docker user credential if any pull limit hit

### Pre-Upgrade Checklist

- [ ] Back up critical application data from edge nodes
- [ ] Document current edge node configurations  
- [ ] Remove all edge clusters and hosts:
  - [Delete clusters](https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/clusters/delete_clusters.html)
  - [De-authorize hosts](https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/deauthorize_host.html)
  - [Delete hosts](https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/set_up_edge_infra/delete_host.html)

## Upgrade Procedure

### Step 1: Prepare Edge Orchestrator Upgrade Environment

You need to follow the steps mentioned in [Prerequisites section] (https://docs.openedgeplatform.intel.com/edge-manage-docs/3.0/deployment_guide/cloud_deployment/cloud_get_started/cloud_start_installer.html#prerequisites) with some changes as listed below:

1. Pull the intended 3.1 cloud installer image and extract it
```bash
# Replace <3.1-TAG> with actual tag
oras pull registry-rs.edgeorchestration.intel.com/edge-orch/common/files/cloud-orchestrator-installer:<3.1-TAG>
tar -xzf _build/cloud-orchestrator-installer.tgz
```

2. Start the installation Environment

- Start the Edge Orchestrator installer
```bash
./start-orchestrator-install.sh
```
-- Type 2 for managing an existing cluster

-- Type in cluster details, including cluster name and the AWS region. (Same values as were passed during cluster provisioning)

-- Specify a location to store the installer settings. (Same values as were passed during cluster provisioning)

3. Provision the Environment

- Go to pod-configs directory
```bash
orchestrator-admin:~$ cd ~/pod-configs
```

- Configure the cluster provisioning parameters
```bash
orchestrator-admin:~/pod-configs$ ./utils/provision.sh config \
--aws-account [AWS account] \
--customer-state-prefix [S3 bucket name prefix to store provision state] \
--environment [Cluster name] \
--parent-domain [Root domain for deployment] \
--region [AWS region to install the cluster] \
--jumphost-ip-allow-list [IPs to permit cluster administration access]
```
> **⚠️ Note**
> Follow the official guide (https://docs.openedgeplatform.intel.com/edge-manage-docs/3.0/deployment_guide/cloud_deployment/cloud_get_started/cloud_orchestrator_install.html#create-provisioning-configuration) to get details about each parameter

- Run the following command to begin upgrade
```bash
orchestrator-admin:~/pod-configs$ ./utils/provision.sh upgrade \
--aws-account [AWS account] \
--customer-state-prefix [S3 bucket name prefix to store provision state] \
--environment [Cluster name] \
--parent-domain [Root domain for deployment] \
--region [AWS region to install the cluster] \
--jumphost-ip-allow-list [IPs to permit cluster administration access]
```

4. Upgrade Edge Orchestrator

- Go to home directory
```bash
orchestrator-admin:~$ cd ~
```

- Configure the cluster deployment options. From the ~ directory in the orchestrator-admin container, run the following command:
```bash
orchestrator-admin:~$ ./configure-cluster.sh
```

- Upgrade the Edge Orchestrator on the cluster
```bash
orchestrator-admin:~$ make upgrade
```

This process will start redeploying the upgraded applications in the cluster starting with root-app. 
Let it continue and you would observe "infra-external" app is failing due to orch-infra-rps and orch-infra-mps databases.
In order to fix the above problem, you need to follow below steps.

### Step 2: Create orch-infra-rps DB

1. Login to aurora postgres DB cluster running in AWS
```bash
PGPASSWORD='<password>' psql -h <host-endpoint-url> -U postgres -d postgres
```
> **⚠️ Note**
> <password> can be obtained from AWS Secret Manager for this specific cluster DB
> <host-endpoint-url> This is the cluster DB endpoint and can be obtained from Aurora and RDS service in AWS

This will take you to postgres prompt where you need to execute DB and user creation commands as given in next steps
```bash
postgres=>
```

2. Create orch-infra-rps DB and user
```bash
CREATE DATABASE "orch-infra-rps";
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON DATABASE "orch-infra-rps" FROM PUBLIC;
CREATE USER "orch-infra-rps-user" WITH PASSWORD '<USER_DEFINED_PASSWORD>';
GRANT CONNECT ON DATABASE "orch-infra-rps" TO "orch-infra-rps-user";
GRANT ALL PRIVILEGES ON DATABASE "orch-infra-rps" TO "orch-infra-rps-user";
ALTER DATABASE "orch-infra-rps" OWNER TO "orch-infra-rps-user";
```

3. Create orch-infra-mps DB and user
```bash
CREATE DATABASE "orch-infra-mps";
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON DATABASE "orch-infra-mps" FROM PUBLIC;
CREATE USER "orch-infra-mps-user" WITH PASSWORD '<USER_DEFINED_PASSWORD>';
GRANT CONNECT ON DATABASE "orch-infra-mps" TO "orch-infra-mps-user";
GRANT ALL PRIVILEGES ON DATABASE "orch-infra-mps" TO "orch-infra-mps-user";
ALTER DATABASE "orch-infra-mps" OWNER TO "orch-infra-mps-user";
\q
```
> **⚠️ Note**
> <USER_DEFINED_PASSWORD> This is the password of your choice. Use the same password everywhere where this appears.

4. Create kubernetes secrets for DB users created above
```bash
kubectl create secret generic mps-aurora-postgresql \
  --from-literal=PGDATABASE=orch-infra-mps \
  --from-literal=PGHOST=<host-endpoint-url> \
  --from-literal=PGPASSWORD=<USER_DEFINED_PASSWORD> \
  --from-literal=PGPORT=5432 \
  --from-literal=PGUSER=orch-infra-mps-user \
  --from-literal=password=<USER_DEFINED_PASSWORD> \
  -n orch-infra
```

```bash
kubectl create secret generic mps-reader-aurora-postgresql \
  --from-literal=PGDATABASE=orch-infra-mps \
  --from-literal=PGHOST=<host-endpoint-url> \
  --from-literal=PGPASSWORD=<USER_DEFINED_PASSWORD> \
  --from-literal=PGPORT=5432 \
  --from-literal=PGUSER=orch-infra-mps-user \
 --from-literal=password=<USER_DEFINED_PASSWORD> \
  -n orch-infra
```

```bash
kubectl create secret generic rps-aurora-postgresql \
  --from-literal=PGDATABASE=orch-infra-rps \
  --from-literal=PGHOST=<host-endpoint-url> \
  --from-literal=PGPASSWORD=<USER_DEFINED_PASSWORD> \
  --from-literal=PGPORT=5432 \
  --from-literal=PGUSER=orch-infra-rps-user \
 --from-literal=password=<USER_DEFINED_PASSWORD> \
  -n orch-infra
```

```bash
kubectl create secret generic rps-reader-aurora-postgresql \
  --from-literal=PGDATABASE=orch-infra-rps \
  --from-literal=PGHOST=<host-endpoint-url> \
  --from-literal=PGPASSWORD=<USER_DEFINED_PASSWORD> \
  --from-literal=PGPORT=5432 \
  --from-literal=PGUSER=orch-infra-rps-user \
 --from-literal=password=<USER_DEFINED_PASSWORD> \
  -n orch-infra
```

### Step 3: Delete/resync amt-dbpassword-secret-job pod in infra-extermal app
Once DB, user and secrets creation is done, you need to delete/resync amt-dbpassword-secret-job pod and it should make infra-external app healthy.


### Step 4: Verification
Log into web UI of the orchestrator.
Go to Settings->OS profiles. There you should see the any of the toolkit version upgraded to latest.