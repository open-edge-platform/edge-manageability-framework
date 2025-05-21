# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

module "waf_web_acl_traefik" {
  source                 = "../module/waf-web-acl"
  name                   = "${var.cluster_name}-traefik"
  assiciate_resource_arn = var.traefik_load_balancer_arn
}

module "waf_web_acl_argocd" {
  source                 = "../module/waf-web-acl"
  name                   = "${var.cluster_name}-argocd"
  assiciate_resource_arn = var.argocd_load_balancer_arn
}
