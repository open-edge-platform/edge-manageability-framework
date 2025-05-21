# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "parent_zone" {
  description = "The route53 zone name of the parent"
}
variable "orch_name" {
  description = "The Orchestrator cluster name"
}
variable "host_name" {
  description = "The host name of the root domain"
  default     = ""
}
variable "vpc_id" {
  description = "The VPC ID for the private route53 zone"
}
variable "vpc_region" {
  description = "The VPC region for the private route53 zone"
  default     = "us-west-2"
}
variable "lb_created" {
  type        = bool
  description = "Wether the LBs for the Orchestrator are created. The CNAME of {orch_name}.{parent_zone} will be created if it is true."
  default     = false
}
variable "create_root_domain" {
  type        = bool
  description = "Whether to create the root_domain."
  default     = true
}
variable "customer_tag" {
  description = "For customers to specify a tag for AWS resources"
  type = string
  default = ""
}
variable "enable_pull_through_cache_proxy" {
  type        = bool
  description = "Whether to enable the pull through cache proxy."
  default     = false
}
