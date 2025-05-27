# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
variable "region" {
  type        = string
  description = "The AWS region to deploy the RDS"
  default     = "us-west-2"
}
variable "customer_tag" {
  type        = string
  description = "For customers to specify a tag for AWS resources"
  default     = ""
}
variable "vpc_id" {
  type        = string
  description = "The VPC ID to deploy the RDS"
}

variable "cluster_name" {
  type        = string
  description = "Name of the RDS cluster associated with this Aurora deployment"
}

variable "subnet_ids" {
  type        = set(string)
  description = "Subnets for the RDS cluster"
}

variable "ip_allow_list" {
  type        = set(string)
  description = "List of IP CIDRs to access the database"
}

variable "availability_zones" {
  type        = set(string)
  description = "Availability zones to asociate to the RDS cluster."
  validation {
    condition     = length(var.availability_zones) >= 3
    error_message = "Aurora requires a minimum of 3 AZs."
  }
}

variable "instance_availability_zones" {
  type        = set(string)
  description = "Availability zones to asociate to the RDS instance."
  validation {
    condition     = length(var.instance_availability_zones) >= 1
    error_message = "At least 1 AZ for RDS instance."
  }
}

variable "postgres_ver_major" {
  type    = string
  default = "14"
}

variable "postgres_ver_minor" {
  type    = string
  default = "6"
}

// Min and max ACUs for the Aurora instances
variable "min_acus" {
  # 1 ACU ~= 2GB memory
  type        = number
  default     = 0.5
  description = "Minimum of ACUs for Aurora instances, 1 ACU ~= 2GB memory."
}
variable "max_acus" {
  type        = number
  default     = 2
  description = "Maximum of ACUs for Aurora instances, 1 ACU ~= 2GB memory."
}

variable "dev_mode" {
  type        = bool
  default     = false
  description = <<EOT
Development mode, apply the following settings when true:
- Disable deletion protection
- Skips final snapshot when delete
- Make backup retention period to 7 days(30 days for production)
- Applys changes immediately instead of update the cluster during the maintaince window.
  EOT
}

variable "username" {
  description = "Default database major user"
  default     = "postgres"
}

variable "ca_cert_identifier" {
  description = "The Certificate authority of the database"
  default     = "rds-ca-rsa2048-g1"
}
