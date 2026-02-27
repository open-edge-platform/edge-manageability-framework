# creating service account for cas controller

resource "kubernetes_service_account" "cluster_autoscaler" {
  metadata {
    name      = var.cas_service_account
    namespace = var.cas_namespace

    annotations = {
      "eks.amazonaws.com/role-arn" = var.cas_controller_arn
    }
  }
}


resource "helm_release" "cluster_autoscaler" {
  name       = "cluster-autoscaler"
  repository = "https://kubernetes.github.io/autoscaler"
  chart      = "cluster-autoscaler"
  namespace  = var.cas_namespace
  version    = "9.46.6"

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
values = [
  yamlencode({
    extraEnv = {
      HTTP_PROXY  = "http://proxy-dmz.intel.com:912"
      HTTPS_PROXY = "http://proxy-dmz.intel.com:912"
      NO_PROXY    = "169.254.169.254,127.0.0.1,localhost,.cluster.local,kubernetes.default,kubernetes.default.svc,kubernetes.default.svc.cluster.local,10.0.0.0/8,172.20.0.0/16,172.20.0.1,192.168.0.0/16"
    }
  })
]
}
