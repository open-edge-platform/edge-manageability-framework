# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.72.1"
    }
  }
}

locals {
  subnets_with_eip = var.internal ? [] : var.subnets
  protocol_mapping = {
    "TCP": "TCP",
    "TLS": "TCP",
    "UDP": "UDP",
    "HTTP": "TCP",
    "HTTPS": "TCP"
  }
}

resource "aws_security_group" "common" {
  name   = "${var.cluster_name}-${var.name}-load-balancer-sg"
  vpc_id = var.vpc_id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
  dynamic "ingress" {
    for_each = var.ports
    content {
      from_port = ingress.value.listen
      to_port = ingress.value.listen
      protocol = local.protocol_mapping[ingress.value.protocol]
      cidr_blocks = var.ip_allow_list
    }
  }
  tags = {
    Name : "${var.cluster_name}-${var.name}-load-balancer-sg"
  }
}

# Create EIP(if not internal), NLB, Listener, TargetGroup
resource "aws_eip" "main" {
  for_each = local.subnets_with_eip
}
resource "aws_lb" "main" {
  name               = substr(sha256("${var.cluster_name}-${var.name}"), 0, 32)
  internal           = var.internal
  load_balancer_type = var.type
  subnets            = var.internal ? var.subnets : null
  dynamic "subnet_mapping" {
    for_each = local.subnets_with_eip
    content {
      subnet_id     = subnet_mapping.key
      allocation_id = aws_eip.main[subnet_mapping.key].id
    }
  }
  enable_deletion_protection = var.enable_deletion_protection
  security_groups            = [aws_security_group.common.id]
  lifecycle {
    ignore_changes = [subnets, subnet_mapping]
  }
  tags = {
    Name = "${var.cluster_name}-${var.name}"
  }
}

resource "aws_lb_target_group" "main" {
  for_each    = var.ports
  name        = substr(sha256("${var.cluster_name}-${var.name}-${each.key}"), 0, 32)
  port        = each.value.target
  protocol    = each.value.protocol
  vpc_id      = var.vpc_id
  target_type = each.value.type
  health_check {
    enabled = each.value.enable_health_check
    port = "traffic-port"
    protocol = each.value.health_check_protocol
    path = each.value.health_check_path
    healthy_threshold = each.value.health_check_healthy_threshold
    unhealthy_threshold = each.value.health_check_unhealthy_threshold
  }
  tags = {
    Name = "${var.cluster_name}-${var.name}-${each.key}"
  }
}

resource "aws_lb_listener" "main" {
  for_each          = var.ports
  load_balancer_arn = aws_lb.main.arn
  port              = each.value.listen
  protocol          = each.value.protocol
  certificate_arn   = each.value.certificate_arn
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.main[each.key].arn
  }
}