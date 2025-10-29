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

output "efs_file_system_id" {
  value = module.efs.efs.id
}

output "s3_prefix" {
  value = var.s3_prefix
}

output "sre_basic_auth_username" {
  value = var.sre_basic_auth_username
  sensitive = true
}

output "sre_basic_auth_password" {
  value = var.sre_basic_auth_password
  sensitive = true
}

output "sre_destination_secret_url" {
  value = var.sre_destination_secret_url
}

output "sre_destination_ca_secret" {
  value = var.sre_destination_ca_secret
}

output "auto_cert" {
  value = var.auto_cert
}

output "smtp_url" {
  value = var.smtp_url
}

output "single_tenancy" {
  value = var.single_tenancy
}
