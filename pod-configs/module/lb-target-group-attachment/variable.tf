# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "name"{
  type = string
  description =  "Name of the service"
}
variable "eks_nodes" {
  type = set(string)
  description =  "List of EKS node instance Ids"
  default = []
}
variable "eks_node_sg_id" {
  type = string
  description =  "EKS node instance security group Id"
  default = ""
}
variable "service_node_port" {
  type = string
  description =  "Node port at which the service is exposed"
}
variable "target_group_arn"{
  type = string
  description =  "Load balancer target group resource identifier to attach to EKS nodes"
}
variable "ip_allow_list" {
  type        = set(string)
  description = "List of IP sources to allow to connect to load balancers."
  default = []
}
