# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "cluster_name" {
  type = string
}

variable "region" {
  type = string
}

variable "customer_tag" {
  type    = string
  default = ""
}

variable "private_subnet_ids" {
  type = set(string)
  description = "Subnet IDs for EFS to attach"
}

variable "vpc_id" {
  type = string
  description = "VPC ID for EFS to attach"
}

variable "eks_oidc_issuer" {
  description = "OIDC issuer URL for EKS cluster"
  type        = string
  default     = ""
}
