# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "certArn" {
  description = "The ACM certificate Arn."
  value = aws_acm_certificate.main.arn
}
