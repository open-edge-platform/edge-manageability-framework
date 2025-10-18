# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Get current region and VPC information
data "aws_region" "current" {}
data "aws_vpc" "main" {
  id = var.vpc_id
}

# Get route table for VPC endpoints
data "aws_route_tables" "private" {
  vpc_id = var.vpc_id
  filter {
    name   = "tag:Name"
    values = ["*private*"]
  }
}

# Check for existing VPC endpoints
data "aws_vpc_endpoint" "existing" {
  vpc_id = var.vpc_id
  filter {
    name   = "service-name"
    values = [
      "com.amazonaws.${data.aws_region.current.name}.elasticloadbalancing",
      "com.amazonaws.${data.aws_region.current.name}.eks",
      "com.amazonaws.${data.aws_region.current.name}.ecr.api",
      "com.amazonaws.${data.aws_region.current.name}.ecr.dkr",
      "com.amazonaws.${data.aws_region.current.name}.secretsmanager",
      "com.amazonaws.${data.aws_region.current.name}.ssm",
      "com.amazonaws.${data.aws_region.current.name}.acm",
      "com.amazonaws.${data.aws_region.current.name}.route53"
    ]
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

  # VPC DNS resolver is always at .2 address
  vpc_dns_resolver = format("%s/32", cidrhost(data.aws_vpc.main.cidr_block, 2))

  # AWS NTP service (same across all regions)
  aws_ntp_server = "169.254.169.123/32"

  # Create mapping of existing VPC endpoints by service name
  existing_endpoints = {
    for endpoint in data.aws_vpc_endpoint.existing.vpc_endpoint :
    endpoint.service_name => endpoint.id
  }

  # Determine which endpoints need to be created
  create_elasticloadbalancing = !contains(keys(local.existing_endpoints), "com.amazonaws.${data.aws_region.current.name}.elasticloadbalancing")
  create_eks                 = !contains(keys(local.existing_endpoints), "com.amazonaws.${data.aws_region.current.name}.eks")
  create_ecr_api            = !contains(keys(local.existing_endpoints), "com.amazonaws.${data.aws_region.current.name}.ecr.api")
  create_ecr_dkr            = !contains(keys(local.existing_endpoints), "com.amazonaws.${data.aws_region.current.name}.ecr.dkr")
  create_secretsmanager     = !contains(keys(local.existing_endpoints), "com.amazonaws.${data.aws_region.current.name}.secretsmanager")
  create_ssm                = !contains(keys(local.existing_endpoints), "com.amazonaws.${data.aws_region.current.name}.ssm")
  create_acm                = !contains(keys(local.existing_endpoints), "com.amazonaws.${data.aws_region.current.name}.acm")
  create_route53            = !contains(keys(local.existing_endpoints), "com.amazonaws.${data.aws_region.current.name}.route53")
}

# FIX: Create VPC endpoints security group first (no dependencies)
resource "aws_security_group" "vpc_endpoints" {
  name_prefix = "${var.cluster_name}-vpc-endpoints-"
  vpc_id      = var.vpc_id
  description = "Security group for VPC endpoints"

  # Allow HTTPS from entire VPC (no circular dependency)
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = [data.aws_vpc.main.cidr_block]
    description = "HTTPS from VPC"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = [data.aws_vpc.main.cidr_block]
    description = "All traffic within VPC"
  }

  tags = {
    Name = "${var.cluster_name}-vpc-endpoints-sg"
  }
}

# FIX: Load balancer security group references VPC endpoints SG (no cycle)
resource "aws_security_group" "common" {
  name   = "${var.cluster_name}-${var.name}-load-balancer-sg"
  vpc_id = var.vpc_id

  # SECURE: All TCP traffic within VPC only
  egress {
    from_port   = 0
    to_port     = 65535
    protocol    = "tcp"
    cidr_blocks = [data.aws_vpc.main.cidr_block]
    description = "All TCP within VPC (EKS pods, RDS, VPC endpoints)"
  }

  # SECURE: UDP traffic within VPC (DNS, NTP)
  egress {
    from_port   = 0
    to_port     = 65535
    protocol    = "udp"
    cidr_blocks = [data.aws_vpc.main.cidr_block]
    description = "All UDP within VPC (DNS resolution)"
  }

  # SECURE: DNS to VPC resolver only
  egress {
    from_port   = 53
    to_port     = 53
    protocol    = "udp"
    cidr_blocks = [local.vpc_dns_resolver]
    description = "DNS to VPC resolver"
  }

  # SECURE: NTP to AWS time service only
  egress {
    from_port   = 123
    to_port     = 123
    protocol    = "udp"
    cidr_blocks = [local.aws_ntp_server]
    description = "NTP to AWS time service"
  }

  dynamic "ingress" {
    for_each = var.ports
    content {
      from_port   = ingress.value.listen
      to_port     = ingress.value.listen
      protocol    = local.protocol_mapping[ingress.value.protocol]
      cidr_blocks = var.ip_allow_list
      description = "Ingress for ${ingress.key} on port ${ingress.value.listen}"
    }
  }

  tags = {
    Name = "${var.cluster_name}-${var.name}-load-balancer-sg"
  }
}

# VPC Endpoints for AWS Services
resource "aws_vpc_endpoint" "s3" {
  vpc_id            = var.vpc_id
  service_name      = "com.amazonaws.${data.aws_region.current.name}.s3"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = data.aws_route_tables.private.ids

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = "*"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:ListBucket"
        ]
        Resource = [
          "arn:aws:s3:::${var.cluster_name}-*",
          "arn:aws:s3:::${var.cluster_name}-*/*"
        ]
      }
    ]
  })

  tags = {
    Name = "${var.cluster_name}-s3-endpoint"
  }
}

resource "aws_vpc_endpoint" "ecr_api" {
  count               = local.create_ecr_api ? 1 : 0
  vpc_id              = var.vpc_id
  service_name        = "com.amazonaws.${data.aws_region.current.name}.ecr.api"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = var.subnets
  security_group_ids  = [aws_security_group.vpc_endpoints.id]
  private_dns_enabled = false

  tags = {
    Name = "${var.cluster_name}-ecr-api-endpoint"
  }
}

resource "aws_vpc_endpoint" "ecr_dkr" {
  count               = local.create_ecr_dkr ? 1 : 0
  vpc_id              = var.vpc_id
  service_name        = "com.amazonaws.${data.aws_region.current.name}.ecr.dkr"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = var.subnets
  security_group_ids  = [aws_security_group.vpc_endpoints.id]
  private_dns_enabled = false

  tags = {
    Name = "${var.cluster_name}-ecr-dkr-endpoint"
  }
}

resource "aws_vpc_endpoint" "eks" {
  count               = local.create_eks ? 1 : 0
  vpc_id              = var.vpc_id
  service_name        = "com.amazonaws.${data.aws_region.current.name}.eks"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = var.subnets
  security_group_ids  = [aws_security_group.vpc_endpoints.id]
  private_dns_enabled = false

  tags = {
    Name = "${var.cluster_name}-eks-endpoint"
  }
}

resource "aws_vpc_endpoint" "elasticloadbalancing" {
  count               = local.create_elasticloadbalancing ? 1 : 0
  vpc_id              = var.vpc_id
  service_name        = "com.amazonaws.${data.aws_region.current.name}.elasticloadbalancing"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = var.subnets
  security_group_ids  = [aws_security_group.vpc_endpoints.id]
  private_dns_enabled = false

  tags = {
    Name = "${var.cluster_name}-elb-endpoint"
  }
}

resource "aws_vpc_endpoint" "secretsmanager" {
  count               = local.create_secretsmanager ? 1 : 0
  vpc_id              = var.vpc_id
  service_name        = "com.amazonaws.${data.aws_region.current.name}.secretsmanager"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = var.subnets
  security_group_ids  = [aws_security_group.vpc_endpoints.id]
  private_dns_enabled = false

  tags = {
    Name = "${var.cluster_name}-secrets-manager-endpoint"
  }
}

resource "aws_vpc_endpoint" "ssm" {
  count               = local.create_ssm ? 1 : 0
  vpc_id              = var.vpc_id
  service_name        = "com.amazonaws.${data.aws_region.current.name}.ssm"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = var.subnets
  security_group_ids  = [aws_security_group.vpc_endpoints.id]
  private_dns_enabled = false

  tags = {
    Name = "${var.cluster_name}-ssm-endpoint"
  }
}

resource "aws_vpc_endpoint" "acm" {
  count               = local.create_acm ? 1 : 0
  vpc_id              = var.vpc_id
  service_name        = "com.amazonaws.${data.aws_region.current.name}.acm"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = var.subnets
  security_group_ids  = [aws_security_group.vpc_endpoints.id]
  private_dns_enabled = false

  tags = {
    Name = "${var.cluster_name}-acm-endpoint"
  }
}

resource "aws_vpc_endpoint" "route53" {
  count               = local.create_route53 ? 1 : 0
  vpc_id              = var.vpc_id
  service_name        = "com.amazonaws.${data.aws_region.current.name}.route53"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = var.subnets
  security_group_ids  = [aws_security_group.vpc_endpoints.id]
  private_dns_enabled = false

  tags = {
    Name = "${var.cluster_name}-route53-endpoint"
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