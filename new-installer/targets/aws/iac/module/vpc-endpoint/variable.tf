# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "vpc" {}
variable "vpc_name" {}
variable "region" {}
variable "sg_name" {
  description = "Name for security group"
}
variable "cidr_allow_list" {
  type        = list(string)
  description = "List of CIDR blocks to access VPC endpoint"
}
variable "subnets" {
  type = map(object({
    az         = string
    cidr_block = string
  }))
  description = "Subnets for all endpoints"
}
variable "endpoints" {
  type = map(object({
    private_dns_enabled = bool
  }))
  description = "VPC endpoints in create"
  default = {
    "elasticfilesystem" : {
      private_dns_enabled = true
    }
    "s3" : {
      private_dns_enabled = false
    }
    "eks" : {
      private_dns_enabled = true
    }
    "sts" : {
      private_dns_enabled = true
    }
    "ec2" : {
      private_dns_enabled = true
    }
    "ec2messages" : {
      private_dns_enabled = true
    }
    "ecr.dkr" : {
      private_dns_enabled = true
    }
    "ecr.api" : {
      private_dns_enabled = true
    }
    "elasticloadbalancing" : {
      private_dns_enabled = true
    }
  }
}
