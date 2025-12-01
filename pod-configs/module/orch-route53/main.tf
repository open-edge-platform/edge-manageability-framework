# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  orch_zone     = var.host_name=="" ? "${var.orch_name}.${var.parent_zone}" : "${var.host_name}.${var.parent_zone}"
  traefik_lb_name  = substr(sha256("${var.orch_name}-traefik"), 0, 32)
  argocd_lb_name   = substr(sha256("${var.orch_name}-argocd"), 0, 32)
  traefik2_lb_name = substr(sha256("${var.orch_name}-traefik2"), 0, 32)
  traefik3_lb_name = substr(sha256("${var.orch_name}-traefik3"), 0, 32)
}

data "aws_route53_zone" "parent_public" {
  count        = var.create_root_domain ? 1 : 0
  name         = var.parent_zone
  private_zone = false
}

data "aws_route53_zone" "parent_private" {
  count        = var.create_root_domain ? 1 : 0
  name         = var.parent_zone
  private_zone = true
}

# Create public Orchestrator zone
resource "aws_route53_zone" "orch_public" {
  count = var.create_root_domain ? 1 : 0
  name  = local.orch_zone
}

# Create private Orchestrator zone
resource "aws_route53_zone" "orch_private" {
  count = var.create_root_domain ? 1 : 0
  name  = local.orch_zone

  vpc {
    vpc_id     = var.vpc_id
    vpc_region = var.vpc_region
  }
}

data "aws_route53_zone" "orch_public" {
  count        = var.create_root_domain ? 0 : 1
  name         = local.orch_zone
  private_zone = false
}

data "aws_route53_zone" "orch_private" {
  count        = var.create_root_domain ? 0 : 1
  name         = local.orch_zone
  private_zone = true
}

resource "aws_route53_record" "orch_public" {
  count      = var.create_root_domain ? 1 : 0
  depends_on = [aws_route53_zone.orch_public]
  zone_id    = var.create_root_domain ? data.aws_route53_zone.parent_public[0].zone_id : -1
  name       = local.orch_zone
  type       = "NS"
  ttl        = 900
  records    = aws_route53_zone.orch_public[0].name_servers
}

resource "aws_route53_record" "orch_private" {
  count      = var.create_root_domain ? 1 : 0
  depends_on = [aws_route53_zone.orch_private]
  zone_id    = var.create_root_domain ? data.aws_route53_zone.parent_private[0].zone_id : -1
  name       = local.orch_zone
  type       = "NS"
  ttl        = 900
  records    = aws_route53_zone.orch_private[0].name_servers
}

data "aws_lb" "traefik" {
  count = var.lb_created ? 1 : 0
  name  = "${local.traefik_lb_name}"
}

data "aws_lb" "argocd" {
  count = var.lb_created ? 1 : 0
  name  = "${local.argocd_lb_name}"
}

data "aws_lb" "traefik2" {
  count = var.lb_created ? 1 : 0
  name  = "${local.traefik2_lb_name}"
}

data "aws_lb" "traefik3" {
  count = var.lb_created ? 1 : 0
  name  = "${local.traefik3_lb_name}"
}

resource "aws_route53_record" "traetik_public" {
  depends_on   = [aws_route53_zone.orch_public]
  count        = var.lb_created ? 1 : 0
  zone_id      = var.create_root_domain ? aws_route53_zone.orch_public[0].zone_id : data.aws_route53_zone.orch_public[0].zone_id
  name         = local.orch_zone
  type         = "A"

  alias {
    name                   = data.aws_lb.traefik[count.index].dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.traefik[count.index].zone_id
  }
}

resource "aws_route53_record" "traetik_private" {
  depends_on   = [aws_route53_zone.orch_private]
  count        = var.lb_created ? 1 : 0
  zone_id      = var.create_root_domain ? aws_route53_zone.orch_private[0].zone_id : data.aws_route53_zone.orch_private[0].zone_id
  name         = local.orch_zone
  type         = "A"

  alias {
    name                   = data.aws_lb.traefik[count.index].dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.traefik[count.index].zone_id
  }
}

resource "aws_route53_record" "argocd_public" {
  depends_on   = [aws_route53_zone.orch_public]
  count        = var.lb_created ? 1 : 0
  zone_id      = var.create_root_domain ? aws_route53_zone.orch_public[0].zone_id : data.aws_route53_zone.orch_public[0].zone_id
  name         = "argocd.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.argocd[count.index].dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.argocd[count.index].zone_id
  }
}

resource "aws_route53_record" "argocd_private" {
  depends_on   = [aws_route53_zone.orch_private]
  count        = var.lb_created ? 1 : 0
  zone_id      = var.create_root_domain ? aws_route53_zone.orch_private[0].zone_id : data.aws_route53_zone.orch_private[0].zone_id
  name         = "argocd.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.argocd[count.index].dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.argocd[count.index].zone_id
  }
}

resource "aws_route53_record" "gitea_public" {
  depends_on   = [aws_route53_zone.orch_public]
  count        = var.lb_created ? 1 : 0
  zone_id      = var.create_root_domain ? aws_route53_zone.orch_public[0].zone_id : data.aws_route53_zone.orch_public[0].zone_id
  name         = "gitea.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.argocd[count.index].dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.argocd[count.index].zone_id
  }
}

resource "aws_route53_record" "gitea_private" {
  depends_on   = [aws_route53_zone.orch_private]
  count        = var.lb_created ? 1 : 0
  zone_id      = var.create_root_domain ? aws_route53_zone.orch_private[0].zone_id : data.aws_route53_zone.orch_private[0].zone_id
  name         = "gitea.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.argocd[count.index].dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.argocd[count.index].zone_id
  }
}

resource "aws_route53_record" "traefik2_public" {
  depends_on   = [aws_route53_zone.orch_public]
  count        = var.lb_created ? 1 : 0
  zone_id      = var.create_root_domain ? aws_route53_zone.orch_public[0].zone_id : data.aws_route53_zone.orch_public[0].zone_id
  name         = "traefik2.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.traefik2[count.index].dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.traefik2[count.index].zone_id
  }
}

resource "aws_route53_record" "traefik2_private" {
  depends_on   = [aws_route53_zone.orch_private]
  count        = var.lb_created ? 1 : 0
  zone_id      = var.create_root_domain ? aws_route53_zone.orch_private[0].zone_id : data.aws_route53_zone.orch_private[0].zone_id
  name         = "traefik2.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.traefik2[count.index].dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.traefik2[count.index].zone_id
  }
}

resource "aws_route53_record" "traefik3_public" {
  depends_on   = [aws_route53_zone.orch_public]
  count        = var.lb_created ? 1 : 0
  zone_id      = var.create_root_domain ? aws_route53_zone.orch_public[0].zone_id : data.aws_route53_zone.orch_public[0].zone_id
  name         = "traefik3.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.traefik3[count.index].dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.traefik3[count.index].zone_id
  }
}

resource "aws_route53_record" "traefik3_private" {
  depends_on   = [aws_route53_zone.orch_private]
  count        = var.lb_created ? 1 : 0
  zone_id      = var.create_root_domain ? aws_route53_zone.orch_private[0].zone_id : data.aws_route53_zone.orch_private[0].zone_id
  name         = "traefik3.${local.orch_zone}"
  type         = "A"

  alias {
    name                   = data.aws_lb.traefik3[count.index].dns_name
    evaluate_target_health = true
    zone_id                = data.aws_lb.traefik3[count.index].zone_id
  }
}

resource "aws_route53_record" "public_hostname" {
  for_each = toset(var.hostname)
  name     = "${each.value}.${local.orch_zone}"
  zone_id  = var.create_root_domain ? aws_route53_zone.orch_public[0].zone_id : data.aws_route53_zone.orch_public[0].zone_id
  ttl      = 900
  type     = "CNAME"
  records  = ["${local.orch_zone}"]
}

resource "aws_route53_record" "private_hostname" {
  for_each = toset(var.hostname)
  name     = "${each.value}.${local.orch_zone}"
  zone_id  = var.create_root_domain ? aws_route53_zone.orch_private[0].zone_id : data.aws_route53_zone.orch_private[0].zone_id
  ttl      = 900
  type     = "CNAME"
  records  = ["${local.orch_zone}"]
}

resource "aws_route53_record" "public_hostname_traefik2" {
  for_each = toset(var.traefik2_hostname)
  name     = "${each.value}.${local.orch_zone}"
  zone_id  = var.create_root_domain ? aws_route53_zone.orch_public[0].zone_id : data.aws_route53_zone.orch_public[0].zone_id
  ttl      = 900
  type     = "CNAME"
  records  = ["traefik2.${local.orch_zone}"]
}

resource "aws_route53_record" "private_hostname_traefik2" {
  for_each = toset(var.traefik2_hostname)
  name     = "${each.value}.${local.orch_zone}"
  zone_id  = var.create_root_domain ? aws_route53_zone.orch_private[0].zone_id : data.aws_route53_zone.orch_private[0].zone_id
  ttl      = 900
  type     = "CNAME"
  records  = ["traefik2.${local.orch_zone}"]
}

resource "aws_route53_record" "public_hostname_traefik3" {
  for_each = toset(var.traefik3_hostname)
  name     = "${each.value}.${local.orch_zone}"
  zone_id  = var.create_root_domain ? aws_route53_zone.orch_public[0].zone_id : data.aws_route53_zone.orch_public[0].zone_id
  ttl      = 900
  type     = "CNAME"
  records  = ["traefik3.${local.orch_zone}"]
}

resource "aws_route53_record" "private_hostname_traefik3" {
  for_each = toset(var.traefik3_hostname)
  name     = "${each.value}.${local.orch_zone}"
  zone_id  = var.create_root_domain ? aws_route53_zone.orch_private[0].zone_id : data.aws_route53_zone.orch_private[0].zone_id
  ttl      = 900
  type     = "CNAME"
  records  = ["traefik3.${local.orch_zone}"]
}