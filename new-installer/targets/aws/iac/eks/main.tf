# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

data "aws_caller_identity" "current" {}

locals {
  cidr_blocks = concat([data.aws_vpc.cidr_block], [for id, cidr in data.aws_vpc.vpc.cidr_block_associations : cidr.cidr_block])
}

data "aws_ami" "eks_node_ami" {
  most_recent = true
  owners      = ["602401143452"] # Amazon EKS AMI owner ID
  filter {
    name   = "name"
    values = ["amazon-eks-node-${var.eks_version}-*"]
  }
}

module "eks" {
  source                      = "../module/eks"
  cluster_name                = var.cluster_name
  aws_account_number          = data.aws_caller_identity.current.account_id
  aws_region                  = var.region
  vpc_id                      = var.vpc_id
  subnets                     = var.private_subnet_ids
  ip_allow_list               = local.cidr_blocks # Allow entire VPC to access the cluster
  eks_node_ami_id             = data.aws_ami.eks_node_ami.id
  volume_size                 = var.eks_volume_size
  volume_type                 = var.eks_volume_type
  eks_node_instance_type      = var.eks_node_instance_type
  desired_size                = var.eks_desired_size
  min_size                    = var.eks_min_size
  max_size                    = var.eks_max_size
  addons                      = var.eks_addons
  eks_version                 = var.eks_version
  max_pods                    = var.eks_max_pods
  additional_node_groups      = var.eks_additional_node_groups
  public_cloud                = var.public_cloud
  enable_cache_registry       = var.enable_cache_registry
  cache_registry              = var.cache_registry
  customer_tag                = var.customer_tag
  user_script_pre_cloud_init  = var.eks_user_script_pre_cloud_init
  user_script_post_cloud_init = var.eks_user_script_post_cloud_init
  http_proxy                  = var.eks_http_proxy
  https_proxy                 = var.eks_https_proxy
  no_proxy                    = var.eks_no_proxy
  eks_cluster_dns_ip          = var.eks_cluster_dns_ip
}
