# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

module "s3" {
  depends_on     = [time_sleep.wait_eks]
  source         = "../module/s3"
  s3_prefix      = var.s3_prefix
  cluster_name   = var.cluster_name
  create_tracing = var.s3_create_tracing
  import_buckets = var.import_s3_buckets
}
