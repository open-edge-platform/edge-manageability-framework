# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "efs_id" {
  value = aws_efs_file_system.efs.id
  description = "The ID of the EFS file system."
}
