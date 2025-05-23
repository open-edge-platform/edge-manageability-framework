# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "eks_auth_map" {
  value = data.template_file.auth_map.rendered
}