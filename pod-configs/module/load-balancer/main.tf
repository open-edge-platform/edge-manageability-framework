# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

data "aws_vpc" "main" {
  id = var.vpc_id
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

  # VPC DNS resolver is always at .2 address
  vpc_dns_resolver = format("%s/32", cidrhost(data.aws_vpc.main.cidr_block, 2))

  # AWS NTP service (same across all regions)
  aws_ntp_server = "169.254.169.123/32"
}

#data "dns_a_record_set" "letsencrypt" {
#  host = "acme-v02.api.letsencrypt.org"
#}

resource "aws_security_group" "common" {
  name   = "${var.cluster_name}-${var.name}-load-balancer-sg"
  vpc_id = var.vpc_id

  # All TCP traffic within VPC only
  # This covers:
  # - EKS pods (health checks, forwarding traffic)
  # - RDS databases (connections)
  # - VPC endpoints (AWS API calls via existing VPC endpoints)

  egress {
    from_port   = 0
    to_port     = 65535
    protocol    = "tcp"
    cidr_blocks = [data.aws_vpc.main.cidr_block]
    description = "All TCP within VPC (EKS pods, RDS, existing VPC endpoints)"
  }

  # UDP traffic within VPC (DNS resolution)
  egress {
    from_port   = 0
    to_port     = 65535
    protocol    = "udp"
    cidr_blocks = [data.aws_vpc.main.cidr_block]
    description = "All UDP within VPC (DNS resolution)"
  }

  # DNS to VPC resolver only
  egress {
    from_port   = 53
    to_port     = 53
    protocol    = "udp"
    cidr_blocks = [local.vpc_dns_resolver]
    description = "DNS to VPC resolver"
  }

  # NTP to AWS time service only
  egress {
    from_port   = 123
    to_port     = 123
    protocol    = "udp"
    cidr_blocks = [local.aws_ntp_server]
    description = "NTP to AWS time service"
  }

  # External HTTPS
  egress {
      from_port   = 443
      to_port     = 443
      protocol    = "tcp"
      #cidr_blocks = [for ip in data.dns_a_record_set.letsencrypt.addrs : "${ip}/32"]
      cidr_blocks = ["0.0.0.0/0"]
      description = "HTTPS to Tinkerbell server for certificate retrieval and letsencrypt"
  }

  # Custom external egress (if explicitly needed)
  dynamic "egress" {
    for_each = length(var.external_egress_rules) > 0 ? var.external_egress_rules : []
    content {
      from_port   = egress.value.from_port
      to_port     = egress.value.to_port
      protocol    = egress.value.protocol
      cidr_blocks = egress.value.cidr_blocks
      description = egress.value.description
    }
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
  tags = {
    Name = "${var.cluster_name}-nlb"
  }
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
