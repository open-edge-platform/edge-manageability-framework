# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_route_table" "public_subnet" {
  for_each = aws_subnet.public_subnet
  vpc_id   = aws_vpc.main.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.igw.id
  }
  tags = {
    Name = "${var.name}-public-subnet-${each.key}"
    VPC  = "${var.name}"
  }
}


locals {
  private_subnet_to_ngw = zipmap(keys(var.private_subnets), keys(aws_nat_gateway.main))
}

resource "aws_route_table" "private_subnet" {
  for_each = local.private_subnet_to_ngw
  vpc_id   = aws_vpc.main.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_nat_gateway.main[each.value].id
  }
  tags = {
    Name = "${var.name}-private-subnet-${each.key}"
    VPC  = "${var.name}"
  }
}

resource "aws_route_table_association" "public_subnet" {
  for_each       = aws_subnet.public_subnet
  subnet_id      = each.value.id
  route_table_id = aws_route_table.public_subnet[each.key].id
}

resource "aws_route_table_association" "private_subnet" {
  for_each       = aws_subnet.private_subnet
  subnet_id      = each.value.id
  route_table_id = aws_route_table.private_subnet[each.key].id
}
