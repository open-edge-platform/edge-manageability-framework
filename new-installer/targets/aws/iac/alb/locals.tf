# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
data "aws_nat_gateways" "vpc_nat_gateways" {
  vpc_id   = var.vpc_id
}

data "aws_nat_gateway" "vpc_nat_gateway" {
  for_each = toset(data.aws_nat_gateways.vpc_nat_gateways.ids)
  id       = each.value
  vpc_id   = var.vpc_id
  state    = "available"
}

locals {
  ssl_polocy = "ELBSecurityPolicy-TLS13-1-2-Res-FIPS-2023-04"
  nat_public_ips    = toset([for id, nat in data.aws_nat_gateway.vpc_nat_gateway : "${nat.public_ip}/32" if nat.connectivity_type == "public"])
  ip_allow_list     = setunion(var.ip_allow_list, local.nat_public_ips)
}
