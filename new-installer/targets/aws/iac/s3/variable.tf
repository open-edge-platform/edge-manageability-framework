# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "region" {
  type = string
}
variable "customer_tag" {
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
