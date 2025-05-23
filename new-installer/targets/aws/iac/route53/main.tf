# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

module "route53_orch" {
  source                          = "../module/orch-route53"
  parent_zone                     = var.parent_zone
  orch_name                       = var.orch_name
  host_name                       = var.host_name
  vpc_id                          = var.vpc_id
  vpc_region                      = var.vpc_region
  lb_created                      = var.lb_created
  create_root_domain              = var.create_root_domain
  enable_pull_through_cache_proxy = var.enable_pull_through_cache_proxy
}
