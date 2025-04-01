# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "needed_namespaces" {
  type    = list(string)
  default = ["orch-sre", "cattle-system", "orch-boots", "argocd", "orch-secret"]
}
variable "istio_namespaces" {
  type    = list(string)
  default = ["orch-infra", "orch-app", "orch-cluster", "orch-ui", "orch-platform", "orch-gateway"]
}

variable "tls_cert" {}
variable "tls_key" {}
variable "ca_cert" {}

variable "webhook_github_netrc" {
  description = "Content of netrc file which contains access token to connect to GitHub."
}

variable "sre_basic_auth_username" {
  type    = string
}

variable "sre_basic_auth_password" {
  type    = string
}

variable "public_cloud" {
  type = bool
  default = false
}

variable "smtp_user" {
  description = "SMTP server username"
  type        = string
  default     = ""
}

variable "smtp_pass" {
  description = "SMTP server password"
  type        = string
  default     = ""
}

variable "smtp_url" {
  description = "SMTP server address"
  type        = string
  default     = ""
}

variable "smtp_port" {
  description = "SMTP server port"
  type        = number
  default     = 587
}

variable "smtp_from" {
  description = "SMTP from header"
  type        = string
  default     = ""
}

variable "auto_cert" {
  type    = bool
  default = false
}

variable "release_service_refresh_token" {
  type = string
  description = "Refresh token to renew release service token"
}

variable "efs_id" {
  type = string
  description = "The EFS ID for EFS storage classes"
}

variable "cluster_name" {
  type = string
  description = "The name of the EKS cluster"
}
