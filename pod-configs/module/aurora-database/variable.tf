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
variable "password" {
  description = "Deprecated, use password_id instead"
  sensitive = true
}
variable "password_id" {
  description = "The secret id of Aurora master password which stored on the secrests manager."
}
variable "databases" {
  description = "Set of databases name to be create."
  type        = set(string)
}
variable "users" {
  description = "Set of users to be created for database"
  type        = set(string)
}
variable "database_user" {
  description = "Map of database to user"
  type        = map(string)
}