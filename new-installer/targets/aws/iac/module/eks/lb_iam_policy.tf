# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_iam_policy" "aws_load_balancer" {
  name   = "${var.cluster_name}-aws-load-balancer"
  policy = file("${path.module}/lb_policy.json")
}
