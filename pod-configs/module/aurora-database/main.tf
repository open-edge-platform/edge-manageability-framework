# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

module "database" {
  for_each        = var.databases
  source          = "../psql"
  host            = var.host
  port            = var.port
  username        = var.username
  password_id     = var.password_id
  command_create  = "CREATE DATABASE \"${each.key}\";"
  command_destroy = "DROP DATABASE \"${each.key}\";"
}
resource "random_password" "user_password" {
  for_each = var.users
  length   = 16
  special  = false
}
module "user" {
  depends_on      = [module.database]
  for_each        = var.users
  source          = "../psql"
  host            = var.host
  port            = var.port
  username        = var.username
  password_id     = var.password_id
  command_create  = "CREATE USER \"${each.key}\" WITH PASSWORD '${random_password.user_password[each.key].result}';"
  command_destroy = "DROP USER \"${each.key}\";"
}
module "grant_user" {
  for_each        = var.database_user
  depends_on      = [module.database, module.user]
  source          = "../psql"
  host            = var.host
  port            = var.port
  username        = var.username
  password_id     = var.password_id
  command_create  = <<EOT
    BEGIN;
    REVOKE ALL ON DATABASE "${each.key}" FROM PUBLIC;
    GRANT CONNECT ON DATABASE "${each.key}" TO "${each.value}";
    GRANT ALL PRIVILEGES ON DATABASE "${each.key}" TO "${each.value}";
    COMMIT;
  EOT
  command_destroy = <<EOT
    BEGIN;
    REVOKE ALL ON DATABASE "${each.key}" FROM PUBLIC;
    REVOKE CONNECT ON DATABASE "${each.key}" FROM "${each.value}";
    REVOKE ALL PRIVILEGES ON DATABASE "${each.key}" FROM "${each.value}";
    COMMIT;
  EOT
}
