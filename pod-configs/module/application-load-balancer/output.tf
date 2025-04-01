# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "lb_dns_name" {
  value = aws_lb.main.dns_name
}
output "lb_zone_id" {
  value = aws_lb.main.zone_id
}
output "target_groups" {
  value = aws_lb_target_group.main
}
output "lb_arn" {
  value = aws_lb.main.arn
}
output "lb_sg_id" {
  value = aws_security_group.common.id
}
