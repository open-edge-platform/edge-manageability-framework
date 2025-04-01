# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "cluster_name" {
  type = string
}

variable "aws_account_number" {
  type = string
}

variable "aws_region" {
  type = string
}

variable "vpc" {
  type = string
}

variable "aws_roles" {
    type = list(string)
}