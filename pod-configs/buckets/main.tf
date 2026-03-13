# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_s3_bucket" "lp_devops_terraform" {
  bucket = var.bucket
}

# Block all public access (AWS-0086, AWS-0087, AWS-0091, AWS-0093)
resource "aws_s3_bucket_public_access_block" "lp_devops_terraform" {
  bucket                  = aws_s3_bucket.lp_devops_terraform.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
