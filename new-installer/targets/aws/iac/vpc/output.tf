# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "vpc_id" {
  value = aws_vpc.id
}
output "public_subnets" {
  value = aws_subnet.public_subnets
}
output "private_subnets" {
  value = aws_subnet.private_subnets
}
