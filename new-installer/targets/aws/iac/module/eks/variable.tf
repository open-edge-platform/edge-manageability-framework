# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "cluster_name" {
  type = string
}

variable "aws_account_number" {
  type = string
}

variable "aws_region" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "subnets" {
  type = list(any)
}

variable "ip_allow_list" {
  type = list(string)
  default = []
}

variable "eks_node_ami_id" {
  type    = string
  default = "ami-0d3aa1878940d0eed"
}

variable "volume_size" {
  type    = number
  default = 120
}

variable "volume_type" {
  type    = string
  default = "gp3"
}

variable "eks_node_instance_type" {
  type    = string
  default = "t3.2xlarge"
}

variable "desired_size" {
  type    = number
  default = 1
}

variable "min_size" {
  type    = number
  default = 1
}

variable "max_size" {
  type    = number
  default = 1
}

variable "addons" {
  type = list(object({
    name                 = string
    version              = string
    configuration_values = optional(string, "")
  }))
}

variable "eks_version" {
  type = string
  // Use latest if not set
  default = "1.32"
}

variable "max_pods" {
  type    = number
  default = 60
}

variable "aws_lb_controller_iam" {
  type    = string
  default = "aws_load_balancer_controller"
}

variable "public_cloud" {
  type    = bool
  default = false
}

variable "enable_cache_registry" {
  type    = string
  default = "false"
}

variable "cache_registry" {
  type    = string
  default = ""
}

variable "additional_iam_policies" {
  type    = map(string)
  default = {}
}

variable "additional_node_groups" {
  type = map(object({
    desired_size : number
    min_size : number
    max_size : number
    taints : map(object({
      value : string
      effect : string # one of "NO_SCHEDULE", "NO_EXECUTE", "PREFER_NO_SCHEDULE"
    }))
    labels : map(string)
    instance_type : string
    volume_size : number
    volume_type : string
  }))
  default = {}
}

variable "cas_namespace" {
  description = "The Kubernetes namespace for cluster autoscaler"
  default     = "kube-system"
}

variable "cas_service_account" {
  description = "The Kubernetes service account name of cluster autoscaler"
  default     = "cluster-autoscaler"
}

variable "customer_tag" {
  description = "For customers to specify a tag for AWS resources"
  type = string
  default = ""
}

variable "user_script_pre_cloud_init" {
  type = string
  default = ""
  description = "Script to run before cloud-init"
}

variable "user_script_post_cloud_init" {
  type = string
  default = ""
  description = "Script to run after cloud-init"
}

variable "http_proxy" {
  type = string
  default = ""
  description = "HTTP proxy to use for EKS nodes"
}

variable "https_proxy" {
  type = string
  default = ""
  description = "HTTPS proxy to use for EKS nodes"
}

variable "no_proxy" {
  type = string
  default = ""
  description = "No proxy to use for EKS nodes"
}

variable "eks_cluster_dns_ip" {
  type = string
  default = ""
  description = "IP address of the DNS server for the cluster, leave empty to use the default DNS server"
}
