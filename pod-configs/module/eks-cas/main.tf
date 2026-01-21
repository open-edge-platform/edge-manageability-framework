# creating service account for cas controller

resource "kubernetes_service_account" "cluster_autoscaler" {
  metadata {
    name      = var.cas_service_account
    namespace = var.cas_namespace

    annotations = {
      "eks.amazonaws.com/role-arn" = var.cas_controller.arn
    }
  }
}


resource "helm_release" "cluster_autoscaler" {
  name       = "cluster-autoscaler"
  repository = "https://kubernetes.github.io/autoscaler"
  chart      = "cluster-autoscaler"
  namespace  = var.cas_namespace

  depends_on = [
    kubernetes_service_account.cluster_autoscaler
  ]

  set = [
  {
    name  = "priorityClassName"
    value = "system-cluster-critical"
  },
  {
    name  = "autoDiscovery.clusterName"
    value = var.cluster_name
  },
  {
    name  = "awsRegion"
    value = var.aws_region
  },
  {
    name  = "rbac.serviceAccount.create"
    value = "false"
  },
  {
    name  = "rbac.serviceAccount.name"
    value = var.cas_service_account
  },
  {
    name  = "extraArgs.balance-similar-node-groups"
    value = "true"
  },
  {
    name  = "extraArgs.expander"
    value = "least-waste"
  }
]

}
