# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "vpc_id" {
  description = "The VPC ID for the deployment"
  type        = string
}
variable "subnet_ids" {
  description = "The subnet IDs for the deployment"
  type        = list(string)
}
variable "ip_allow_list" {
  description = "The IP address allow list for the deployment"
  type        = list(string)
}
variable "name" {
  description = "The name of the cluster"
  type        = string
  default = "pull-through-cache-proxy"
}
variable "region" {
  description = "The region of the deployment"
  type        = string
  default = "us-west-2"
}
# For more about CPU and memory values, see https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-cpu-memory-error.html
variable "cpu" {
  description = "The number of CPU units (in 1/1,024 of CPU) for the deployment"
  type        = number
  default     = 1024
}
variable "memory" {
  description = "The amount of memory (in MB) for the deployment"
  type        = number
  default     = 2048
}
variable "desired_count" {
  description = "The desired count of tasks for the ECS service"
  type        = number
  default     = 1
}
variable "tls_cert_key" {
  description = "The private key for the SSL certificate"
  type        = string
  sensitive = true
}
variable "tls_cert_body" {
  description = "The body of the SSL certificate"
  type        = string
}
variable "https_proxy" {
  description = "The HTTPS proxy URL"
  type        = string
  default     = ""
}

variable "http_proxy" {
  description = "The HTTP proxy URL"
  type        = string
  default     = ""
}

variable "no_proxy" {
  description = "The no proxy list"
  type        = string
  default     = ""
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
