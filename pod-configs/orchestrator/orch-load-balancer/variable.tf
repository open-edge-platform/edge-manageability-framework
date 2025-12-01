# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "vpc_terraform_backend_bucket" {}
variable "vpc_terraform_backend_key" {}
variable "vpc_terraform_backend_region" {}

variable "cluster_terraform_backend_bucket" {}
variable "cluster_terraform_backend_key" {}
variable "cluster_terraform_backend_region" {}

variable "cluster_name" {
  description = "The cluster name for this load balancer"
}

variable "ip_allow_list" {
  type        = set(string)
  description = "List of IP sources to allow to connect to load balancers."
  default = []
}

variable "root_domain" {
  type        = string
  description = "Root domains for the cluster, the Terraform will set up target group for gRPC based on specific domains."
}

variable "create_target_group_attachment" {
  type        = bool
  description = "[Deprecated] Set true to create load balancer target group attachment resource"
  default     = false
}

variable "create_target_group_binding" {
  type        = bool
  description = "Set true to create load balancer target group attachment resource"
  default     = true
}

variable "enable_deletion_protection" {
  description = "Enables load balancer deletion protection"
  default     = true
}

variable "create_traefik2_load_balancer" {
  type        = bool
  description = "Set true to create dedicated load balancer for traefik2"
  default     = true
}

variable "create_traefik3_load_balancer" {
  type        = bool
  description = "Set true to create dedicated load balancer for traefik3"
  default     = true
}

variable "create_argocd_load_balancer" {
  type        = bool
  description = "Set true to create dedicated load balancer for infra service like ArgoCD and Gitea"
  default     = true
}

# Import them with TF_VAR_x
variable "tls_cert_body" {
  description = "The TLS certificate body of access proxy"
}

variable "tls_cert_chain" {
  description = "The TLS certificate chain of access proxy"
}

variable "tls_key" {
  description = "The TLS certificate key of access proxy"
  sensitive   = true
}

variable "internal" {
  description = "Create load balancers for internal VPC"
  default     = false
}

variable "customer_tag" {
  description = "For customers to specify a tag for AWS resources"
  type = string
  default = ""
}