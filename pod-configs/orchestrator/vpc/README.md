# VPC

To create or manage the public VPCs

This config will create the following:

- The VPC with default CIDR block
- Attach additional CIDR block to the VPC
- Public and private subnets for VPC
- VPC Endpoints
- Jump host which attach to the public subnet
- NAT gateway which attached to the public subnet
- Internet gateway
- Route table for public and private subnet
  - Default route for public subnet uses Internet gateway
  - Default route for private subnet uses NAT gateway

## Creating VPCs

### Initialize backend and terraform state

Each environment has one directory under `environments` directory

To initialize the backend and initial state run:

```bash
terraform init -backend-config=environments/[env]/backend.tf
```

### Plan and apply the terraform config

```bash
terraform plan -var-file=environments/[env]/variables.tfvar
```
