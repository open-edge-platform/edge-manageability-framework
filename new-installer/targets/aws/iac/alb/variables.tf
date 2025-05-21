# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
variable "internal" {
  description = "Whether the load balancer is internal or external"
  type        = bool
  default     = false
}
variable "vpc_id" {
  description = "The VPC ID where the load balancer will be created"
  type        = string
}
variable "cluster_name" {
  description = "The name of the cluster"
  type        = string
}
variable "public_subnet_ids" {
  description = "The list of public subnet IDs where the load balancer will be created"
  type        = list(string)
}
variable "ip_allow_list" {
  description = "The list of IP addresses that are allowed to access the load balancer"
  type        = list(string)
}
variable "enable_deletion_protection" {
  description = "Enables load balancer deletion protection"
  type        = bool
  default     = true
}
