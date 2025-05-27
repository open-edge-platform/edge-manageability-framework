# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

data "aws_route53_zone" "parent_private" {
  name = var.route53_zone_name
  private_zone = true
}

resource "aws_route53_record" "pull_through_cache_proxy" {
  zone_id = data.aws_route53_zone.parent_private.zone_id
  name    = "docker-cache.${var.route53_zone_name}"
  type    = "A"

  alias {
    name                   = aws_lb.pull_through_cache_proxy.dns_name
    evaluate_target_health = true
    zone_id                = aws_lb.pull_through_cache_proxy.zone_id
  }
}
