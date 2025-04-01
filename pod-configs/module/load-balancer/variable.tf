# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "vpc_id" {}
variable "cluster_name" {}
variable "name" {
  description = "Load balancer name"
}
variable "internal" {
  description = "Create load balancers for internal VPC"
  default     = true
}
variable "type" {
  description = "Load balancer type"
  default     = "network"
}
variable "subnets" {
  description = "List of subnet ids for this load balancer"
  type        = set(string)
}
variable "ports" {
  description = "Map of listener to target port and protocol, the key is a name for listener and target group"
  type = map(object({
    listen                           = number
    target                           = number
    protocol                         = optional(string, "TCP")
    type                             = optional(string, "ip")
    certificate_arn                  = optional(string, null)
    enable_health_check              = optional(bool, true)
    health_check_protocol            = optional(string, "TCP")
    health_check_path                = optional(string, null)
    health_check_healthy_threshold   = optional(number, 5)
    health_check_unhealthy_threshold = optional(number, 2)
  }))
  default = {
    "https" : {
      listen   = 443
      target   = 443
      protocol = "TCP"
    }
  }
}
variable "ip_allow_list" {
  type        = set(string)
  description = "List of IP sources to allow to connect to load balancers."
  default = []
}
variable "enable_deletion_protection" {
  default = true
}
