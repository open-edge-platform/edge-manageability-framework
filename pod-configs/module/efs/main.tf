# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# IAM Policy
data "http" "iam_policy" {
  count = var.generate_eks_policy ? 1 : 0
  url = var.policy_source

  request_headers = {
    Accept = "application/json"
  }
}

resource "aws_iam_policy" "efs_policy" {
  count = var.generate_eks_policy ? 1 : 0
  name   = "${var.cluster_name}-${var.policy_name}"
  policy = data.http.iam_policy[0].response_body
}


# IAM Role and OIDC issuers from EKS clusters
data "aws_eks_cluster" "eks" {
  count = var.generate_eks_policy ? 1 : 0
  name = var.cluster_name
}

locals {
  eks_issuer = var.generate_eks_policy ? replace(data.aws_eks_cluster.eks[0].identity[0].oidc[0].issuer, "https://", "") : ""
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
      identifiers = ["arn:aws:iam::${var.aws_accountid}:oidc-provider/${local.eks_issuer}"]
      type        = "Federated"
    }
  }
}

resource "aws_iam_role" "efs_role" {
  count = var.generate_eks_policy ? 1 : 0
  name                = "${var.cluster_name}-${var.role_name}"
  assume_role_policy  = data.aws_iam_policy_document.efs_assume_role_policy.json
  managed_policy_arns = var.generate_eks_policy ? [aws_iam_policy.efs_policy[0].arn] : []
}

# Create Security Group
resource "aws_security_group" "allow_nfs" {

  name   = "${var.cluster_name}-${var.sg_name}"
  vpc_id = var.vpc_id

  ingress {
    description = "TLS from VPC"
    from_port   = 2049
    to_port     = 2049
    protocol    = "tcp"
    cidr_blocks = var.efs_sg_cidr_blocks
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.cluster_name}-${var.sg_name}"
  }
}

# EFS
resource "aws_efs_file_system" "efs" {
  encrypted       = var.encrypted
  throughput_mode = var.throughput_mode
  tags = {
    Name = var.cluster_name
  }

  lifecycle_policy {
    transition_to_ia = var.transition_to_ia
  }
  lifecycle_policy {
    transition_to_primary_storage_class = var.transition_to_primary_storage_class
  }
}

resource "aws_efs_mount_target" "target" {
  for_each = var.subnets

  file_system_id  = aws_efs_file_system.efs.id
  subnet_id       = each.value
  security_groups = [aws_security_group.allow_nfs.id]
}

resource "aws_efs_access_point" "access_point" {
  for_each = var.access_points
  file_system_id = aws_efs_file_system.efs.id
  root_directory {
    path = each.value.root_dir
  }
  posix_user {
    uid = each.value.uid
    gid = each.value.gid
  }
}

