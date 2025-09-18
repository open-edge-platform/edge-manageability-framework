# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  gitea_users  = toset(["argocd", "apporch", "clusterorch"])
  argocd_repos = toset(["edge-manageability-framework"])
}

data "aws_caller_identity" "current" {}

resource "random_password" "gitea_master_password" {
  length           = 16
  special          = false
}

resource "kubernetes_namespace" "gitea" {
  metadata {
    name = "gitea"
  }
  lifecycle {
    ignore_changes = [metadata]
  }
}

resource "kubernetes_secret" "gitea_tls" {
  depends_on = [kubernetes_namespace.gitea]
  metadata {
    name      = "gitea-tls-certs"
    namespace = "gitea"
  }
  data = {
    "tls.crt" = var.tls_cert_body
    "tls.key" = var.tls_key
  }
}

resource "helm_release" "gitea" {
  depends_on = [kubernetes_namespace.gitea]
  name       = var.name
  chart      = "gitea"
  repository = "https://dl.gitea.com/charts/"
  version    = var.gitea_chart_version
  values = [templatefile("${path.module}/gitea-values.yaml.tpl", {
    gitea_password : random_password.gitea_master_password.result
    gitea_domain   : var.gitea_fqdn
  })]
  namespace    = "gitea"
  force_update = false
  replace      = true
  timeout      = 600
}

resource "random_password" "gitea_user_password" {
  for_each         = local.gitea_users
  length           = 16
  special          = false
}

resource "null_resource" "create_kubecnofig" {
  triggers = {
    always = timestamp()
  }
  provisioner "local-exec" {
    command = <<EOT
    aws eks update-kubeconfig --name "${var.cluster_name}" --region "${var.aws_region}" --kubeconfig "/tmp/kubeconfig-${var.cluster_name}"
    EOT
  }
}

resource "null_resource" "initialize_gitea_users" {
  for_each   = local.gitea_users
  depends_on = [helm_release.gitea]
  triggers = {
    always = timestamp() # Always try to re-apply the configuration
  }
  provisioner "local-exec" {
    command = <<EOF
#!/bin/bash
export GITEA_USER='${each.key}'
export GITEA_PASSWORD='${random_password.gitea_user_password[each.key].result}'
export GITEA_ADMIN_USER='gitea_admin'
export GITEA_ADMIN_PASSWORD='${random_password.gitea_master_password.result}'
export KUBECONFIG='/tmp/kubeconfig-${var.cluster_name}'
kubectl config set-context 'arn:aws:eks:${var.aws_region}:${data.aws_caller_identity.current.account_id}:cluster/${var.cluster_name}'
send_command_to_gitea() {
  kubectl exec -n gitea -it -c gitea "$(kubectl get pods -n gitea -l app.kubernetes.io/name=gitea -o jsonpath='{.items[0].metadata.name}')" -- "$@"
}
USER_EXISTS=$(send_command_to_gitea gitea admin user list | grep -c "$GITEA_USER")
if [ "$USER_EXISTS" -eq 0 ]; then
  echo "Creating user $GITEA_USER"
  send_command_to_gitea gitea admin user create --username "$GITEA_USER" --password "$GITEA_PASSWORD" --email "$GITEA_USER@orch.internal"
fi
# Ensure password is update-to-date when updating the password
send_command_to_gitea gitea admin user change-password --username "$GITEA_USER" --password "$GITEA_PASSWORD" --must-change-password=false
EOF
  }
}

resource "kubernetes_secret" "apporch_gitea_secrets" {
  metadata {
    name      = "app-gitea-credential"
    namespace = "orch-platform"

  }
  data = {
    "username" = "apporch"
    "password" = random_password.gitea_user_password["apporch"].result
  }
}

resource "kubernetes_secret" "clusterorch_gitea_secrets" {
  metadata {
    name      = "cluster-gitea-credential"
    namespace = "orch-platform"

  }
  data = {
    "username" = "clusterorch"
    "password" = random_password.gitea_user_password["clusterorch"].result
  }
}

resource "kubernetes_secret" "argocd_gitea_secrets" {
  for_each = local.argocd_repos
  metadata {
    name      = "gitea-credential-${each.key}"
    namespace = "argocd"
    labels = {
      "argocd.argoproj.io/secret-type" : "repository"
    }
  }
  data = {
    "username" = "argocd"
    "password" = random_password.gitea_user_password["argocd"].result
    "url"      = "https://${var.gitea_fqdn}/argocd/${each.key}"
    "type"     = "git"
  }
}

# Need to create this config map to allow ArgoCD to access the Gitea server with specific certificate.
# This ConfigMap contains required label and annotations to be managed by Helm.
resource "kubernetes_config_map" "argocd_tls_certs_cm" {
  metadata {
    name      = "argocd-tls-certs-cm"
    namespace = "argocd"
    labels = {
      "app.kubernetes.io/managed-by": "Helm"
      "meta.helm.sh/release-name": "argocd"
    }
    annotations = {
      "helm.sh/resource-policy": "keep" # Do not delete this ConfigMap when deleting the Helm release since it is managed by Terraform.
      "meta.helm.sh/release-name": "argocd"
      "meta.helm.sh/release-namespace": "argocd"
    }
  }
  data = {
    "${var.gitea_fqdn}" : var.tls_cert_body
  }
}
