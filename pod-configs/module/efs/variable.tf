# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "policy_name" {
  type    = string
  default = "EFS_CSI_Driver_Policy"
}

variable "role_name" {
  type    = string
  default = "EFS_CSI_DriverRole"
}

variable "sg_name" {
  type        = string
  default     = "efs-nfs"
  description = "The postfix which will attach to securoty group of EFS"
}

variable "subnets" {
  type        = set(string)
  description = "Subnets for EFS to attach"
}

variable "policy_source" {
  type    = string
  default = "https://raw.githubusercontent.com/kubernetes-sigs/aws-efs-csi-driver/v1.5.4/docs/iam-policy-example.json"
}

variable "aws_accountid" {
  type = string
}

variable "transition_to_ia" {
  type    = string
  default = "AFTER_7_DAYS"
}

variable "transition_to_primary_storage_class" {
  type    = string
  default = "AFTER_1_ACCESS"
}

variable "efs_sg_cidr_blocks" {
  type        = list(string)
  description = "CIDR blocks to allow access the EFS/NFS"
}

variable "cluster_name" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "generate_eks_policy" {
  type = bool
  default = true
}
variable "encrypted" {
  default = false
}

variable "access_points" {
  description = "Access points to create"
  type = map(object({
    root_dir = optional(string, "/")
    uid = optional(number, 1000)
    gid = optional(number, 1000)
  }))
  default = {}
}

variable "throughput_mode" {
  type    = string
  default = "bursting"
}

variable "permissions_boundary" {
  description = "ARN of the IAM permissions boundary policy"
  type        = string
  default     = ""
}
