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
terraform apply -var-file=environments/[env]/variables.tfvar
```

## Key Configuration Variables

### Required Variables

- `vpc_name` - Name for the VPC
- `vpc_cidr_block` - Primary CIDR block for the VPC
- `vpc_additional_cidr_blocks` - Set of additional CIDR blocks to attach to the VPC
- `private_subnets` - Map of private subnet configurations
- `public_subnets` - Map of public subnet configurations
- `region` - AWS region to deploy into

### Optional Variables

- `customer_tag` - Optional tag for identifying customer resources
- `jumphost_ip_allow_list` - List of IPs allowed to access the jump host
- `jumphost_instance_ssh_key_pub` - SSH public key for the jump host
- `production` - Boolean to indicate if this is a production environment (defaults to true)

## Jump Host Configuration

To access the jump host, you need to:

1. Configure `jumphost_ip_allow_list` with your allowed source IPs
2. Provide an SSH public key via the `jumphost_instance_ssh_key_pub` variable
3. Configure the `jumphost_subnet` variable to specify which subnet to place the jump host in
