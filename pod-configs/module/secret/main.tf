# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_secretsmanager_secret" "secret" {
  name = var.name
  recovery_window_in_days = var.recovery_window_in_days
}

resource "aws_secretsmanager_secret_version" "secret_value" {
  secret_id     = aws_secretsmanager_secret.secret.id
  secret_string = var.secret_string
}

data "aws_iam_policy_document" "read_only" {
  statement {
    effect = "Allow"
    actions = [
      "secretsmanager:GetResourcePolicy",
      "secretsmanager:GetSecretValue",
      "secretsmanager:DescribeSecret",
      "secretsmanager:ListSecretVersionIds"
    ]
    resources = [
      aws_secretsmanager_secret.secret.arn
    ]
  }
}

resource "aws_iam_policy" "read_only" {
  name        = "secret_read_${var.name}"
  description = "Policy to read value from secret ${var.name}"
  policy      = data.aws_iam_policy_document.read_only.json
}

data "aws_iam_policy_document" "read_write" {
  statement {
    effect = "Allow"
    actions = [
      "secretsmanager:GetSecretValue",
      "secretsmanager:DescribeSecret",
      "secretsmanager:PutSecretValue",
      "secretsmanager:UpdateSecret"
    ]
    resources = [
      aws_secretsmanager_secret.secret.arn
    ]
  }
}

resource "aws_iam_policy" "read_write" {
  name        = "secret_read_write_${var.name}"
  description = "Policy to read and write the secret ${var.name}"
  policy      = data.aws_iam_policy_document.read_write.json
}
