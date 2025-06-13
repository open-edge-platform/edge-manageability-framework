# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

data "aws_caller_identity" "current" {}

locals {
  account_id = data.aws_caller_identity.current.account_id
}

resource "aws_secretsmanager_secret" "pull_through_cache_proxy" {
  name        = "${var.cluster_name}-ptcp"
  description = "Nginx configs and TLS certs for Docker cache"
  tags = {
    Name = var.cluster_name
  }
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "pull_through_cache_proxy" {
  secret_id     = aws_secretsmanager_secret.pull_through_cache_proxy.id
  secret_string = jsonencode({
    tls_cert_key   = var.tls_cert_key,
    tls_cert_body  = var.tls_cert_body,
    nginx_config = templatefile("${path.module}/nginx.conf.tpl", {
      backend_registry = "${local.account_id}.dkr.ecr.${var.region}.amazonaws.com"
    }),
    nginx_entrypoint = file("${path.module}/nginx-entrypoint.sh")
  })
}
