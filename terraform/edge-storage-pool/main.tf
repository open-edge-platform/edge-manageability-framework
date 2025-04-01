# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "libvirt_pool" "edge" {
  name = var.name
  type = "dir"
  target {
    path = abspath(var.target_path)
  }
}
