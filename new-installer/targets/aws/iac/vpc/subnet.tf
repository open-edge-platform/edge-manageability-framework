# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_subnet" "private_subnet" {
  for_each          = var.private_subnets
  vpc_id            = aws_vpc.main.id
  cidr_block        = each.value.cidr_block
  availability_zone = each.value.az

  tags = {
    Network                           = "Private"
    Name                              = "${var.name}-${each.key}"
    "kubernetes.io/role/internal-elb" = 1
  }
}

resource "aws_subnet" "public_subnet" {
  for_each          = var.public_subnets
  vpc_id            = aws_vpc.main.id
  cidr_block        = each.value.cidr_block
  availability_zone = each.value.az

  tags = {
    Network                  = "Public"
    Name                     = "${var.name}-${each.key}"
    "kubernetes.io/role/elb" = 1
  }
}
