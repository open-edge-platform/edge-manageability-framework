# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "web_acl_arn" {
  value = aws_wafv2_web_acl.main.arn
}
