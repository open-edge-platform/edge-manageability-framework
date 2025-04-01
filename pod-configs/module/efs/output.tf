# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "role-arn" {
  value = var.generate_eks_policy ? aws_iam_role.efs_role[0].arn : ""
}

output "efs" {
  value = aws_efs_file_system.efs
}

output "access_point_id" {
  value = {for key, val in aws_efs_access_point.access_point : key => val.id}
}
