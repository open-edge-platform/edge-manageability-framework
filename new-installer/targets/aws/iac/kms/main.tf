# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

data "aws_caller_identity" "current" {}

locals {
  account_id = data.aws_caller_identity.current.account_id
}

# Set up IAM user for Vault to access KMS
resource "aws_iam_user" "vault" {
  name = "vault-${var.cluster_name}"
}

resource "aws_iam_access_key" "vault" {
  user = aws_iam_user.vault.name
}

# Set up KMS key with alias
resource "aws_kms_key" "vault" {
  description             = "Vault unseal key"
  deletion_window_in_days = 10
}

resource "aws_kms_alias" "vault" {
  name          = "alias/vault-kms-unseal-${var.cluster_name}"
  target_key_id = aws_kms_key.vault.key_id
}

resource "aws_kms_key_policy" "vault" {
  key_id = aws_kms_key.vault.id
  policy = jsonencode({
    Id = "vault"
    Statement = [
      {
          "Sid": "Enable IAM User Permissions",
          "Effect": "Allow",
          "Principal": {
              "AWS": "arn:aws:iam::${local.account_id}:root"
          },
          "Action": "kms:*",
          "Resource": "*"
      },
      {
        "Sid": "Allow use of the key"
        "Action": [
          "kms:Encrypt",
          "kms:Decrypt",
          "kms:ReEncrypt*",
          "kms:GenerateDataKey*",
          "kms:DescribeKey"
        ]
        "Effect": "Allow"
        "Principal": {
          "AWS": "arn:aws:iam::${local.account_id}:user/${aws_iam_user.vault.name}"
        }
        "Resource": "*"
      },
    ]
    Version = "2012-10-17"
  })
}
