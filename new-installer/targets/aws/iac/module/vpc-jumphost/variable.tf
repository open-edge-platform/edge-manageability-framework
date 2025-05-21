# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "vpc_name" {}
variable "vpc_id" {}
variable "region" {}
variable "ip_allow_list" {
  type        = set(string)
  description = "Set of IPs which used for security group rules."
  default = []
}
variable "egress_ip_allow_list" {
  type = set(string)
  default = []
}

variable "jumphost_ami_id" {
  type        = string
  description = "AWS AMI ID used for jump host instance."
  default     = "ami-0af9d24bd5539d7af"
}

variable "jumphost_instance_type" {
  type        = string
  description = "EC2 instance type used for jump host."
  default     = "t3.medium"
}

variable "jumphost_instance_ssh_key_pub" {
  type        = string
  description = "SSH public key to configure jumphost EC2 instance."
  default     = ""
}

variable "subnet" {
  description = "Subnet for this jumphost"
  type = object({
    name       = string
    az         = string
    cidr_block = string
  })
}

variable "production" {
  type = bool
  description = "Whether it is a production environment, this will disable the metadata service and login shell"
  default = true
}
