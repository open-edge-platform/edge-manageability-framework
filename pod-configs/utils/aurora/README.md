# AWS Aurora for PostgreSQL

This directory contains scripts to manage AWS Aurora for Orchestrator.

## Requirements

The database configuration must satisfy the following requirements:

- One database instance to be shared by all Orchestrator services (M-A/C/I, harbor, keycloak, vault, etc.)
- Isolation between services achieved through dedicated credentials and databases
- Region-level high availability to ensure database survival during node and/or entire AZ failures
- Backup and restore capabilities
- Data encryption in transit and at rest

## High-Level Design

### AWS Aurora Configuration

- Compatible with PostgreSQL 14.6
- DB storage replicated across three AZs (same as EKS nodes)
- Managed admin credentials, automatically rotated every 7 days (via AWS Secrets Manager, only devops can access)
- Nightly backups on S3 with 30 day retention
- Reachable only from within EKS private subnets
- SSL enforced for all connections
- Storage encrypted with AWS managed Key (via AWS KMS)
- One ServerlessV2 instance per AZ (one writer/primary, one or more reader/backup) running on different AZs
- Transparent failover, where the reader instance is automatically promoted to primary if the writer fails (endpoint/host stays the same)
- The cost of ServelessV2 instances depends on load and is evaluated each second in increments of 0.5 Aurora Capacity Units (where 1 ACU ~= 2GB memory)
- When completely idle, the monthly cost of the reader instance should be around ~$50
- The plan is to evaluate usage and decide later whether to switch to non-serverless instances

### Database Isolation

- All screts to create database tables, users, and permissions are developed as Terraform module, see `module/aurora-database/main.tf` for detail of the implementation.
- The Terraform state stores database connection credentials in a k8s secret with a randomly generated password
- Each database gets a dedicated user ("role" in PostgreSQL terminology)
- The user/role has full privileges within the associated database
- Isolation is enforced at the connection level, allowing only one user/role to connect to each database

## Quick Start Instructions

All modules to create Aurora databases, users, and roles are included in the cluster module (`internal/cluster/main.tf`).

  ```terraform
module "aurora" {
  source                      = "../../module/aurora"
  vpc_id                      = local.vpc_id
  cluster_name                = var.eks_cluster_name
  subnet_ids                  = local.private_subnets
  ip_allow_list               = local.cidr_blocks # Allow entire VPC to access it
  availability_zones          = var.aurora_availability_zones
  instance_availability_zones = var.aurora_instance_availability_zones
  postgres_ver_major          = var.aurora_postgres_ver_major
  postgres_ver_minor          = var.aurora_postgres_ver_minor
  min_acus                    = var.aurora_min_acus
  max_acus                    = var.aurora_max_acus
  dev_mode                    = var.aurora_dev_mode
}

#...

module "aurora_database" {
  depends_on    = [module.aurora]
  source        = "../../module/aurora-database"
  host          = module.aurora.host
  port          = module.aurora.port
  username      = module.aurora.username
  password_id   = module.aurora.password_id
  databases     = local.database_names
  users         = local.database_users
  database_user = local.database_user_mapping
}

module "aurora_import" {
  depends_on       = [module.aurora_database, module.orch_init]
  source           = "../../module/aurora-import"
  for_each         = var.orch_databases
  host             = module.aurora.host
  host_reader      = module.aurora.host_reader
  port             = module.aurora.port
  eks_cluster_name = var.eks_cluster_name
  username         = each.value.user
  password         = module.aurora_database.user_password[each.value.user].result
  namespace        = each.value.namespace
  database         = each.key
}
  ```

  See the variable definition from each module for detail information.

2. These module does following:

  1. The "aurora" module: Create Aurora database and configure it based on the module parameter
  2. The "aurora_database" module: Create database, add users, and grant permissions for each user to database (with psql command)
  3. The "aurora_import" module: Creates Kubernetes secret with database credential (e.g., host, port, username, password)

## Detailed Instructions

### Reset Database Password and Secret

If the database secret gets deleted, the only option is to reset the password and create a new secret.

```text
$ cd internal/cluster # or external/cluster
$ ../../utils/aurora/reset-db-password.sh [namespace] [database] [username]
```

### Validate Secret

To validate the database privileges and the content of the secret, you can run
the `validate-db-secret.sh` script:

```text
$ cd internal/cluster # or external/cluster
$ ../../utils/aurora/validate-db-secret.sh orch-sre db1-aurora-postgresql

*** Retrieving secret orch-sre/db1-aurora-postgresql...
  - PGHOST=demo-aurora-postgresql.cluster-cfka4txnb9fx.us-west-2.rds.amazonaws.com
  - PGPORT=5432
  - PGUSER=orch-sre-system-db1_user
  - PGPASSWORD=(16 characters)
  - PGDATABASE=orch-sre-system-db1

*** Creating psql client pod...
*** Waiting for psql-client-14031 to be running...

*** Checking write permissions...
CREATE TABLE
INSERT 0 1

*** Checking read permissions...
 id | data
----+------
  1 | test
(1 row)


*** Cleaning up tables...
DROP TABLE

*** All tests passed
*** Cleaning up psql client...
pod "psql-client-14031" deleted
```

#### Negative Test

Validate access to `db2` using the generated secret -- it should work!
```text
$ cd internal/cluster # or external/cluster
$ ../../utils/aurora/validate-db-secret.sh orch-sre db2-aurora-postgresql
...
...
*** All tests passed
```

Now try to connect to `orch-sre-system-db1` (third parameter) using `db2` credentials -- the
connection should FAIL!

```text
$ cd internal/cluster # or external/cluster
$ ../../utils/aurora/validate-db-secret.sh orch-sre db2-aurora-postgresql orch-sre-system-db1

*** Retrieving secret orch-sre/db2-aurora-postgresql...
  - PGHOST=sc-dev-aurora-postgresql.cluster-clrdxvrnkf8x.us-west-2.rds.amazonaws.com
  - PGPORT=5432
  - PGUSER=orch-sre-system-db2_user
  - PGPASSWORD=(16 characters)
  - PGDATABASE=orch-sre-system-db2

*** Performing test on database 'orch-sre-system-db1'

*** Checking write permissions...
psql: FATAL:  permission denied for database "orch-sre-system-db1"
DETAIL:  User does not have CONNECT privilege.
command terminated with exit code 2
```

### PostgresSQL Shell

To get a psql shell on a given database:

```text
$ cd internal/cluster # or external/cluster
$ ../../utils/aurora/psql.sh orch-sre db1-aurora-postgresql
*** Retrieving secret orch-sre/db1-aurora-postgresql...
  - PGHOST=sc-dev-aurora-postgresql.cluster-clrdxvrnkf8x.us-west-2.rds.amazonaws.com
  - PGPORT=5432
  - PGUSER=orch-sre-system-db1_user
  - PGPASSWORD=(16 characters)
  - PGDATABASE=orch-sre-system-db1

psql (14.6, server 14.6)
SSL connection (protocol: TLSv1.2, cipher: AES128-SHA256, bits: 128, compression: off)
Type "help" for help.

orch-sre-system-db1=>
```

To get a shell for the admin user:

```text
$ ../../utils/aurora/psql.sh --admin
```

### pgWeb

[pgWeb](https://github.com/sosedoff/pgweb) is an open-source, web GUI based client for Postgres

To access pgweb on a given database:

```text
$ ../../utils/aurora/pgweb.sh orch-platform vault-aurora-postgresql

*** Creating pgweb pod...
*** Retrieving secret orch-platform/vault-aurora-postgresql...
  - PGHOST=demo-aurora-postgresql.cluster-cfka4txnb9fx.us-west-2.rds.amazonaws.com
  - PGPORT=5432
  - PGUSER=orch-platform-system-vault_user
  - PGPASSWORD=(16 characters)
  - PGDATABASE=orch-platform-system-vault

*** Waiting for pgweb-9801 to be running...
*** Waiting for pgweb-9801 to be running...
*** Waiting for pgweb-9801 to be running...
*** Waiting for pgweb-9801 to be running...
*** Waiting for pgweb-9801 to be running...
*** Start kubectl port-forward. pgweb will be available at http://localhost:8081
Forwarding from 127.0.0.1:8081 -> 8081
Forwarding from [::1]:8081 -> 8081
```

Then open the browser to access `http://localhost:8081`

To get a shell for the admin user:

```text
$ ../../utils/aurora/pgweb.sh --admin <CLUSTER_NAME> <AWS_REGION>
```

### Wipe out table content

Wiping out table content can be useful when we want to restart a service from clean slate.
In this section, we will take catalog service as an example to show you how to empty the table content while keeping the structure.

First, we need to access PostgreSQL shell as instructed [here](#postgressql-shell).

We can verify the table contents by:

```text
orch-app-system-catalog-service=> select name from applications;
     name
---------------
 console
 engage
 gsm-sigint
 librespeed-vm
 usb-helper
 visibility
 wifi-sigint
 win19-server
 wordpress
 console
 wifi-sigint
 engage
 wifi-sigint
 chatbot
 gsm-sigint
 librespeed-vm
 quividi
 gsm-sigint
 wifi-sigint
 iperf-web-vm
 win19-server
 librespeed-vm
 nginx1
 nginx2
(24 rows)
```

We can wipe out table content by

```text
orch-app-system-catalog-service=> truncate applications;
TRUNCATE TABLE
```

Finally let's confirm that table has been emptied.

```text
orch-app-system-catalog-service=> select name from applications;
     name
---------------
(0 rows)
```

### Wipe out entire database table

Wiping out table content is sometimes not enough to recover more complicated issues such as database migration.
In that case, we can completely wipe out entire table including structure.

First, we need to access PostgreSQL shell as instructed [here](#postgressql-shell).

We can completely drop the table by:

```text
orch-app-system-catalog-service=> drop table applications;
DROP TABLE
```

Finally let's confirm that table has been dropped.

```text
orch-app-system-catalog-service=> \dt
       List of relations
 Schema | Name | Type | Owner
--------+------+------+-------
(0 rows)
```
