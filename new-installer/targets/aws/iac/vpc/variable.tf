# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "region" {
  description = "Region for resources"
}

variable "vpc_name" {
  description = "The VPC name"
}

variable "vpc_cidr_block" {
  description = "Default VPC CIDR blcok"
}

variable "vpc_additional_cidr_blocks" {
  type        = set(string)
  description = "Additional IPv4 CIDR blocks for this VPC"
}
variable "vpc_enable_dns_hostnames" {
  type    = bool
  default = true
}
variable "vpc_enable_dns_support" {
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
  default     = "SGForAllVPCEndpoints"
}

variable "jumphost_ip_allow_list" {
  type        = list(string)
  description = "List of IPs which used for security group rules. Default is empty which will not allow any traffic."
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

variable "jumphost_subnet" {
  type = object({
    name       = string
    az         = string
    cidr_block = string
  })
  description = "The subnet which jumphost will use"
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
