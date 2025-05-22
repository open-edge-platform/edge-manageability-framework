# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_eip" "ngw" {
  for_each = aws_subnet.public_subnet
}

resource "aws_nat_gateway" "main" {
  for_each      = aws_subnet.public_subnet
  allocation_id = aws_eip.ngw[each.key].id
  subnet_id     = each.value.id
  tags = {
    Name = "${var.name}-${each.key}-ngw"
    VPC  = "${var.name}"
  }
}
