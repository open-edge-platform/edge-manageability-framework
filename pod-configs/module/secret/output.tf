# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "secret_id" {
  value = aws_secretsmanager_secret.secret.id
}

output "secret_arn" {
  value = aws_secretsmanager_secret.secret.arn
}

output "read_only_policy_arn" {
  value = aws_iam_policy.read_only.arn
}

output "read_write_policy_arn" {
  value = aws_iam_policy.read_write.arn
}

output "read_only_policy_name" {
  value = aws_iam_policy.read_only.name
}

output "read_write_policy_name" {
  value = aws_iam_policy.read_write.name
}
