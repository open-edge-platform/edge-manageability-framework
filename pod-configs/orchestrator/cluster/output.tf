# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "cluster_name" {
  value = var.eks_cluster_name
}

output "eks_nodegroup_instance_ids" {
  value = module.eks.eks_nodegroup_instance_ids
}

output "eks_nodegroup_instance_sg_ids" {
  value = module.eks.security_groups
}

output "eks_node_group_name" {
  value = module.eks.eks_node_group_name
}

output "eks_cluster_OIDC_issuer" {
  value = module.eks.eks_cluster_identity[0].oidc[0].issuer
}

output "eks_auth_map"{
  value = var.enable_eks_auth ? module.eks_auth[0].eks_auth_map : ""
}

output "gitea_user_passwords" {
  value = module.gitea.gitea_user_passwords
  sensitive = true
}

output "gitea_master_password" {
  value = module.gitea.gitea_master_password
  sensitive = true
}
