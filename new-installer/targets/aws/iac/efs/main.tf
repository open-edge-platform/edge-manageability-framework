# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0


data "aws_eks_cluster" "eks" {
  name = var.cluster_name
}

data "aws_vpc" "main" {
  id = var.vpc_id
}

data "aws_caller_identity" "current" {}

locals {
  account_id = data.aws_caller_identity.current.account_id
  efs_policy_source = "https://raw.githubusercontent.com/kubernetes-sigs/aws-efs-csi-driver/v1.5.4/docs/iam-policy-example.json"
  eks_issuer = replace(data.aws_eks_cluster.eks[0].identity[0].oidc[0].issuer, "https://", "")
  efs_throughput_mode = "elastic"
  efs_encryption = true
  transition_to_ia = "AFTER_7_DAYS"
  transition_to_primary_storage_class = "AFTER_1_ACCESS"
}

# IAM Policy
data "http" "iam_policy" {

  url = local.efs_policy_source

  request_headers = {
    Accept = "application/json"
  }
}

resource "aws_iam_policy" "efs_policy" {
  name   = "${var.cluster_name}-EFS_CSI_Driver_Policy"
  policy = data.http.iam_policy[0].response_body
}

data "aws_iam_policy_document" "efs_assume_role_policy" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    condition {
      test     = "StringEquals"
      variable = "${local.eks_issuer}:sub"
      values   = ["system:serviceaccount:kube-system:efs-csi-controller-sa"]
    }

    principals {
      identifiers = ["arn:aws:iam::${local.account_id}:oidc-provider/${local.eks_issuer}"]
      type        = "Federated"
    }
  }
}

resource "aws_iam_role" "efs_role" {
  name                = "${var.cluster_name}-${var.role_name}"
  assume_role_policy  = data.aws_iam_policy_document.efs_assume_role_policy.json
}

resource "aws_iam_role_policy_attachment" "efs_role" {
  role       = aws_iam_role.efs_role.name
  policy_arn = aws_iam_policy.efs_policy[0].arn
}

# Create Security Group
resource "aws_security_group" "allow_nfs" {
  name   = "${var.cluster_name}-efs-nfs"
  vpc_id = var.vpc_id
  description = "Allow NFS traffic from VPC"
  tags = {
    Name = "${var.cluster_name}-efs-nfs"
    environment = var.cluster_name
  }
}

resource "aws_security_group_rule" "allow_nfs" {
  type              = "ingress"
  from_port         = 2049
  to_port           = 2049
  protocol          = "tcp"
  cidr_blocks       = data.aws_vpc.main.cidr_block
  security_group_id = aws_security_group.allow_nfs.id
  description       = "Allow NFS traffic from VPC"

}

# EFS
resource "aws_efs_file_system" "efs" {
  encrypted       = local.efs_encryption
  throughput_mode = local.efs_throughput_mode
  tags = {
    Name = var.cluster_name
  }

  lifecycle_policy {
    transition_to_ia = local.transition_to_ia
  }
  lifecycle_policy {
    transition_to_primary_storage_class = local.transition_to_primary_storage_class
  }
}

resource "aws_efs_mount_target" "target" {
  for_each = var.private_subnet_ids

  file_system_id  = aws_efs_file_system.efs.id
  subnet_id       = each.key
  security_groups = [aws_security_group.allow_nfs.id]
}
