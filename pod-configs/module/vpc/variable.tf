# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "name" {}
variable "cidr_block" {}
variable "additional_cidr_blocks" {
  type = set(string)
}
variable "enable_dns_hostnames" {
  type = bool
  default = true
}
variable "enable_dns_support" {
  type = bool
  default = true
}
