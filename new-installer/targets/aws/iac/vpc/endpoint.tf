# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
 endpoints = {
    "elasticfilesystem" : {
      private_dns_enabled = true
    }
    "s3" : {
      private_dns_enabled = false
    }
    "eks" : {
      private_dns_enabled = true
    }
    "sts" : {
      private_dns_enabled = true
    }
    "ec2" : {
      private_dns_enabled = true
    }
    "ec2messages" : {
      private_dns_enabled = true
    }
    "ecr.dkr" : {
      private_dns_enabled = true
    }
    "ecr.api" : {
      private_dns_enabled = true
    }
    "elasticloadbalancing" : {
      private_dns_enabled = true
    }
  }
}

resource "aws_security_group" "vpc_endpoints" {
  name   = var.endpoint_sg_name
  vpc_id = aws_vpc.main.id
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = var.cidr_block
    description = "Allow HTTPS traffic from VPC"
  }
  description = "Allow HTTPS traffic from VPC"
}

resource "aws_vpc_endpoint" "endpoint" {
  for_each          = local.endpoints
  vpc_id            = aws_vpc.main.id
  service_name      = "com.amazonaws.${var.region}.${each.key}"
  vpc_endpoint_type = "Interface"

  subnet_ids = [for k, v in aws_subnet.private_subnet : v.id]
  security_group_ids = [
    aws_security_group.vpc_endpoints.id
  ]

  private_dns_enabled = each.value.private_dns_enabled
  tags = {
    VPC  = "${var.name}"
    Name = "${var.name}-${each.key}-endpoint"
  }
}
