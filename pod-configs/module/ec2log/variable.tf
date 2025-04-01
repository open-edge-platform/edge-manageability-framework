# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "cluster_name" {
  type = string
}

variable "nodegroup_role" {
  type = string
}

variable "upload_file_list" {
  type = string
  default = "/var/log/messages* /var/log/aws-routed-eni/* /var/log/dmesg /tmp/kubelet.log /tmp/free.log /tmp/df.log /tmp/top.log"
}

variable "script" {
  type    = string
  default = "sudo journalctl -xeu kubelet >/tmp/kubelet.log; free >/tmp/free.log; df -h >/tmp/df.log; top -b -n 3 >/tmp/top.log"
}

variable "s3_expire" {
  description = "Expiration period in days for the uploaded logs"
  type        = number
  default     = 30
}

variable "cloudwatch_expire" {
  description = "Expiration period in days for the CloudWatch log group for the Lambda function"
  type        = number
  default     = 7
}

variable "s3_prefix" {
  type    = string
  default = ""
}