# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
module "kms" {
  source             = "../module/kms"
  cluster_name       = var.cluster_name
}
