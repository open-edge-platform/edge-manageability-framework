# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_iam_policy" "aws_load_balancer" {
  count  = var.feature_flags.iam_roles ? 1 : 0
  name   = "aws_load_balancer_controller"
  policy = file("${path.module}/lb_policy.json")
}
