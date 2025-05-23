# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0


data "aws_vpc" "vpc" {
  id = var.vpc_id
}

locals {
  cidr_blocks = concat([data.aws_vpc.cidr_block], [for id, cidr in data.aws_vpc.vpc.cidr_block_associations : cidr.cidr_block])
}

module "aurora" {
  source                      = "../module/aurora"
  vpc_id                      = var.vpc_id
  cluster_name                = var.cluster_name
  subnet_ids                  = var.private_subnet_ids
  ip_allow_list               = local.cidr_blocks # Allow entire VPC to access it
  availability_zones          = var.aurora_availability_zones
  instance_availability_zones = var.aurora_instance_availability_zones
  postgres_ver_major          = var.aurora_postgres_ver_major
  postgres_ver_minor          = var.aurora_postgres_ver_minor
  min_acus                    = var.aurora_min_acus
  max_acus                    = var.aurora_max_acus
  dev_mode                    = var.aurora_dev_mode
}
