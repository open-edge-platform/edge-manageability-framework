# Orchestrator Cloud Infrastructure

## Design philosophy

- Modules should be general and simple which should include single category of
  tasks
- Example: The eks module should only create and manage EKS-releated resources,
  it should not be able to manage other AWS resources which does not directly
  connect to the EKS
- Similar deployments shares the same Terraform configuration, and use variable
  files to manage the difference.

## Repository architecture

### Directory for different categories

Each directory contains a `main.tf` file a the Terraform entrypoint, and
a `variable.tf` file to manage available vairables.

- buckets:  set up S3 buckets to store Terraform state
- account: set up resources under certain account, such as IAM roles.
- vpc-external: set up external-facing VPCs, those VPCs will be fully managed by
  the team.
- external/{subcategory}: Deployments for public clusters, such as production,
  public staging.

### The module directory

`module` directory contains all modules for deployment which can be used by each
deployments.

### The utils directory

This directory contains utilities to manage the deployment, like the CLI for
Aurora database.

## Backends

For every Terraform configs, we always use the following path format for S3
backend:

S3 Bucket: `lp-devops-[AWS account alias]-terraform`
S3 Key: `[AWS region]/[category]/[name or id]`

## Deploy resources

To deploy resources from a category, the first thing is to initialize Terraform
modules and backend:

`terraform init -backend-config environments/[environemnt]/backend.tf`

This command will include required modules and download providers that used by
the Terraform config.

Next is to check if eveything will be configured/deployed as expected

`terraform plan -var-file environments/[environment]/variable.tfvar`

This will show the plan about which resources will be created or updated.

Finally, we can use the following to apply Terraform configuration

`terraform apply -var-file environments/[environment]/variable.tfvar`

## Naming convention

Directory, filename: all lower case with `-` as word separator.

Terraform resource, data name: all lower case with `_` as word separator.

Infra resource name(e.g., cluster name, vm name, subnet name): all lower case
with `-` as word separator.
