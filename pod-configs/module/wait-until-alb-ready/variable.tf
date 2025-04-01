# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "traefik_alb_arn" {
  description = "The ARN of the Traefik ALB to wait for"
}

variable "argocd_alb_arn" {
  description = "The ARN of the ArgoCD ALB to wait for"
}

variable "timeout" {
  default = 300
  description = "The maximum time in seconds to wait for the ALB to be ready"
}
