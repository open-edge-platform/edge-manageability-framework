# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  kube_config_path = "/tmp/kubeconfig-${var.cluster_name}"

}

data "template_file" "auth_map" {
  template = templatefile("${path.module}/auth_map.yaml.tftpl", { 
    aws_roles             = var.aws_roles
    aws_account_number    = var.aws_account_number
    cluster_name          = var.cluster_name
    vpc                   = var.vpc
  })
}

resource "local_file" "aws_auth" {
  content  = data.template_file.auth_map.rendered
  filename = "${path.module}/aws_auth.yaml"
}

resource "null_resource" "set_cluster_access" {
  depends_on = [local_file.aws_auth]
  provisioner "local-exec" {
    command = <<EOT
        set -eu
        # The scripts below will use the configuration file specified in the variable so that it will not affect the default configuration.
        aws eks update-kubeconfig --name "${var.cluster_name}" --region "${var.aws_region}" --kubeconfig "${local.kube_config_path}"

        kubectl apply --kubeconfig "${local.kube_config_path}" --context "arn:aws:eks:${var.aws_region}:${var.aws_account_number}:cluster/${var.cluster_name}" -f "${path.module}/aws_auth.yaml"
EOT
  }
}