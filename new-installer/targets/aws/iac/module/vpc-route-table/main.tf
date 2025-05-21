# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  public_route_subnets = var.set_up_public_route ? var.public_subnets : {}
}
resource "aws_route_table" "public_subnet" {
  for_each = local.public_route_subnets
  vpc_id   = var.vpc.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = var.internet_gateway.id
  }
  tags = {
    Name = "${var.vpc_name}-public-subnet-${each.key}"
    VPC  = "${var.vpc_name}"
  }
}

locals {
  private_subnet_to_ngw = zipmap(keys(var.private_subnets), keys(var.nat_gateways))
}

resource "aws_route_table" "private_subnet" {
  for_each = local.private_subnet_to_ngw
  vpc_id   = var.vpc.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = var.nat_gateways[each.value].id
  }
  tags = {
    Name = "${var.vpc_name}-private-subnet-${each.key}"
    VPC  = "${var.vpc_name}"
  }
}

data "aws_subnet" "public_subnet" {
  for_each          = local.public_route_subnets
  availability_zone = each.value.az
  cidr_block        = each.value.cidr_block
  vpc_id            = var.vpc.id
}

data "aws_subnet" "private_subnet" {
  for_each          = var.private_subnets
  availability_zone = each.value.az
  cidr_block        = each.value.cidr_block
  vpc_id            = var.vpc.id
}

resource "aws_route_table_association" "public_subnet" {
  for_each       = local.public_route_subnets
  subnet_id      = data.aws_subnet.public_subnet[each.key].id
  route_table_id = aws_route_table.public_subnet[each.key].id
}

resource "aws_route_table_association" "private_subnet" {
  for_each       = var.private_subnets
  subnet_id      = data.aws_subnet.private_subnet[each.key].id
  route_table_id = aws_route_table.private_subnet[each.key].id
}
