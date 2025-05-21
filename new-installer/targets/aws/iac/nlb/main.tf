# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0


data "aws_nat_gateways" "vpc_nat_gateways" {
  vpc_id = var.vpc_id
}

data "aws_nat_gateway" "vpc_nat_gateway" {
  for_each = toset(data.aws_nat_gateways.vpc_nat_gateways.ids)
  id       = each.value
  vpc_id   = var.vpc_id
  state    = "available"
}
locals {
  nat_public_ips = toset([for id, nat in data.aws_nat_gateway.vpc_nat_gateway : "${nat.public_ip}/32" if nat.connectivity_type == "public"])
  ip_allow_list  = setunion(var.ip_allow_list, local.nat_public_ips)
  nlb_ports = {
    "https" : {
      listen              = 443
      target              = 31443
      type                = "ip"
      protocol            = "TCP"
      enable_health_check = true
    }
  }
}

module "traefik2_load_balancer" {
  source                     = "../module/load-balancer"
  name                       = "nginx"
  internal                   = var.internal
  vpc_id                     = var.vpc_id
  cluster_name               = var.cluster_name
  subnets                    = var.public_subnet_ids
  ip_allow_list              = local.ip_allow_list
  ports                      = local.nlb_ports
  enable_deletion_protection = var.enable_deletion_protection
}
