# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "s3_prefix_used" {
  description = "The actual S3 prefix used in bucket names (either provided or randomly generated)"
  value       = var.s3_prefix == "" ? tostring(random_integer.random_prefix.result) : var.s3_prefix
}
