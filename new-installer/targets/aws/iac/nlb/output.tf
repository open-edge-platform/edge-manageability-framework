# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "nlb_dns_name" {
  value = aws_lb.main.dns_name
}

output "nlb_target_group_arn" {
  value = aws_lb_target_group.main.arn
}

output "nlb_arn" {
  value = aws_lb.main.arn
}
