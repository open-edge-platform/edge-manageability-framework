# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "host" {
  value = aws_rds_cluster.main.endpoint
}

output "host_reader" {
  value = aws_rds_cluster.main.reader_endpoint
}

output "port" {
  value = "5432"
}

output "username" {
  value = var.username
}

output "password" {
  value     = jsondecode(data.aws_secretsmanager_secret_version.rds_master_password.secret_string)["password"]
  sensitive = true
}
output "password_id" {
  value     = data.aws_secretsmanager_secret.rds_master_password.id
}
