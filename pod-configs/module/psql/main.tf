# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

data "aws_secretsmanager_secret_version" "rds_master_password" {
  secret_id = var.password_id
}
locals {
  password = jsondecode(data.aws_secretsmanager_secret_version.rds_master_password.secret_string)["password"]
}

resource "null_resource" "wait_until_aurora_available" {

  provisioner "local-exec" {
    environment = {
      PGHOST : var.host
      PGPORT : var.port
      PGUSER : var.username
      PGPASSWORD : local.password
      PGDATABASE : var.database
    }
    command = <<-EOT
#!/bin/bash
MAX_RETRIES=60
RETRY_COUNT=0
while true; do
  if psql -c 'select VERSION();'; then
    break
  fi
  RETRY_COUNT=$((RETRY_COUNT + 1))
  if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
    echo "Maximum number of retries reached. Exiting..."
    exit 1
  fi
  sleep 5
done
    EOT
  }
}

resource "null_resource" "psql" {
  depends_on = [ null_resource.wait_until_aurora_available ]
  lifecycle {
    ignore_changes = [ triggers ]
  }
  triggers = {
    host = var.host
    port = var.port
    username = var.username
    database = var.database
    command_destroy = var.command_destroy
    password_id = var.password_id
  }
  provisioner "local-exec" {
    when = create
    environment = {
      PGHOST : var.host
      PGPORT : var.port
      PGUSER : var.username
      PGPASSWORD : local.password
      PGDATABASE : var.database
    }
    interpreter = ["psql", "-c"]
    command     = var.command_create
  }
  provisioner "local-exec" {
    when = destroy
    environment = {
      PGHOST : self.triggers.host
      PGPORT : self.triggers.port
      PGUSER : self.triggers.username
      PGDATABASE : self.triggers.database
    }
    interpreter = ["/bin/bash", "-c"]
    command     =<<-EOF
#!/bin/bash
export PGPASSWORD=$(aws secretsmanager get-secret-value --output json --no-paginate --secret-id '${self.triggers.password_id}' | jq -r .SecretString | jq -r .password)
psql -c '${self.triggers.command_destroy}'
    EOF
  }
}
