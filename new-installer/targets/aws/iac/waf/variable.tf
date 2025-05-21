# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "cluster_name" {
  description = "The name of the cluster"
  type        = string
}
variable "region" {
  type = string
}
variable "customer_tag" {
  type    = string
  default = ""
}
variable "traefik_load_balancer_arn" {
  type    = string
}
variable "argocd_load_balancer_arn" {
  type    = string
}
