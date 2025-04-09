# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.94.1"
    }
  }
}

locals {
  protocol_mapping = {
    "HTTP" : "TCP",
    "HTTPS" : "TCP"
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
    for_each = var.listeners
    content {
      from_port   = ingress.value.listen
      to_port     = ingress.value.listen
      protocol    = local.protocol_mapping[ingress.value.protocol]
      cidr_blocks = var.ip_allow_list
    }
  }
  tags = {
    Name : "${var.cluster_name}-${var.name}-load-balancer-sg"
  }
}

# Create EIP(if not internal), NLB, Listener, TargetGroup
resource "aws_lb" "main" {
  name                       = substr(sha256("${var.cluster_name}-${var.name}"), 0, 32)
  internal                   = var.internal
  load_balancer_type         = "application"
  subnets                    = var.subnets
  enable_deletion_protection = var.enable_deletion_protection
  security_groups            = [aws_security_group.common.id]
  tags = {
    Name = "${var.cluster_name}-${var.name}"
  }
}

resource "aws_lb_target_group" "main" {
  for_each         = var.target_groups
  name             = substr(sha256("${var.cluster_name}-${var.name}-${each.key}"), 0, 32)
  port             = each.value.target
  protocol         = each.value.protocol
  protocol_version = each.value.protocol_version
  vpc_id           = var.vpc_id
  target_type      = each.value.type
  health_check {
    enabled             = each.value.enable_health_check
    port                = "traffic-port"
    protocol            = each.value.health_check_protocol
    path                = each.value.health_check_path
    healthy_threshold   = each.value.health_check_healthy_threshold
    unhealthy_threshold = each.value.health_check_unhealthy_threshold
    matcher             = each.value.expected_health_check_status_code
  }
  tags = {
    Name = "${var.cluster_name}-${var.name}-${each.key}"
  }
}

resource "aws_lb_listener" "main" {
  for_each          = var.listeners
  load_balancer_arn = aws_lb.main.arn
  port              = each.value.listen
  protocol          = each.value.protocol
  certificate_arn   = each.value.certificate_arn
  ssl_policy        = var.default_ssl_policy
  default_action {
    type             = lookup(var.target_groups, "default", null) != null ? "forward" : "fixed-response"
    dynamic "forward" {
      for_each = lookup(var.target_groups, "default", null) != null ? ["forward"] : []
      content {
        target_group {
          arn = aws_lb_target_group.main["default"].arn
        }
      }
    }
    dynamic "fixed_response" {
      for_each = lookup(var.target_groups, "default", null) != null ? [] : ["fixed_response"]
      content {
        content_type = "text/plain"
        message_body = "Not found"
        status_code  = "404"
      }
    }
  }
}

locals {
  tg_with_match_headers = { for name, target in var.target_groups : name => target if length(target.match_headers) != 0 }
  tg_with_match_hosts = { for name, target in var.target_groups : name => target if length(target.match_hosts) != 0 }
}

resource "aws_lb_listener_rule" "match_headers" {
  for_each     = local.tg_with_match_headers
  listener_arn = aws_lb_listener.main[each.value.listener].arn
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.main[each.key].arn
  }
  condition {
    dynamic "http_header" {
      for_each = each.value.match_headers
      content {
        http_header_name = http_header.key
        values           = [http_header.value]
      }
    }
  }
}

resource "aws_lb_listener_rule" "match_hosts" {
  for_each     = local.tg_with_match_hosts
  listener_arn = aws_lb_listener.main[each.value.listener].arn
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.main[each.key].arn
  }
  condition {
    host_header {
      values = each.value.match_hosts
    }
  }
}
