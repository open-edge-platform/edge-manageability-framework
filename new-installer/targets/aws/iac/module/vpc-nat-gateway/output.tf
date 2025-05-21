# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "nat_gateways_with_eip" {
  value = aws_nat_gateway.ngw_with_eip
}
output "nat_gateways_without_eip" {
  value = aws_nat_gateway.ngw_without_eip
}