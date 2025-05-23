# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "region" {
  description = "The AWS region to deploy to"
}
variable "bucket" {
  description = "The name of the S3 bucket"
}
variable "orch_name" {
  description = "The name of the orchestration environment"
}