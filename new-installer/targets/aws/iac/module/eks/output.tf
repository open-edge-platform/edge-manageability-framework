# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "eks_api_endpoint" {
  value = aws_eks_cluster.eks_cluster.endpoint
}

output "eks_certificate_authority" {
  description = "Base64 encoded certificate data required to communicate with the cluster."
  value       = aws_eks_cluster.eks_cluster.certificate_authority[0].data
}

output "security_groups" {
  value = [aws_security_group.eks_cluster.id, aws_eks_cluster.eks_cluster.vpc_config[0].cluster_security_group_id]
}

output "eks_cluster_identity" {
  value = aws_eks_cluster.eks_cluster.identity
}

output "eks_node_group_name" {
  value = aws_eks_node_group.nodegroup.node_group_name
}

output "eks_nodegroup_instance_ids" {
  value = data.aws_instances.eks_nodegroup_instances.ids
}

output "eks_nodegroup_role_name" {
  value = local.eks_nodegroup_role_name
}

output "eks_security_group_id" {
  value = aws_security_group.eks_cluster.id
  description = "The major security group ID for the EKS cluster"
}
