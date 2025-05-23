# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "bindings" {
  type = map(object({
    serviceNamespace = string
    serviceName = string
    servicePort = number
    target_id = string
  }))
  default = {}
}
