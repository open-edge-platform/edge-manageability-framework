# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_internet_gateway" "igw" {
  vpc_id = var.vpc.id
  tags = {
    Name = "${var.vpc_name}-igw"
    VPC  = "${var.vpc_name}"
  }
}
