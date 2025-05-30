# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "cluster_name" {
  type = string
}

variable "region" {
  type = string
  default = "us-west-2"
}
variable "customer_tag" {
  type = string
  default = ""
}