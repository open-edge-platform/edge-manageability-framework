# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "host" {
  description = "The database server host"
}
variable "port" {
  description = "The database server port"
  default     = "5432"
}
variable "username" {
  description = "The database username"
  default     = "postgres"
}
variable "database" {
  default     = ""
  description = "Database to use when connect"
}
variable "command_create" {
  description = "Command to run when module is created"
}
variable "command_destroy" {
  description = "Command to run when module is destroyed"
}
variable "password_id" {
  description = "ID to fetch database password from AWS secret manager"
}
