# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "cluster_name" {
  description = "The name of the deployment"
  type        = string
}

variable "region" {
  description = "The region of the deployment"
  type        = string
  default = "us-west-2"
}

variable "vpc_id" {
  description = "VPC ID"
  type = string
}

variable "subnet_ids" {
  description = "List of subnet IDs for the ECS service"
  type        = list(string)
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

variable "route53_zone_name" {
  description = "The Route53 zone ID for the deployment"
  type        = string
}

variable "ip_allow_list" {
  description = "The IP address allow list for the deployment"
  type        = list(string)
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
