# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

data "aws_caller_identity" "current" {}

resource "aws_kms_key" "log_group_key" {
  description = "Key to encrypt/decrypt log group ${var.name}"
  key_usage   = "ENCRYPT_DECRYPT"
  policy = jsonencode({
    "Version" : "2012-10-17",
    "Id" : "key-default-1",
    "Statement" : [
      {
        "Sid" : "Enable IAM User Permissions",
        "Effect" : "Allow",
        "Principal" : {
          "AWS" : "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"
        },
        "Action" : "kms:*",
        "Resource" : "*"
      },
      {
        "Sid" : "Allow log group to use the policy",
        "Effect" : "Allow",
        "Principal" : {
          "Service" : "logs.us-west-2.amazonaws.com"
        },
        "Action" : [
          "kms:Encrypt*",
          "kms:Decrypt*",
          "kms:ReEncrypt*",
          "kms:GenerateDataKey*",
          "kms:DescribeKey"
        ],
        "Resource" : "*"
      }
    ]
  })
}

resource "aws_kms_alias" "log_group_key" {
  name          = "alias/log-group-${var.name}"
  target_key_id = aws_kms_key.log_group_key.key_id
}

resource "aws_cloudwatch_log_group" "main" {
  name              = var.name
  retention_in_days = var.retention_in_days
  kms_key_id        = aws_kms_key.log_group_key.arn
}
