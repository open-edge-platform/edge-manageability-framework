# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

terraform {
  backend "s3" {}
  required_providers {
    kubernetes = {
      source = "hashicorp/kubernetes"
      version = "2.33.0"
    }
    tls = {
      source = "hashicorp/tls"
      version = "4.0.6"
    }
    aws = {
      source = "hashicorp/aws"
      version = "5.93.0"
    }
  }
}

provider "aws" {
  region  = var.aws_region
  default_tags {
    tags = {
      environment = var.name
      customer = var.customer_tag
    }
  }
}
