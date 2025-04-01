# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "traefik_target_groups" {
  value = module.traefik_load_balancer.target_groups
}
output "traefik2_target_groups" {
  value = var.create_traefik2_load_balancer ? module.traefik2_load_balancer[0].target_groups : null
}
output "argocd_target_groups" {
  value = var.create_argocd_load_balancer ? module.argocd_load_balancer[0].target_groups : null
}
