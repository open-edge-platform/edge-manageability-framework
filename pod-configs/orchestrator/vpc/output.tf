# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "vpc_name" {
  value = var.vpc_name
}
output "public_subnets" {
  value = data.aws_subnet.pubilc_subnets
}
output "private_subnets" {
  value = data.aws_subnet.private_subnets
}
output "cidr_blocks" {
  value = concat([var.vpc_cidr_block], tolist(var.vpc_additional_cidr_blocks))
}
output "vpc_id" {
  value = module.vpc.vpc.id
}
output "region" {
  value = var.region
}
