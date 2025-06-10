# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "traefik_dns_name" {
  value = aws_lb.traefik.dns_name
}

output "infra_dns_name" {
  value = aws_lb.infra.dns_name
}

output "traefik_target_group_arn" {
  value = aws_lb_target_group.traefik.arn
}

output "traefik_grpc_target_group_arn" {
  value = aws_lb_target_group.traefik_grpc.arn
}

output "infra_argocd_target_group_arn" {
  value = aws_lb_target_group.infra_argocd.arn
}

output "infra_gitea_target_group_arn" {
  value = aws_lb_target_group.infra_gitea.arn
}

output "traefik_lb_arn" {
  value = aws_lb.traefik.arn
}

output "infra_lb_arn" {
  value = aws_lb.infra.arn
}
