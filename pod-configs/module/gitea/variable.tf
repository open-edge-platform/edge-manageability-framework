# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "aws_region" {
  description = "The AWS region, used for getting the kubeconfig for EKS cluster"
}
variable "cluster_name" {
  description = "The name of the EKS cluster, used for getting the kubeconfig"
}
variable "name" {
  default = "gitea"
  description = "Name of the Gitea helm release"
}
variable "tls_cert_body" {
  description = "The TLS certificate body of Gitea service"
}
variable "tls_key" {
  description = "The TLS certificate key of Gitea service"
  sensitive   = true
}
variable "gitea_chart_version" {
  default = "10.4.0"
  description = "The version of the Gitea helm chart"
}
variable "gitea_fqdn" {
  description = "The domain of the Gitea service"
}

# Uncomment following variables when switching to RDS
# variable "gitea_database_endpoint" {
#   description = "The endpoint of the Gitea database"
# }
# variable "gitea_database_username" {
#   description = "The username of the Gitea database"
# }
# variable "gitea_database_password" {
#   description = "The password of the Gitea database"
#   sensitive = true
# }
# variable "gitea_database" {
#   default = "gitea"
#   description = "Name of database for Gitea service"
# }
