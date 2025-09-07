# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "aws_accountid" {
  type    = string
  default = ""
}

variable "s3_prefix" {
  type    = string
  default = ""
}

variable "cluster_name" {
  type        = string
  default     = ""
  description = "EKS Cluster which will related to the S3 buckets"
}

variable "create_tracing" {
  type    = bool
  default = false
}

variable "import_buckets" {
  type    = bool
  default = false
}

variable "permissions_boundary" {
  description = "ARN of the IAM permissions boundary policy"
  type        = string
  default     = ""
}
