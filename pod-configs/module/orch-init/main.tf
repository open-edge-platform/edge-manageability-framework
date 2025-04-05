# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "kubernetes_namespace" "ns" {
  for_each = toset(var.needed_namespaces)

  metadata {
    name = each.value
  }

  lifecycle {
    ignore_changes = [
      metadata,
    ]
  }
}

resource "kubernetes_namespace" "istio_namespaces" {
  for_each = toset(var.istio_namespaces)

  metadata {
    name = each.value
    labels = {
      "istio-injection" = "enabled"
    }
  }

  lifecycle {
    ignore_changes = [
      metadata,
    ]
  }
}

resource "time_sleep" "wait_ns" {
  create_duration = "10s"
  depends_on = [
    kubernetes_namespace.istio_namespaces,
    kubernetes_namespace.ns,
  ]
}

locals {
  # Create a filtered list of namespaces that excludes 'orch-gateway' if var.auto_cert is true
  namespaces_for_tls = [for ns in concat(var.needed_namespaces, var.istio_namespaces): ns if ns != "orch-gateway" || !var.auto_cert]
}

resource "kubernetes_secret" "tls-ingress" {
  depends_on = [time_sleep.wait_ns]
  for_each   = toset(local.namespaces_for_tls)

  metadata {
    name      = each.key == "argocd" ? "argocd-server-tls" : "tls-orch"
    namespace = each.key
  }
  data = {
    "tls.crt" = var.tls_cert
    "tls.key" = var.tls_key
  }
  type = "kubernetes.io/tls"
  lifecycle {
    ignore_changes = [
      metadata,
    ]
  }
}

# creates the tls-autocert secret in orch-gateway using the lets encrypt certs
# this is for an auto-cert deployment and exists because cert-synchronizer handles
# the creation of tls-orch in this scenario.
resource "kubernetes_secret" "tls-autocert" {
  count      = var.auto_cert ? 1 : 0
  depends_on = [time_sleep.wait_ns]
  metadata {
    name      = "tls-autocert"
    namespace = "orch-gateway"
    annotations = {
      "cert-manager.io/ip-sans": ""
      "cert-manager.io/issuer-group": ""
      "cert-manager.io/uri-sans": ""
      "cert-manager.io/issuer-kind": "ClusterIssuer"
      "cert-manager.io/issuer-name": "orchestrator-autocert-issuer"
    }
  }
  data = {
    "tls.crt" = var.tls_cert
    "tls.key" = var.tls_key
  }
  type = "kubernetes.io/tls"
  lifecycle {
    ignore_changes = [
      metadata,
    ]
  }
}

resource "kubernetes_secret" "tls-ca" {
  depends_on = [time_sleep.wait_ns]

  metadata {
    name      = "tls-ca"
    namespace = "cattle-system"
  }
  data = {
    "cacerts.pem" = var.ca_cert
  }
  lifecycle {
    ignore_changes = [
      metadata,
    ]
  }
}

resource "kubernetes_secret" "webhook_github_secret" {
  depends_on = [time_sleep.wait_ns]

  metadata {
    name      = "webhook-github-secret"
    namespace = "orch-app"
  }
  data = {
    ".netrc" = var.webhook_github_netrc
  }
}

resource "kubernetes_secret" "sre_basic_auth_username" {
  depends_on = [time_sleep.wait_ns]

  metadata {
    name      = "basic-auth-username"
    namespace = "orch-sre"
  }
  data = {
    "username" = var.sre_basic_auth_username
  }
}

resource "kubernetes_secret" "sre_basic_auth_password" {
  depends_on = [time_sleep.wait_ns]

  metadata {
    name      = "basic-auth-password"
    namespace = "orch-sre"
  }
  data = {
    "password" = var.sre_basic_auth_password
  }
}

resource "kubernetes_secret" "sre_destination_secret_url" {
  depends_on = [time_sleep.wait_ns]

  metadata {
    name      = "destination-secret-url"
    namespace = "orch-sre"
  }
  data = {
    "password" = var.sre_destination_secret_url
  }
}

resource "kubernetes_secret" "sre_destination_ca_secret" {
  depends_on = [time_sleep.wait_ns]

  metadata {
    name      = "destination-secret-ca"
    namespace = "orch-sre"
  }
  data = {
    "password" = var.sre_destination_ca_secret
  }
}

resource "kubernetes_secret" "smtp" {
  count = var.smtp_user != "" && var.smtp_url != "" ? 1 : 0

  depends_on = [time_sleep.wait_ns]

  metadata {
    name      = "smtp"
    namespace = "orch-infra"
  }

  type = "Opaque"

  data = {
    "smartHost"     = var.smtp_url
    "smartPort"     = var.smtp_port
    "from"          = var.smtp_from
    "authUsername"  = var.smtp_user
  }
}

resource "kubernetes_secret" "smtp_auth" {
  count = var.smtp_pass != "" ? 1 : 0

  depends_on = [time_sleep.wait_ns]

  metadata {
    name      = "smtp-auth"
    namespace = "orch-infra"
  }

  type = "kubernetes.io/basic-auth"

  data = {
    "password" = var.smtp_pass
  }
}

resource "kubernetes_secret" "release_service_refresh_token" {
  depends_on = [time_sleep.wait_ns]
  metadata {
    name      = "azure-ad-creds"
    namespace = "orch-secret"
  }
  type = "Opaque"
  data = {
    "refresh_token" = "${var.release_service_refresh_token}"
  }
}

resource "kubernetes_storage_class" "efs_999" {
  metadata {
    name = "efs-999"
  }
  storage_provisioner = "efs.csi.aws.com"

  reclaim_policy = "Delete"
  volume_binding_mode = "Immediate"
  mount_options = [
    "tls"
  ]
  parameters = {
    "basePath": "/dynamic_provisioning",
    "directoryPerms": "700",
    "fileSystemId": var.efs_id,
    "gid": "999",
    "provisioningMode": "efs-ap",
    "uid": "999"
  }
}

resource "kubernetes_storage_class" "efs_1000" {
  metadata {
    annotations = {
      "storageclass.kubernetes.io/is-default-class": "true"
    }
    name = "efs-1000"
  }
  storage_provisioner = "efs.csi.aws.com"

  reclaim_policy = "Delete"
  volume_binding_mode = "Immediate"
  mount_options = [
    "tls"
  ]
  parameters = {
    "basePath": "/dynamic_provisioning",
    "directoryPerms": "700",
    "fileSystemId": var.efs_id,
    "gid": "1000",
    "provisioningMode": "efs-ap",
    "uid": "1000"
  }
}

resource "kubernetes_storage_class" "gp3" {
  metadata {
    name = "gp3"
  }
  storage_provisioner = "ebs.csi.aws.com"
  reclaim_policy = "Delete"
  volume_binding_mode = "WaitForFirstConsumer"
  parameters = {
    "encrypted": "true",
    "csi.storage.k8s.io/fstype": "ext4",
    "type": "gp3",
    "tagSpecification_1": "environment=${var.cluster_name}"
  }
}
