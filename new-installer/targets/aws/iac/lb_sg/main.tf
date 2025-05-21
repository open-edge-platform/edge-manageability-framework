# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0


locals {
  lb_sg_ids = {
    "traefik": {
      port = 8443,
      security_group_id = module.traefik_load_balancer.lb_sg_id
    },
    "traefik2": {
      port = 443,
      security_group_id = module.traefik2_load_balancer[0].lb_sg_id
    },
    "argocd": {
      port = 8080,
      security_group_id = module.argocd_load_balancer[0].lb_sg_id
    },
    "gitea": {
      port = 3000,
      security_group_id = module.argocd_load_balancer[0].lb_sg_id
    }
  }
}

resource "aws_security_group_rule" "node_sg_rule" {
  for_each          = local.lb_sg_ids
  type              = "ingress"
  from_port         = each.value.port
  to_port           = each.value.port
  protocol          = "tcp"
  source_security_group_id = each.value.security_group_id
  security_group_id = var.eks_node_sg_id
  description       = "From sg ${each.value.security_group_id} to eks node port ${each.value.port}"
}
