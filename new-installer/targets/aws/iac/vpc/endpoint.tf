# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  endpoint_private_dns_enabled = {
    "elasticfilesystem" : true,
    "s3" : false,
    "eks" : true,
    "sts" : true,
    "ec2" : true,
    "ec2messages" : true,
    "ecr.dkr" : true,
    "ecr.api" : true,
    "elasticloadbalancing" : true,
    "ecs" : true,
  }
}

resource "aws_security_group" "vpc_endpoints" {
  name   = var.endpoint_sg_name
  vpc_id = aws_vpc.main.id
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = [var.cidr_block]
    description = "Allow HTTPS traffic from VPC"
  }
  description = "Allow HTTPS traffic from VPC"
}

resource "aws_vpc_endpoint" "endpoint" {
  for_each          = var.endpoints
  vpc_id            = aws_vpc.main.id
  service_name      = "com.amazonaws.${var.region}.${each.key}"
  vpc_endpoint_type = "Interface"

  subnet_ids = [for k, v in aws_subnet.private_subnet : v.id]
  security_group_ids = [
    aws_security_group.vpc_endpoints.id
  ]

  private_dns_enabled = local.endpoint_private_dns_enabled[each.key]
  tags = {
    VPC  = "${var.name}"
    Name = "${var.name}-${each.key}-endpoint"
  }
}
