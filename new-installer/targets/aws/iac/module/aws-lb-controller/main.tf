# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  helm_values = <<EOF
clusterName: ${var.cluster_name}
env:
  http_proxy: ${var.http_proxy}
  https_proxy: ${var.https_proxy}
  no_proxy: ${var.no_proxy}
defaultTargetType: ip
ingressClassParams:
  spec:
    scheme: internal
keepTLSSecret: true
  EOF
}

resource "helm_release" "aws-load-balancer-controller" {
  name = "aws-lb"
  chart = "aws-load-balancer-controller"
  repository = "https://aws.github.io/eks-charts"
  version = "1.7.1"
  values = [local.helm_values]
  namespace = "kube-system"
  force_update = true
}
