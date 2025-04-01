# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "name" {
  description = "Name of the WAF WebACL"
}
variable "waf_rule_groups" {
  description = "Set of rule groups to apply to the WebACL"
  type = set(object({
    name        = string
    vendor_name = string
    priority    = number
  }))
  default = []
}
variable "assiciate_resource_arn" {
  description = "Resource ARN to associate with WAF WebACL"
}
