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
  nat_public_ips    = toset([for id, nat in data.aws_nat_gateway.vpc_nat_gateway : "${nat.public_ip}/32" if nat.connectivity_type == "public"])
  ip_allow_list     = setunion(var.ip_allow_list, local.nat_public_ips)
  listeners = {
    "https" : {
      listen          = 443
      protocol        = "HTTPS"
      certificate_arn = module.ap_tls_cert.cert.arn
    }
  }
  infra_service_target_groups = {
    "argocd" : {
      listener = "https"
      type     = "ip"
      match_hosts = ["argocd.*"]
    },
    "gitea" : {
      listener = "https"
      type     = "ip"
      match_hosts = ["gitea.*"]
    }
  }
  traefik_target_groups = {
    "default" : {
      listener                          = "https"
      type                              = "ip"
      expected_health_check_status_code = 404
    },
    "grpc" : {
      listener                          = "https"
      protocol_version                  = "GRPC"
      expected_health_check_status_code = 0
      type                              = "ip"
      match_headers = {
        "content-type" = "application/grpc*"
      }
    }
  }
}

module "traefik_load_balancer" {
  source                     = "../module/application-load-balancer"
  name                       = "traefik"
  internal                   = var.internal
  vpc_id                     = var.vpc_id
  cluster_name               = var.cluster_name
  subnets                    = var.public_subnet_ids
  ip_allow_list              = var.ip_allow_list
  listeners                  = local.listeners
  target_groups              = local.traefik_target_groups
  enable_deletion_protection = var.enable_deletion_protection
}

module "argocd_load_balancer" {
  source                     = "../module/application-load-balancer"
  name                       = "argocd"
  internal                   = var.internal
  vpc_id                     = var.vpc_id
  cluster_name               = var.cluster_name
  subnets                    = var.public_subnet_ids
  ip_allow_list              = var.ip_allow_list
  listeners                  = local.listeners
  target_groups              = local.infra_service_target_groups
  enable_deletion_protection = var.enable_deletion_protection
}
