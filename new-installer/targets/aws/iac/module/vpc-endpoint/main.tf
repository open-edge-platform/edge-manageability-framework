# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_security_group" "vpc_endpoints" {
  name   = var.sg_name
  vpc_id = var.vpc.id
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = var.cidr_allow_list
  }
}

data "aws_subnet" "endpoint" {
  for_each          = var.subnets
  vpc_id            = var.vpc.id
  availability_zone = each.value.az
  cidr_block        = each.value.cidr_block
}

resource "aws_vpc_endpoint" "endpoint" {
  for_each          = var.endpoints
  vpc_id            = var.vpc.id
  service_name      = "com.amazonaws.${var.region}.${each.key}"
  vpc_endpoint_type = "Interface"

  subnet_ids = [for k, v in data.aws_subnet.endpoint : v.id]
  security_group_ids = [
    aws_security_group.vpc_endpoints.id
  ]

  private_dns_enabled = each.value.private_dns_enabled
  tags = {
    VPC  = "${var.vpc_name}"
    Name = "${var.vpc_name}-${each.key}-endpoint"
  }
}
