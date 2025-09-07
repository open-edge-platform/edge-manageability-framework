# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "vpc_terraform_backend_bucket" {}
variable "vpc_terraform_backend_key" {}
variable "vpc_terraform_backend_region" {}

variable "name" {
  description = "The name of the deployment"
  type        = string
}

variable "aws_region" {
  description = "The AWS region to deploy the cluster"
  type        = string
  default     = "us-west-2"
}

variable "http_proxy" {
  type = string
  default = ""
  description = "HTTP proxy to use for ECS task"
}

variable "https_proxy" {
  type = string
  default = ""
  description = "HTTPS proxy to use for ECS task"
}

variable "no_proxy" {
  type = string
  default = ""
  description = "No proxy to use for ECS task"
}

variable "customer_tag" {
  description = "For customers to specify a tag for AWS resources"
  type        = string
  default     = ""
}

variable "tls_cert" {
  description = "The body of the SSL certificate"
  type        = string
}

variable "tls_key" {
  description = "The private key for the SSL certificate"
  type        = string
  sensitive   = true
}

variable "route53_zone_name" {
  description = "The Route53 zone ID for the deployment"
  type        = string
}

variable "with_public_ip" {
  description = "Whether to assign a public IP to the ECS service"
  type        = bool
  default     = false
}

variable "permissions_boundary" {
  description = "ARN of the IAM permissions boundary policy"
  type        = string
  default     = ""
}
