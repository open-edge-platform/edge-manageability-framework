# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "jumphost_ip" {
  value = aws_instance.jumphost.private_ip
}
