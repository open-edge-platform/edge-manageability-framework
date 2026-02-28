variable "cluster_name" {
  type = string
}

variable "cas_namespace" {
  description = "The Kubernetes namespace for cluster autoscaler"
  default     = "kube-system"
}

variable cas_version {
  type = string
  default = "9.46.6"
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

variable "CAS_HP" {
  type    = string
  default = ""
}

variable "CAS_HPS" {
  type    = string
  default = ""
}

variable "CAS_NP" {
  type    = string
  default = ""
}
