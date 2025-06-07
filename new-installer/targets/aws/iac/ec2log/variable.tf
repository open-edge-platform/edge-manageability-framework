# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "cluster_name" {
  type = string
}

variable "nodegroup_role" {
  type = string
}

variable "s3_prefix" {
  type    = string
  default = ""
}

variable "region" {
  type = string
}

variable "customer_tag" {
  type    = string
  default = ""
}
