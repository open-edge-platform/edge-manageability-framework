# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "vpc_id" {
  value = aws_vpc.main.id
}
output "public_subnets" {
  value = aws_subnet.public_subnet
}
output "private_subnets" {
  value = aws_subnet.private_subnet
}

output "public_subnet_ids" {
  value = [
    for subnet in aws_subnet.public_subnet : subnet.id
  ]
}

output "private_subnet_ids" {
  value = [
    for subnet in aws_subnet.private_subnet : subnet.id
  ]
}

output "jumphost_ip" {
  value = aws_eip.jumphost.public_ip
}
