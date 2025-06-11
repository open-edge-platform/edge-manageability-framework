# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

data "aws_nat_gateways" "vpc_nat_gateways" {
  vpc_id = var.vpc_id
}

data "aws_nat_gateway" "vpc_nat_gateway" {
  for_each = toset(data.aws_nat_gateways.vpc_nat_gateways.ids)
  id       = each.value
  vpc_id   = var.vpc_id
  state    = "available"
}

locals {
  subnets_with_eip = var.internal ? [] : var.subnets
  nat_public_ips = toset([for id, nat in data.aws_nat_gateway.vpc_nat_gateway : "${nat.public_ip}/32" if nat.connectivity_type == "public"])
  ip_allow_list  = setunion(var.ip_allow_list, local.nat_public_ips)
  nlb_ports = {
    "https" : {
      listen              = 443
      target              = 31443
      type                = "ip"
      protocol            = "TCP"
      enable_health_check = true
    }
  }
}

resource "aws_security_group" "common" {
  name   = "${var.cluster_name}-network-load-balancer-sg"
  vpc_id = var.vpc_id
  tags = {
    Name : "${var.cluster_name}-network-load-balancer-sg"
  }
}

resource "aws_security_group_rule" "common" {
  type              = "ingress"
  from_port         = 443
  to_port           = 443
  protocol          = "TCP"
  cidr_blocks       = local.ip_allow_list
  security_group_id = aws_security_group.common.id

}

# Create EIP(if not internal), NLB, Listener, TargetGroup
resource "aws_eip" "main" {
  for_each = local.subnets_with_eip
}
resource "aws_lb" "main" {
  name               = substr(sha256("${var.cluster_name}-traefik2"), 0, 32)
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
    Name = "${var.cluster_name}-traefik2"
  }
}

resource "aws_lb_target_group" "main" {
  name        = substr(sha256("${var.cluster_name}-https"), 0, 32)
  port        = 31443
  protocol    = "TCP"
  vpc_id      = var.vpc_id
  target_type = "ip"
  health_check {
    enabled = true
    port = "traffic-port"
    protocol = "TCP"
    healthy_threshold = 5
    unhealthy_threshold = 2
  }
  tags = {
    Name = "${var.cluster_name}-https"
  }
}

resource "aws_lb_listener" "main" {
  load_balancer_arn = aws_lb.main.arn
  port              = 443
  protocol          = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.main.arn
  }
}
