# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "eks_node_sg_id" {
  type = string
  description = "EKS node instance security group ID"
}

variable "lb_sg_ids" {
  type = map(object({
    security_group_id = string,
    port = number
  }))
  description = "Map of load balancer security group ID and port on EKS node, we use this to set up security group rule for load balancer to EKS node communication"
}
