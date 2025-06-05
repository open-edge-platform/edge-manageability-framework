# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_security_group" "infra" {
  name   = "${var.cluster_name}-infra-load-balancer-sg"
  vpc_id = var.vpc_id
  tags = {
    Name : "${var.cluster_name}-infra-load-balancer-sg"
    environment : var.cluster_name
  }
  description = "Security group for the infrastructure load balancer for cluster ${var.cluster_name}"
}

resource "aws_security_group_rule" "infra_allow_https" {
  type              = "ingress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = tolist(local.ip_allow_list)
  security_group_id = aws_security_group.infra.id
  description = "Allow HTTPS traffic from IP allow list"
}

#trivy:ignore:AVD-AWS-0053 Allow public access to the load balancer
resource "aws_lb" "infra" {
  name                       = substr(sha256("${var.cluster_name}-argocd"), 0, 32)
  internal                   = var.internal
  load_balancer_type         = "application"
  subnets                    = var.public_subnet_ids
  enable_deletion_protection = var.enable_deletion_protection
  security_groups            = [aws_security_group.infra.id]
  drop_invalid_header_fields = true
  tags = {
    Name = "${var.cluster_name}-infra"
  }
}

resource "aws_lb_target_group" "infra_argocd" {
  name             = substr(sha256("${var.cluster_name}-infra-argocd"), 0, 32)
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
    matcher             = 200
  }
  tags = {
    Name = "${var.cluster_name}-infra-argocd"
  }
}

resource "aws_lb_target_group" "infra_gitea" {
  name             = substr(sha256("${var.cluster_name}-infra-gitea"), 0, 32)
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
    matcher             = 200
  }
  tags = {
    Name = "${var.cluster_name}-infra-argocd"
  }
}

resource "aws_lb_listener" "infra" {
  load_balancer_arn = aws_lb.infra.arn
  port              = 443
  protocol          = "HTTPS"
  certificate_arn   = var.tls_cert_arn
  ssl_policy        = local.ssl_polocy
  default_action {
    type             = "fixed-response"
    fixed_response {
      content_type = "text/plain"
      message_body = "Not Found"
      status_code  = "404"
    }
  }
}

resource "aws_lb_listener_rule" "infra_argocd" {
  listener_arn = aws_lb_listener.infra.arn
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.infra_argocd.arn
  }
  condition {
    host_header {
      values = ["argocd.*"]
    }
  }
}

resource "aws_lb_listener_rule" "infra_gitea" {
  listener_arn = aws_lb_listener.infra.arn
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.infra_gitea.arn
  }
  condition {
    host_header {
      values = ["gitea.*"]
    }
  }
}
