# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "cluster_name" {
  description = "The name of the EKS cluster"
}

variable "http_proxy" {
  description = "The HTTP proxy to use for the AWS Load Balancer Controller"
  default = ""
}

variable "https_proxy" {
  description = "The HTTPS proxy to use for the AWS Load Balancer Controller"
  default = ""
}

variable "no_proxy" {
  description = "The no proxy to use for the AWS Load Balancer Controller"
  default = ""
}
