# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

terraform {
  backend "s3" {}
}
provider "aws" {
  default_tags {
    tags = {
      environment = var.orch_name
      customer = var.customer_tag
    }
  }
}