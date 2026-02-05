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
variable "subnets" {
  description = "List of subnet ids for this load balancer"
  type        = set(string)
}
variable "listeners" {
  description = "Map of listeners definition, where the key is the name of listener"
  type = map(object({
    listen                           = number
    protocol                         = optional(string, "HTTPS")
    certificate_arn                  = string
  }))
  default = {}
}
variable "target_groups" {
  description = "Map of target group definition, the key is a name for target group"
  type = map(object({
    listener                          = string // listener name from listeners variable
    target                            = optional(number, 1) // default target port
    protocol                          = optional(string, "HTTPS") // target protocol
    protocol_version                  = optional(string, "HTTP1") // HTTP1, HTTP2, GRPC
    type                              = optional(string, "instance") // ip, instances
    enable_health_check               = optional(bool, true)
    health_check_protocol             = optional(string, "HTTPS")
    health_check_path                 = optional(string, "/")
    health_check_healthy_threshold    = optional(number, 5)
    health_check_unhealthy_threshold  = optional(number, 2)
    expected_health_check_status_code = optional(number, 200)
    match_headers                     = optional(map(string), {}) // header to match to apply this target group, leave null for default group
    match_hosts                       = optional(list(string), []) // host to match to apply this target group, leave null for default group
  }))
  default = {}
}
variable "ip_allow_list" {
  type        = set(string)
  description = "List of IP sources to allow to connect to load balancers."
  default = []
}
variable "enable_deletion_protection" {
  default = true
}
variable "default_ssl_policy" {
  type    = string
  default = "ELBSecurityPolicy-TLS13-1-2-Res-FIPS-2023-04"
}
