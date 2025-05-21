# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
data "aws_vpc" "vpc" {
  id = var.vpc_id
}

locals {
  cidr_blocks = concat([data.aws_vpc.cidr_block], [for id, cidr in data.aws_vpc.vpc.cidr_block_associations : cidr.cidr_block])
}

module "efs" {
  source                              = "../module/efs"
  subnets                             = var.private_subnet_ids
  efs_sg_cidr_blocks                  = local.cidr_blocks # Allow VPC to access EFS
  cluster_name                        = var.cluster_name
  vpc_id                              = var.vpc_id
}
