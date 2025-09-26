# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Get OIDC from EKS cluster
data "aws_eks_cluster" "eks" {
  name = var.cluster_name
}

# Define service accounts for Vault
locals {
  vault_service_accounts = [
    "system:serviceaccount:orch-platform:vault-service-account",
    "system:serviceaccount:orch-platform:vault"
  ]
}

# Create KMS policy
resource "aws_iam_policy" "vault_kms_policy" {
  description = "Policy that allows Vault access to KMS in ${var.cluster_name} cluster"
  name        = "${var.cluster_name}-vault-kms-policy"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        "Action": [
          "kms:Encrypt",
          "kms:Decrypt",
          "kms:ReEncrypt*",
          "kms:GenerateDataKey*",
          "kms:DescribeKey"
        ]
        "Effect": "Allow"
        Resource = aws_kms_key.vault.arn
      }
    ]
  })
}

# Create trust policy using OIDC
data "aws_iam_policy_document" "vault_trust_policy" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"
    condition {
      test     = "StringEquals"
      variable = "${replace(data.aws_eks_cluster.eks.identity[0].oidc[0].issuer, "https://", "")}:sub"
      values   = local.vault_service_accounts
    }
    principals {
      identifiers = ["arn:aws:iam::${var.aws_account_number}:oidc-provider/${replace(data.aws_eks_cluster.eks.identity[0].oidc[0].issuer, "https://", "")}"]
      type        = "Federated"
    }
  }
}

# Create role
resource "aws_iam_role" "vault_kms" {
  description         = "Role that allows Vault to access KMS in ${var.cluster_name} cluster"
  name                = "${var.cluster_name}-vault-kms-role"
  assume_role_policy  = data.aws_iam_policy_document.vault_trust_policy.json
  managed_policy_arns = [aws_iam_policy.vault_kms_policy.arn]
  permissions_boundary = var.permissions_boundary != "" ? var.permissions_boundary : null
}

# Create service account with role annotation
resource "kubernetes_service_account" "vault" {
  metadata {
    name      = "vault-service-account"
    namespace = "orch-platform"
    annotations = {
      "eks.amazonaws.com/role-arn" = aws_iam_role.vault_kms.arn
    }
  }
}

# KMS Key
resource "aws_kms_key" "vault" {
  description             = "Vault unseal key for ${var.cluster_name}"
  deletion_window_in_days = 10

  tags = {
    Name        = "vault-kms-unseal-${var.cluster_name}"
    Cluster     = var.cluster_name
    Purpose     = "vault-unseal"
    Application = "vault"
  }
}

resource "aws_kms_alias" "vault" {
  name          = "alias/vault-kms-unseal-${var.cluster_name}"
  target_key_id = aws_kms_key.vault.key_id
}

resource "kubernetes_secret" "vault_kms_unseal" {
  metadata {
    name      = "vault-kms-unseal"
    namespace = "orch-platform"
  }
  data = {
    # Configuration values
    "AWS_ROLE_ARN"  = aws_iam_role.vault_kms.arn
    "KMS_KEY_ID"    = aws_kms_key.vault.key_id
    "AWS_REGION"    = var.region
  }
}
