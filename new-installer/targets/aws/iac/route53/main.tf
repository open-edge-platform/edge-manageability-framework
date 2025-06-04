# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  orch_zone     = "${var.orch_name}.${var.parent_zone}"
  traefik_lb_name  = substr(sha256("${var.orch_name}-traefik"), 0, 32)
  argocd_lb_name   = substr(sha256("${var.orch_name}-argocd"), 0, 32)
  traefik2_lb_name = substr(sha256("${var.orch_name}-traefik2"), 0, 32)
}

data "aws_route53_zone" "parent_public" {
  name         = var.parent_zone
  private_zone = false
}

data "aws_route53_zone" "parent_private" {
  name         = var.parent_zone
  private_zone = true
}

resource "aws_route53_zone" "orch_public" {
  name  = local.orch_zone
}

resource "aws_route53_zone" "orch_private" {
  name  = local.orch_zone

  vpc {
    vpc_id     = var.vpc_id
    vpc_region = var.vpc_region
  }
}

resource "aws_route53_record" "orch_public" {
  zone_id    = data.aws_route53_zone.parent_public.zone_id
  name       = local.orch_zone
  type       = "NS"
  ttl        = 900
  records    = aws_route53_zone.orch_public.name_servers
}

resource "aws_route53_record" "orch_private" {
  zone_id    = data.aws_route53_zone.parent_private.zone_id
  name       = local.orch_zone
  type       = "NS"
  ttl        = 900
  records    = aws_route53_zone.orch_private.name_servers
}

data "aws_lb" "traefik" {
  name  = "${local.traefik_lb_name}"
}

data "aws_lb" "argocd" {
  name  = "${local.argocd_lb_name}"
}

data "aws_lb" "traefik2" {
  name  = "${local.traefik2_lb_name}"
}

resource "aws_route53_record" "traetik_public" {
  zone_id      = aws_route53_zone.orch_public.zone_id
  name         = local.orch_zone
  type         = "A"

  alias {
    name                   = data.aws_lb.traefik.dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.traefik.zone_id
  }
}

resource "aws_route53_record" "traetik_private" {
  zone_id      = aws_route53_zone.orch_private.zone_id
  name         = local.orch_zone
  type         = "A"

  alias {
    name                   = data.aws_lb.traefik.dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.traefik.zone_id
  }
}

resource "aws_route53_record" "argocd_public" {
  zone_id      = aws_route53_zone.orch_public.zone_id
  name         = "argocd.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.argocd.dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.argocd.zone_id
  }
}

resource "aws_route53_record" "argocd_private" {
  zone_id      = aws_route53_zone.orch_private.zone_id
  name         = "argocd.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.argocd.dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.argocd.zone_id
  }
}

resource "aws_route53_record" "gitea_public" {
  zone_id      = aws_route53_zone.orch_public.zone_id
  name         = "gitea.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.argocd.dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.argocd.zone_id
  }
}

resource "aws_route53_record" "gitea_private" {
  zone_id      = aws_route53_zone.orch_private.zone_id
  name         = "gitea.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.argocd.dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.argocd.zone_id
  }
}

resource "aws_route53_record" "traefik2_public" {
  zone_id      = aws_route53_zone.orch_public.zone_id
  name         = "traefik2.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.traefik2.dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.traefik2.zone_id
  }
}

resource "aws_route53_record" "traefik2_private" {
  zone_id      = aws_route53_zone.orch_private.zone_id
  name         = "traefik2.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.traefik2.dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.traefik2.zone_id
  }
}

resource "aws_route53_record" "public_hostname" {
  for_each = toset(var.hostname)
  name     = "${each.value}.${local.orch_zone}"
  zone_id  = aws_route53_zone.orch_public.zone_id
  ttl      = 900
  type     = "CNAME"
  records  = ["${local.orch_zone}"]
}

resource "aws_route53_record" "private_hostname" {
  for_each = toset(var.hostname)
  name     = "${each.value}.${local.orch_zone}"
  zone_id  = aws_route53_zone.orch_private.zone_id
  ttl      = 900
  type     = "CNAME"
  records  = ["${local.orch_zone}"]
}

resource "aws_route53_record" "public_hostname_traefik2" {
  for_each = toset(var.traefik2_hostname)
  name     = "${each.value}.${local.orch_zone}"
  zone_id  = aws_route53_zone.orch_public.zone_id
  ttl      = 900
  type     = "CNAME"
  records  = ["traefik2.${local.orch_zone}"]
}

resource "aws_route53_record" "private_hostname_traefik2" {
  for_each = toset(var.traefik2_hostname)
  name     = "${each.value}.${local.orch_zone}"
  zone_id  = aws_route53_zone.orch_private.zone_id
  ttl      = 900
  type     = "CNAME"
  records  = ["traefik2.${local.orch_zone}"]
}
