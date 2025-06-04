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

variable "vpc_id" {
  description = "The VPC ID where the load balancer will be created"
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

variable "internal" {
  description = "Create load balancers for internal VPC"
  default     = false
}
variable "type" {
  description = "Load balancer type"
  default     = "network"
}
variable "subnets" {
  description = "List of subnet ids for this load balancer"
  type        = set(string)
}
variable "certificate_arn" {
  description = "The ARN of the certificate to use for the load balancer"
  type        = string
}
