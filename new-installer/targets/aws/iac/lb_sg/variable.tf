# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "cluster_name" {
  type        = string
  description = "The name of the cluster"
}

variable "customer_tag" {
  type    = string
  default = ""
}

variable "region" {
  type = string
}

variable "eks_node_sg_id" {
  type        = string
  description = "The security group ID of the EKS node group"
}

variable "traefik_sg_id" {
  type        = string
  description = "The security group ID for Traefik"
}

variable "traefik2_sg_id" {
  type        = string
  description = "The security group ID for Traefik2"
}

variable "argocd_sg_id" {
  type        = string
  description = "The security group ID for ArgoCD"
}
