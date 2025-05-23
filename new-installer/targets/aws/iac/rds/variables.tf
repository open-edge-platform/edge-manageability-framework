# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "vpc_id" {
  description = "The VPC ID where the RDS will be created"
  type        = string
}
variable "cluster_name" {
  description = "The name of the cluster"
  type        = string
}
variable "private_subnet_ids" {
  description = "The list of private subnet IDs where the RDS will be created"
  type        = list(string)
}
variable "region" {
  type = string
}
variable "customer_tag" {
  type    = string
  default = ""
}
variable "aurora_availability_zones" {
  type        = set(string)
  description = "Availability zones to asociate to the RDS cluster."
  validation {
    condition     = length(var.aurora_availability_zones) >= 3
    error_message = "Aurora requires a minimum of 3 AZs."
  }
}
variable "aurora_instance_availability_zones" {
  type        = set(string)
  description = "Availability zones to asociate to the RDS instance."
  validation {
    condition     = length(var.aurora_instance_availability_zones) >= 1
    error_message = "At least 1 AZ for RDS instance."
  }
}
variable "aurora_postgres_ver_major" {
  type    = string
  default = "14"
}
variable "aurora_postgres_ver_minor" {
  type    = string
  default = "9"
}
variable "aurora_min_acus" {
  # 1 ACU ~= 2GB memory
  type        = number
  default     = "0.5"
  description = "Minimum of ACUs for Aurora instances, 1 ACU ~= 2GB memory."
}
variable "aurora_max_acus" {
  type        = number
  default     = 2
  description = "Maximum of ACUs for Aurora instances, 1 ACU ~= 2GB memory."
}
variable "aurora_dev_mode" {
  type        = bool
  default     = true
  description = <<EOT
Development mode, apply the following settings when true:
- Disable deletion protection
- Skips final snapshot when delete
- Make backup retention period to 7 days(30 days for production)
- Applys changes immediately instead of update the cluster during the maintaince window.
  EOT
}
