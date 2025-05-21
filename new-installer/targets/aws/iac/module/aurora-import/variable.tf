# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "eks_cluster_name" {
  description = "The EKS cluster to connect for database secrets"
}
variable "namespace" {
  description = "The kubernetes namespace to create the secret"
}
variable "host" {
  description = "The database server host"
}
variable "host_reader" {
  description = "The database server reader host"
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
  description = "The database password"
  sensitive   = true
}
variable "database" {
  description = "The database name"
}
