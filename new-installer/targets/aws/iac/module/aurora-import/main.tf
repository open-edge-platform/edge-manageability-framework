# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  secret_name        = "${var.database}-aurora-postgresql"
  secret_name_reader = "${var.database}-reader-aurora-postgresql"
}
resource "kubernetes_secret" "db" {
  metadata {
    name      = local.secret_name
    namespace = var.namespace
  }
  data = {
    PGHOST     = var.host
    PGPORT     = var.port
    PGUSER     = var.username
    PGPASSWORD = var.password
    password   = var.password
    PGDATABASE = "${var.namespace}-${var.database}"
  }
  type = "Opaque"
}
resource "kubernetes_secret" "db_reader" {
  metadata {
    name      = local.secret_name_reader
    namespace = var.namespace
  }
  data = {
    PGHOST     = var.host_reader
    PGPORT     = var.port
    PGUSER     = var.username
    PGPASSWORD = var.password
    password   = var.password
    PGDATABASE = "${var.namespace}-${var.database}"
  }
  type = "Opaque"
}
