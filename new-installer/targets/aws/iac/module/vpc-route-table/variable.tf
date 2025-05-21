# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "vpc" {}
variable "vpc_name" {}
variable "internet_gateway" {
  default  = null
  nullable = true
}
variable "nat_gateways" {}
variable "public_subnets" {
  type = map(object({
    az         = string
    cidr_block = string
  }))
  default = {}
}
variable "private_subnets" {
  type = map(object({
    az         = string
    cidr_block = string
  }))
}
variable "set_up_public_route" {
  type    = bool
  default = false
}