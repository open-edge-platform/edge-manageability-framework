# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "random_integer" "random_prefix" {
  min = 100000
  max = 999999
}

## IAM Policy
resource "aws_iam_policy" "s3_policy" {
  description = "Policy that allows access to S3 buckets in ${var.cluster_name} cluster"
  name        = "${var.cluster_name}-s3-policy"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action   = "s3:*",
        Sid      = "VisualEditor0"
        Effect   = "Allow"
        Resource = "arn:aws:s3:::${var.cluster_name}-*"
      },
    ]
  })
}

## IAM Role
# OIDC issuers from EKS clusters
data "aws_eks_cluster" "eks" {
  name = var.cluster_name
}

locals {
  service_accounts = [
    // namespace:account-name
    "system:serviceaccount:orch-platform:aws-s3-sa-mimir",
    "system:serviceaccount:orch-platform:aws-s3-sa-loki",
    "system:serviceaccount:orch-infra:aws-s3-sa-mimir",
    "system:serviceaccount:orch-infra:aws-s3-sa-loki"
  ]
}

data "aws_iam_policy_document" "s3_policy" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"
    condition {
      test     = "StringEquals"
      variable = "${replace(data.aws_eks_cluster.eks.identity[0].oidc[0].issuer, "https://", "")}:sub"
      // https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_condition-logic-multiple-context-keys-or-values.html
      // If a single condition operator includes multiple values for a context key, those values are evaluated using a logical OR.
      values   = local.service_accounts
    }
    principals {
      identifiers = ["arn:aws:iam::${var.aws_accountid}:oidc-provider/${replace(data.aws_eks_cluster.eks.identity[0].oidc[0].issuer, "https://", "")}"]
      type        = "Federated"
    }
  }
}

resource "aws_iam_role" "s3_role" {
  description         = "Role that can access S3 buckets in ${var.cluster_name} cluster"
  name                = "${var.cluster_name}-s3-role"
  assume_role_policy  = data.aws_iam_policy_document.s3_policy.json
  managed_policy_arns = [aws_iam_policy.s3_policy.arn]
}

# S3
locals {
  buckets = toset([
    "orch-loki-admin",
    "orch-loki-chunks",
    "orch-loki-ruler",
    "orch-mimir-ruler",
    "orch-mimir-tsdb",
    "fm-loki-admin",
    "fm-loki-chunks",
    "fm-loki-ruler",
    "fm-mimir-ruler",
    "fm-mimir-tsdb"
  ])
}

resource "aws_s3_bucket" "bucket" {
  for_each      = local.buckets
  bucket        = var.s3_prefix == "" ? "${var.cluster_name}-${random_integer.random_prefix.result}-${each.key}" : "${var.cluster_name}-${var.s3_prefix}-${each.key}"
  force_destroy = true
}

resource "aws_s3_bucket_lifecycle_configuration" "bucket_config" {
  for_each = local.buckets
  bucket   = aws_s3_bucket.bucket[each.key].id
  # name   = "intelligent-tiering"

  rule {
    id = "intelligent-tiering"

    transition {
      days          = 0
      storage_class = "INTELLIGENT_TIERING"
    }
    status = "Enabled"
  }
}

data "aws_iam_policy_document" "bucket_policy_doc" {
  for_each = var.import_buckets ? [] : local.buckets
  statement {
    sid = "OnlyAllowAccessViaSSL"
    principals {
      type        = "*"
      identifiers = ["*"]
    }

    effect = "Deny"
    actions = [
      "s3:*",
    ]

    resources = [
      aws_s3_bucket.bucket[each.key].arn,
      "${aws_s3_bucket.bucket[each.key].arn}/*",
    ]

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"

      values = [
        "false",
      ]
    }
  }
}

resource "aws_s3_bucket_policy" "bucket_policy" {
  for_each = local.buckets
  bucket   = aws_s3_bucket.bucket[each.key].id
  policy   = data.aws_iam_policy_document.bucket_policy_doc[each.key].json
}

# Create tracing bucket
resource "aws_s3_bucket" "tracing" {
  count         = var.create_tracing ? 1 : 0
  bucket        = var.s3_prefix == "" ? "${var.cluster_name}-${random_integer.random_prefix.result}-tempo-traces" : "${var.cluster_name}-${var.s3_prefix}-tempo-traces"
  force_destroy = true
}

resource "aws_s3_bucket_lifecycle_configuration" "tracing_config" {
  count  = var.create_tracing ? 1 : 0
  bucket = aws_s3_bucket.tracing[0].id
  # name = "intelligent-tiering"

  rule {
    id = "intelligent-tiering"

    transition {
      days          = 0
      storage_class = "INTELLIGENT_TIERING"
    }
    status = "Enabled"
  }
}

data "aws_iam_policy_document" "tracing_policy_doc" {
  count = var.create_tracing ? 1 : 0
  statement {
    sid = "OnlyAllowAccessViaSSL"
    principals {
      type        = "*"
      identifiers = ["*"]
    }

    effect = "Deny"
    actions = [
      "s3:*",
    ]

    resources = [
      aws_s3_bucket.tracing[0].arn,
      "${aws_s3_bucket.tracing[0].arn}/*",
    ]

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"

      values = [
        "false",
      ]
    }
  }
}

resource "aws_s3_bucket_policy" "tracing_policy" {
  count  = var.create_tracing ? 1 : 0
  bucket = aws_s3_bucket.tracing[0].id
  policy = data.aws_iam_policy_document.tracing_policy_doc[0].json
}
