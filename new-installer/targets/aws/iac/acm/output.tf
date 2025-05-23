# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "cert" {
  description = "The ACM certificate"
  value = aws_acm_certificate.main
}
