# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "var1" {
  type = string
}

variable "var2" {
  type = number
}

resource "null_resource" "res1" {}
resource "null_resource" "res2" {}

output "output1" {
  value = var.var1
}

output "output2" {
  value = var.var2
}
