# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
data "terraform_remote_state" "vpc" {
  backend = "s3"
  config = {
    bucket = var.vpc_terraform_backend_bucket
    key    = var.vpc_terraform_backend_key
    region = var.vpc_terraform_backend_region
  }
}

locals {
  subnet_ids = [for name, subnet in data.terraform_remote_state.vpc.outputs.private_subnets : subnet.id]
  vpc_id            = data.terraform_remote_state.vpc.outputs.vpc_id
  region            = data.terraform_remote_state.vpc.outputs.region
  vpc_cidr_blocks   = data.terraform_remote_state.vpc.outputs.cidr_blocks
}

module "pull_through_cache_proxy" {
  source                = "../../module/pull-through-cache-proxy"
  vpc_id                = local.vpc_id
  subnet_ids            = local.subnet_ids
  ip_allow_list         = local.vpc_cidr_blocks
  name                  = var.name
  region                = local.region
  tls_cert_body         = var.tls_cert
  tls_cert_key          = var.tls_key
  https_proxy           = var.https_proxy
  http_proxy            = var.http_proxy
  no_proxy              = var.no_proxy
  route53_zone_name     = var.route53_zone_name
  with_public_ip        = var.with_public_ip
}
