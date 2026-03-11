# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  rds_identifier = "${var.cluster_name}-aurora-postgresql"
}

resource "aws_security_group" "rds" {
  vpc_id = var.vpc_id
  name   = local.rds_identifier
  # Allow connections only from EKS subnets
  ingress {
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = var.ip_allow_list
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = var.ip_allow_list
  }
}

resource "aws_db_subnet_group" "main" {
  name       = var.cluster_name
  subnet_ids = var.subnet_ids
}

resource "aws_rds_cluster" "main" {
  skip_final_snapshot         = var.dev_mode ? true : false
  deletion_protection         = var.dev_mode ? false : true
  cluster_identifier          = local.rds_identifier
  engine                      = "aurora-postgresql"
  engine_mode                 = "provisioned"
  engine_version              = "${var.postgres_ver_major}.${var.postgres_ver_minor}"
  availability_zones          = var.availability_zones
  master_username             = var.username
  manage_master_user_password = true
  backup_retention_period     = var.dev_mode ? 7 : 30 // Days
  # What timezone?
  preferred_backup_window         = "02:00-03:00"
  db_subnet_group_name            = aws_db_subnet_group.main.name
  vpc_security_group_ids          = [aws_security_group.rds.id]
  final_snapshot_identifier       = "${local.rds_identifier}-final-snapshot"
  enabled_cloudwatch_logs_exports = ["postgresql"]
  db_cluster_parameter_group_name = aws_rds_cluster_parameter_group.default.name
  apply_immediately               = var.dev_mode ? true : false
  # What timezone?
  preferred_maintenance_window = "Sat:04:00-Sat:05:00"
  storage_encrypted            = true

  serverlessv2_scaling_configuration {
    # 1 ACU ~= 2GB memory
    min_capacity = var.min_acus
    max_capacity = var.max_acus
  }
}

resource "aws_rds_cluster_parameter_group" "default" {
  name        = "${local.rds_identifier}-cluster-pg"
  family      = "aurora-postgresql${var.postgres_ver_major}"
  description = "${local.rds_identifier} default cluster parameter group"

  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }
}

resource "aws_rds_cluster_instance" "main" {
  // Create one instance per AZ to withstand failure of any AZ
  for_each                              = var.instance_availability_zones
  identifier                            = "${local.rds_identifier}-instance-${each.key}"
  cluster_identifier                    = aws_rds_cluster.main.id
  instance_class                        = "db.serverless"
  engine                                = aws_rds_cluster.main.engine
  engine_version                        = aws_rds_cluster.main.engine_version
  performance_insights_enabled          = true
  performance_insights_retention_period = 31
  availability_zone                     = each.key
  ca_cert_identifier                    = var.ca_cert_identifier
  # monitoring_interval = 60
  # monitoring_role_arn = ""
}

data "aws_secretsmanager_secret" "rds_master_password" {
  arn = aws_rds_cluster.main.master_user_secret[0].secret_arn
}
data "aws_secretsmanager_secret_version" "rds_master_password" {
  secret_id = data.aws_secretsmanager_secret.rds_master_password.id
}
