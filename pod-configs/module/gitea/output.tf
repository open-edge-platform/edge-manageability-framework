# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "gitea_master_password" {
  value = random_password.gitea_master_password.result
}
output "gitea_user_passwords" {
  value = { for user, password in random_password.gitea_user_password : user => password.result }
}
