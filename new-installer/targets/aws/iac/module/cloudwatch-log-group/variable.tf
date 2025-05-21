# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "name" {}

variable "retention_in_days" {
  description = "pecifies the number of days you want to retain log events in the specified log group."
  default = 90
  type = number
}
