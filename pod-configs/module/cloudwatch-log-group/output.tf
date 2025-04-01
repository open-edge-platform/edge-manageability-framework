# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "name" {
  value = aws_cloudwatch_log_group.main.name
}
output "kms_key_id" {
  value = aws_kms_key.log_group_key.key_id
}
