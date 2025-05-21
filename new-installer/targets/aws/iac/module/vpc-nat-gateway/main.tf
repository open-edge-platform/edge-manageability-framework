# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_eip" "ngw" {
  for_each = var.subnets_with_eip
}

data "aws_subnet" "subnets_with_eip" {
  for_each          = var.subnets_with_eip
  availability_zone = each.value.az
  cidr_block        = each.value.cidr_block
  vpc_id            = var.vpc.id
}

resource "aws_nat_gateway" "ngw_with_eip" {
  for_each      = var.subnets_with_eip
  allocation_id = aws_eip.ngw[each.key].id
  subnet_id     = data.aws_subnet.subnets_with_eip[each.key].id
  tags = {
    Name = "${var.vpc_name}-${each.key}-ngw"
    VPC  = "${var.vpc_name}"
  }
}

data "aws_subnet" "ngw_without_eip" {
  for_each          = var.subnets_without_eip
  availability_zone = each.value.az
  cidr_block        = each.value.cidr_block
  vpc_id            = var.vpc.id
}

resource "aws_nat_gateway" "ngw_without_eip" {
  for_each  = var.subnets_without_eip
  subnet_id = data.aws_subnet.ngw_without_eip[each.key].id
  tags = {
    Name = "${var.vpc_name}-${each.key}-ngw"
    VPC  = "${var.vpc_name}"
  }
}
