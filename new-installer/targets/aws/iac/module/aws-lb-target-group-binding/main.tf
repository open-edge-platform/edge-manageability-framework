# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "kubernetes_manifest" "aws_lb_tgb" {
  for_each = var.bindings
  manifest = {
    apiVersion = "elbv2.k8s.aws/v1beta1"
    kind = "TargetGroupBinding"
    metadata = {
      namespace = each.value.serviceNamespace
      name = each.key
    }
    spec = {
      serviceRef = {
        name = each.value.serviceName
        port = each.value.servicePort
      }
      targetGroupARN = each.value.target_id
    }
  }
}
