variable "cluster_name" {
  type = string
}

variable "cas_namespace" {
  description = "The Kubernetes namespace for cluster autoscaler"
  default     = "kube-system"
}

variable "cas_service_account" {
  description = "The Kubernetes service account name of cluster autoscaler"
  default     = "cluster-autoscaler"
}

variable "cas_controller_arn" {
  type = string
}

variable "aws_region" {
  type = string
}
