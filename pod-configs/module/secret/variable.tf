# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "name" {
  description = "The secret name"
}

variable "secret_string" {
  sensitive = true
  description = "The string value to store in the secret"
}

variable "recovery_window_in_days" {
  description = <<-EOF
  Number of days that AWS Secrets Manager waits before it can delete the secret.
  This value can be 0 to force deletion without recovery or range from 7 to 30 days.
  The default value is 0.
  EOF
  default = 0
  type = number
}
