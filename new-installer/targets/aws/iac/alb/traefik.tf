# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_security_group" "traefik" {
  name   = "${var.cluster_name}-traefik-load-balancer-sg"
  vpc_id = var.vpc_id
  description = "Security group for the infrastructure load balancer for cluster ${var.cluster_name}"
  tags = {
    Name : "${var.cluster_name}-traefik-load-balancer-sg"
    environment: var.cluster_name
  }
}

resource "aws_security_group_rule" "traefik_allow_https" {
  type              = "ingress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = tolist(local.ip_allow_list)
  security_group_id = aws_security_group.traefik.id
  description = "Allow HTTPS traffic from IP allow list"
}

#trivy:ignore:AVD-AWS-0053 Allow public access to the load balancer
resource "aws_lb" "traefik" {
  name                       = substr(sha256("${var.cluster_name}-traefik"), 0, 32)
  internal                   = var.internal
  load_balancer_type         = "application"
  subnets                    = var.public_subnet_ids
  enable_deletion_protection = var.enable_deletion_protection
  security_groups            = [aws_security_group.traefik.id]
  drop_invalid_header_fields = true
  tags = {
    Name = "${var.cluster_name}-traefik"
  }
}

resource "aws_lb_target_group" "traefik" {
  name             = substr(sha256("${var.cluster_name}-traefik"), 0, 32)
  port             = 1
  protocol         = "HTTPS"
  protocol_version = "HTTP1"
  vpc_id           = var.vpc_id
  target_type      = "ip"
  health_check {
    enabled             = true
    port                = "traffic-port"
    protocol            = "HTTPS"
    path                = "/"
    healthy_threshold   = 5
    unhealthy_threshold = 2
    matcher             = 404
  }
  tags = {
    Name = "${var.cluster_name}-traefik"
  }
}

resource "aws_lb_target_group" "traefik_grpc" {
  name             = substr(sha256("${var.cluster_name}-traefik-grpc"), 0, 32)
  port             = 1
  protocol         = "HTTPS"
  protocol_version = "GRPC"
  vpc_id           = var.vpc_id
  target_type      = "ip"
  health_check {
    enabled             = true
    port                = "traffic-port"
    protocol            = "HTTPS"
    path                = "/"
    healthy_threshold   = 5
    unhealthy_threshold = 2
    matcher             = 0
  }
  tags = {
    Name = "${var.cluster_name}-traefik"
  }
}

resource "aws_lb_listener" "traefik" {
  load_balancer_arn = aws_lb.traefik.arn
  port              = 443
  protocol          = "HTTPS"
  certificate_arn   = var.tls_cert_arn
  ssl_policy        = local.ssl_polocy
  default_action {
    type = "forward"
    forward {
      target_group {
        arn = aws_lb_target_group.traefik.arn
      }
    }
  }
}

resource "aws_lb_listener_rule" "traefik_grpc" {
  listener_arn = aws_lb_listener.traefik.arn
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.traefik_grpc.arn
  }
  condition {
    http_header {
      http_header_name = "content-type"
      values           = ["application/grpc*"]
    }
  }
}
