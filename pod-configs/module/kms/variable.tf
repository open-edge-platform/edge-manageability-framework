# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "cluster_name" {
  type = string
}

variable "aws_account_number" {
  type = string
}

variable "permissions_boundary" {
  description = "ARN of the IAM permissions boundary policy"
  type        = string
  default     = ""
}
