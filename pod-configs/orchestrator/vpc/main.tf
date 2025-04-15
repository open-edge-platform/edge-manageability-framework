# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

terraform {
  backend "s3" {}
}

provider "aws" {
  region = var.region
  default_tags {
    tags = {
      environment = "vpc-${var.vpc_name}"
      customer = "${var.customer_tag}"
    }
  }
}

module "vpc" {
  source                 = "../../module/vpc"
  name                   = var.vpc_name
  cidr_block             = var.vpc_cidr_block
  additional_cidr_blocks = var.vpc_additional_cidr_blocks
  enable_dns_hostnames   = var.vpc_enable_dns_hostnames
  enable_dns_support     = var.vpc_enable_dns_support
}

module "subnet" {
  depends_on      = [module.vpc]
  source          = "../../module/vpc-subnet"
  vpc             = module.vpc.vpc
  vpc_name        = var.vpc_name
  public_subnets  = var.public_subnets
  private_subnets = var.private_subnets
}

module "nat_gateway" {
  source           = "../../module/vpc-nat-gateway"
  vpc              = module.vpc.vpc
  vpc_name         = var.vpc_name
  subnets_with_eip = var.public_subnets
  depends_on       = [module.subnet]
}

module "internet_gateway" {
  source   = "../../module/vpc-internet-gateway"
  vpc      = module.vpc.vpc
  vpc_name = var.vpc_name
}

module "route_table" {
  source              = "../../module/vpc-route-table"
  vpc                 = module.vpc.vpc
  vpc_name            = var.vpc_name
  set_up_public_route = true
  internet_gateway    = module.internet_gateway.internet_gateway
  nat_gateways        = module.nat_gateway.nat_gateways_with_eip
  public_subnets      = var.public_subnets
  private_subnets     = var.private_subnets
  depends_on          = [module.subnet]
}

module "endpoint" {
  source          = "../../module/vpc-endpoint"
  region          = var.region
  vpc             = module.vpc.vpc
  vpc_name        = var.vpc_name
  cidr_allow_list = concat([var.vpc_cidr_block], tolist(var.vpc_additional_cidr_blocks))
  subnets         = var.private_subnets
  sg_name         = var.endpoint_sg_name
  depends_on      = [module.subnet]
}

module "jumphost" {
  depends_on                    = [module.subnet]
  source                        = "../../module/vpc-jumphost"
  vpc_id                        = module.vpc.vpc.id
  vpc_name                      = var.vpc_name
  region                        = var.region
  egress_ip_allow_list          = concat([var.vpc_cidr_block], tolist(var.vpc_additional_cidr_blocks))
  jumphost_ami_id               = var.jumphost_ami_id
  jumphost_instance_type        = var.jumphost_instance_type
  jumphost_instance_ssh_key_pub = var.jumphost_instance_ssh_key_pub
  subnet                        = var.jumphost_subnet
  ip_allow_list                 = var.jumphost_ip_allow_list
  production                    = var.production
}

# Prepare for output
data "aws_subnet" "public_subnets" {
  for_each          = var.public_subnets
  availability_zone = each.value.az
  cidr_block        = each.value.cidr_block
  vpc_id            = module.vpc.vpc.id
  depends_on        = [module.subnet]
}

data "aws_subnet" "private_subnets" {
  for_each          = var.private_subnets
  availability_zone = each.value.az
  cidr_block        = each.value.cidr_block
  vpc_id            = module.vpc.vpc.id
  depends_on        = [module.subnet]
}
