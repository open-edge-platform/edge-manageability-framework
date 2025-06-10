# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "region" {
  description = "Region for resources"
}

variable "name" {
  description = "The VPC name"
}

variable "cidr_block" {
  description = "Default VPC CIDR blcok"
}

variable "enable_dns_hostnames" {
  type    = bool
  default = true
}
variable "enable_dns_support" {
  type    = bool
  default = true
}

variable "private_subnets" {
  type = map(object({
    az         = string
    cidr_block = string
  }))
  description = "Subnet for internal communication such as EKS and VPCE"
}

variable "public_subnets" {
  type = map(object({
    az         = string
    cidr_block = string
  }))
  description = "Subnet for external connection to edge nodes or customers"
}

variable "endpoint_sg_name" {
  description = "Endpoint security group name"
}

variable "jumphost_ip_allow_list" {
  type        = set(string)
  description = "List of IPs which used for security group rules. Default is empty which will not allow any traffic."
  default = []
}

variable "jumphost_instance_ssh_key_pub" {
  type        = string
  description = "SSH public key to configure jumphost EC2 instance."
}

variable "jumphost_subnet" {
  type = string
  description = "The subnet name which jumphost will use"
}

variable "production" {
  type = bool
  description = "Whether it is a production environment"
  default = true
}

variable "customer_tag" {
  type = string
  description = "For customers to specify a tag for AWS resources"
  default = ""
}

variable "endpoints" {
  type = set(string)
  description = "List of AWS service endpoints to create in the VPC."
  default = [
    "elasticfilesystem",
    "s3",
    "eks",
    "sts",
    "ec2",
    "ec2messages",
    "ecr.dkr",
    "ecr.api",
    "elasticloadbalancing",
    "ecs"
  ]
}
