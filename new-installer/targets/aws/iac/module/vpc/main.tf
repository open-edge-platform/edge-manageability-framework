# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_vpc" "main" {
  cidr_block           = var.cidr_block
  enable_dns_hostnames = var.enable_dns_hostnames
  enable_dns_support   = var.enable_dns_support
  tags = {
    Name : var.name
  }
}
resource "aws_vpc_ipv4_cidr_block_association" "secondary_cidr" {
  for_each   = var.additional_cidr_blocks
  vpc_id     = aws_vpc.main.id
  cidr_block = each.value
}
