# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0}

# Create IAM role for Vault KMS access
resource "aws_iam_role" "vault_kms" {
  name = "vault-kms-role-${var.cluster_name}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          # Allow EC2 instances to assume this role (for jumphost/external access)
          Service = "ec2.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
  
  permissions_boundary = var.permissions_boundary != "" ? var.permissions_boundary : null
  
  tags = {
    Name    = "vault-kms-role-${var.cluster_name}"
    Cluster = var.cluster_name
    Purpose = "vault-kms-access"
  }
}

# Create KMS key
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

# Create KMS alias
resource "aws_kms_alias" "vault" {
  name          = "alias/vault-kms-unseal-${var.cluster_name}"
  target_key_id = aws_kms_key.vault.key_id
}

# KMS key policy - only root permissions, no user-specific policies
resource "aws_kms_key_policy" "vault" {
  key_id = aws_kms_key.vault.id
  policy = jsonencode({
    Id = "vault-kms-policy"
    Statement = [
      {
        "Sid": "Enable IAM User Permissions",
        "Effect": "Allow",
        "Principal": {
          "AWS": "arn:aws:iam::${var.aws_account_number}:root"
        },
        "Action": "kms:*",
        "Resource": "*"
      }
    ]
    Version = "2012-10-17"
  })
}

# Attach KMS policy to the role
resource "aws_iam_role_policy" "vault_kms_access" {
  name = "vault-kms-access-policy"
  role = aws_iam_role.vault_kms.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "kms:Encrypt",
          "kms:Decrypt",
          "kms:ReEncrypt*",
          "kms:GenerateDataKey*",
          "kms:DescribeKey"
        ]
        Resource = aws_kms_key.vault.arn
      }
    ]
  })
}

# Create Kubernetes service account with role annotation (for IRSA)
resource "kubernetes_service_account" "vault" {
  metadata {
    name      = "vault-service-account"
    namespace = "orch-platform"
    annotations = {
      "eks.amazonaws.com/role-arn" = aws_iam_role.vault_kms.arn
    }
  }
}

resource "kubernetes_secret" "vault_kms_unseal" {
  metadata {
    name      = "vault-kms-unseal"
    namespace = "orch-platform"
  }
  data = {
    "AWS_ROLE_ARN"     = aws_iam_role.vault_kms.arn
    "KMS_KEY_ID"       = aws_kms_key.vault.key_id
    "KMS_REGION"       = var.region
  }
}
