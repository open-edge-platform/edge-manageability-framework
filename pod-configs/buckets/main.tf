# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_s3_bucket" "lp_devops_terraform" {
  bucket = var.bucket
}
