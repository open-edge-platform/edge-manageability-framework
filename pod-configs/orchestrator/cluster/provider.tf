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
      version = "4.1.0"
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
      environment = var.eks_cluster_name
      customer = var.customer_tag
    }
  }
}

provider "kubernetes" {
  host                   = data.aws_eks_cluster.eks_cluster_data.endpoint
  cluster_ca_certificate = base64decode(data.aws_eks_cluster.eks_cluster_data.certificate_authority[0].data)
  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    args        = ["eks", "get-token", "--cluster-name", var.eks_cluster_name]
    command     = "aws"
  }
}

provider "helm" {
  kubernetes {
    host                   = data.aws_eks_cluster.eks_cluster_data.endpoint
    cluster_ca_certificate = base64decode(data.aws_eks_cluster.eks_cluster_data.certificate_authority[0].data)
    exec {
      api_version = "client.authentication.k8s.io/v1beta1"
      args        = ["eks", "get-token", "--cluster-name", var.eks_cluster_name]
      command     = "aws"
    }
  }
}