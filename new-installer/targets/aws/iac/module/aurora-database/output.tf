# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "database_user" {
  value = var.database_user
}
output "user_password" {
  value = random_password.user_password
  sensitive = true
}
