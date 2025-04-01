# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "name" {
  value = libvirt_pool.edge.name
}

output "target_path" {
  value = libvirt_pool.edge.target.0.path
}
