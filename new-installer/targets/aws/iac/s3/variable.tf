# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "cluster_name" {
  description = "The name of the cluster"
  type        = string
}

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

variable "s3_create_tracing" {
  type    = bool
  default = false
}
variable "import_s3_buckets" {
  type    = bool
  default = false
}