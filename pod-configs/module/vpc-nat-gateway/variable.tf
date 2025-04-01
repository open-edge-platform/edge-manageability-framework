# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "vpc" {}
variable "vpc_name" {}
variable "subnets_with_eip" {
  type = map(object({
    az         = string
    cidr_block = string
  }))
  default = {}
}

variable "subnets_without_eip" {
  type = map(object({
    az         = string
    cidr_block = string
  }))
  default = {}
}
