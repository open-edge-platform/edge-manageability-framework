# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "certificate_body" {
  description = "The content of certificate"
}

variable "certificate_chain" {
  description = "The content of certificate"
  default = null
}

variable "private_key" {
  description = "The content of key"
}

variable "cluster_name" {
  description = "The name of the cluster"
}
