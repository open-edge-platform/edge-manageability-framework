# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_security_group" "alb" {
  name   = "${var.name}-alb"
  vpc_id = var.vpc_id
}

resource "aws_security_group_rule" "sg_to_alb" {
  type                     = "ingress"
  from_port                = 443
  to_port                  = 443
  protocol                 = "tcp"
  cidr_blocks              = var.ip_allow_list
  security_group_id        = aws_security_group.alb.id
}

resource "aws_security_group_rule" "alb_to_ecs_egress" {
  type                     = "egress"
  from_port                = 8443
  to_port                  = 8443
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.ecs_service.id
  security_group_id        = aws_security_group.alb.id
}

resource "aws_lb" "pull_through_cache_proxy" {
  name               = "${var.name}-ptcp"
  internal           = true
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = var.subnet_ids

  enable_deletion_protection = false
  idle_timeout               = 60
}

# The certificate can be one with full chain, we need to extract the leaf one.
data "tls_certificate" "cert_top" {
  content = var.tls_cert_body
}

resource "aws_acm_certificate" "cert" {
  private_key       = var.tls_cert_key
  certificate_body  = one(data.tls_certificate.cert_top.certificates).cert_pem
  certificate_chain = var.tls_cert_body
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.pull_through_cache_proxy.arn
  port              = 443
  protocol          = "HTTPS"

  ssl_policy      = "ELBSecurityPolicy-TLS13-1-2-Res-FIPS-2023-04"
  certificate_arn = aws_acm_certificate.cert.arn

  default_action {
    type = "forward"
    forward {
      target_group {
        arn = aws_lb_target_group.pull_through_cache_proxy.arn
      }
    }
  }
}

resource "aws_lb_target_group" "pull_through_cache_proxy" {
  name        = var.name
  port        = 8443
  protocol    = "HTTPS"
  vpc_id      = var.vpc_id
  target_type = "ip"

  health_check {
    path                = "/v2/"
    interval            = 30
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 2
    protocol            = "HTTPS"
    matcher             = 200
  }
}
