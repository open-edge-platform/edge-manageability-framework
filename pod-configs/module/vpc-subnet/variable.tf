# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "vpc" {}
variable "vpc_name" {}
variable "private_subnets" {
  type = map(object({
    az         = string
    cidr_block = string
  }))
  description = "Subnet for internal communication such as EKS and VPCE"
}
variable "public_subnets" {
  type = map(object({
    az         = string
    cidr_block = string
  }))
  description = "Subnet for external connection to edge nodes or customers"
}
