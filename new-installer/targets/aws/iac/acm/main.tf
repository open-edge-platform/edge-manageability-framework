# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_acm_certificate" "main" {
  private_key = var.private_key
  certificate_body = var.certificate_body
  certificate_chain = var.certificate_chain
  tags = {
    Name = "acm-certificate-${var.cluster_name}"
  }
}
