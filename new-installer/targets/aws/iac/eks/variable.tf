# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "name" {
  description = "The name of the cluster"
  type        = string
}

variable "region" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "customer_tag" {
  type    = string
  default = ""
}

variable "subnet_ids" {
  type        = set(string)
  description = "Subnet IDs for EKS cluster"
}

variable "eks_version" {
  type        = string
  default     = "1.32"
  description = "EKS version to use for the cluster"
}

variable "volume_size" {
  type        = number
  description = "The size of the EKS volume in GiB"
}

variable "volume_type" {
  type        = string
  default     = "gp3"
  description = "The type of the EKS volume (e.g., gp2, gp3)"
}

variable "node_instance_type" {
  type        = string
  description = "The instance type for EKS nodes"
  default     = "t3.2xlarge"
}

variable "desired_size" {
  type        = number
  description = "The desired number of EKS nodes"
  default     = 1
}

variable "min_size" {
  type        = number
  description = "The minimum number of EKS nodes"
  default     = 1
}

variable "max_size" {
  type        = number
  description = "The maximum number of EKS nodes"
  default     = 1
}

variable "addons" {
  type = list(object({
    name                 = string
    version              = string
    configuration_values = optional(string, "")
  }))
  default = [
    {
      name    = "aws-ebs-csi-driver",
      version = "v1.39.0-eksbuild.1"
    },
    {
      name                 = "vpc-cni",
      version              = "v1.19.2-eksbuild.1"
      configuration_values = "{\"enableNetworkPolicy\": \"true\", \"nodeAgent\": {\"healthProbeBindAddr\": \"8163\", \"metricsBindAddr\": \"8162\"}}"
    },
    {
      name    = "aws-efs-csi-driver"
      version = "v2.1.4-eksbuild.1"
    }
  ]
}

variable "max_pods" {
  default = 58
}

variable "additional_node_groups" {
  type = map(object({
    desired_size : number
    min_size : number
    max_size : number
    taints : map(object({
      value : string
      effect : string
    }))
    labels : map(string)
    instance_type : string
    volume_size : number
    volume_type : string
  }))
  default = {
    "observability" : {
      desired_size = 1
      min_size     = 1
      max_size     = 1
      labels = {
        "node.kubernetes.io/custom-rule" : "observability"
      }
      taints = {
        "node.kubernetes.io/custom-rule" : {
          value  = "observability"
          effect = "NO_SCHEDULE"
        }
      }
      instance_type = "t3.2xlarge"
      volume_size   = 20
      volume_type   = "gp3"
    }
  }
}

variable "enable_cache_registry" {
  type    = string
  default = "false"
}
variable "cache_registry" {
  type    = string
  default = ""
}

variable "user_script_pre_cloud_init" {
  type        = string
  default     = ""
  description = "Script to run before cloud-init"
}

variable "user_script_post_cloud_init" {
  type        = string
  default     = ""
  description = "Script to run after cloud-init"
}
variable "http_proxy" {
  type        = string
  default     = ""
  description = "HTTP proxy to use for EKS nodes"
}

variable "https_proxy" {
  type        = string
  default     = ""
  description = "HTTPS proxy to use for EKS nodes"
}

variable "no_proxy" {
  type        = string
  default     = ""
  description = "No proxy to use for EKS nodes"
}
