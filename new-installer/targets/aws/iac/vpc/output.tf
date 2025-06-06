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

output "jumphost_ip" {
  value = aws_instance.jumphost.public_ip
}
