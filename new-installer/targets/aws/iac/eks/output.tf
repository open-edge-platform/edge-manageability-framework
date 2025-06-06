# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "eks_oidc_issuer" {
  description = "The OIDC issuer URL for the EKS cluster."
  value = aws_eks_cluster.eks.identity[0].oidc[0].issuer
}
