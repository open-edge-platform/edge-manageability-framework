# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_wafv2_web_acl" "traefik" {
  name  = "${var.cluster_name}-traefik"
  scope = "REGIONAL"

  default_action {
    allow {}
  }

  dynamic "rule" {
    for_each = var.waf_rule_groups
    content {
      name     = "${rule.value.vendor_name}-${rule.value.name}"
      priority = rule.value.priority
      statement {
        managed_rule_group_statement {
          name        = rule.value.name
          vendor_name = rule.value.vendor_name
        }
      }
      override_action {
        none {}
      }
      visibility_config {
        cloudwatch_metrics_enabled = true
        metric_name                = "web-acl-${rule.value.vendor_name}-${rule.value.name}"
        sampled_requests_enabled   = false
      }
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "web-acl-${var.cluster_name}-traefik"
    sampled_requests_enabled   = false
  }
}

resource "aws_wafv2_web_acl_association" "traefik" {
  resource_arn = var.traefik_load_balancer_arn
  web_acl_arn  = aws_wafv2_web_acl.traefik.arn
}
