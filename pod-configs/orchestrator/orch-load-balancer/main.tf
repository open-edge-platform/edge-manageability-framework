# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

data "terraform_remote_state" "vpc" {
  backend = "s3"
  config = {
    bucket = var.vpc_terraform_backend_bucket
    key    = var.vpc_terraform_backend_key
    region = var.vpc_terraform_backend_region
  }
}

data "terraform_remote_state" "eks" {
  backend = "s3"
  config = {
    bucket = var.cluster_terraform_backend_bucket
    key    = var.cluster_terraform_backend_key
    region = var.cluster_terraform_backend_region
  }
}

module "ap_tls_cert" {
  source            = "../../module/acm_import"
  certificate_body  = var.tls_cert_body
  certificate_chain = var.tls_cert_chain
  private_key       = var.tls_key
  cluster_name      = var.cluster_name
}

data "aws_nat_gateways" "vpc_nat_gateways" {
  vpc_id   = local.vpc_id
}

data "aws_nat_gateway" "vpc_nat_gateway" {
  for_each = toset(data.aws_nat_gateways.vpc_nat_gateways.ids)
  id       = each.value
  vpc_id   = local.vpc_id
  state    = "available"
}

locals {
  public_subnet_ids = [for name, subnet in data.terraform_remote_state.vpc.outputs.public_subnets : subnet.id]
  vpc_id            = data.terraform_remote_state.vpc.outputs.vpc_id
  region            = data.terraform_remote_state.vpc.outputs.region
  nat_public_ips    = toset([for id, nat in data.aws_nat_gateway.vpc_nat_gateway : "${nat.public_ip}/32" if nat.connectivity_type == "public"])
  ip_allow_list     = setunion(var.ip_allow_list, local.nat_public_ips)
  listeners = {
    "https" : {
      listen          = 443
      protocol        = "HTTPS"
      certificate_arn = module.ap_tls_cert.cert.arn
    }
  }
  default_target_groups = {
    "default" : {
      listener = "https"
      type     = "ip"
    }
  }
  infra_service_target_groups = {
    "argocd" : {
      listener = "https"
      type     = "ip"
      match_hosts = ["argocd.*"]
    },
    "gitea" : {
      listener = "https"
      type     = "ip"
      match_hosts = ["gitea.*"]
    }
  }
  traefik_target_groups = {
    "default" : {
      listener                          = "https"
      type                              = "ip"
      expected_health_check_status_code = 404
    },
    "grpc" : {
      listener                          = "https"
      protocol_version                  = "GRPC"
      expected_health_check_status_code = 0
      type                              = "ip"
      match_headers = {
        "content-type" = "application/grpc*"
      }
    }
  }
  nlb_ports = {
    "https" : {
      listen              = 443
      target              = 31443
      type                = "ip"
      protocol            = "TCP"
      enable_health_check = true
    }
  }

  vpro_ports = {
    "vpro" : {
      listen              = 4433
      target              = 4433
      type                = "ip"
      protocol            = "TCP"
      enable_health_check = true
    }
  }
}

module "traefik_load_balancer" {
  source                     = "../../module/application-load-balancer"
  name                       = "traefik"
  internal                   = var.internal
  vpc_id                     = local.vpc_id
  cluster_name               = var.cluster_name
  subnets                    = local.public_subnet_ids
  ip_allow_list              = local.ip_allow_list
  listeners                  = local.listeners
  target_groups              = local.traefik_target_groups
  enable_deletion_protection = var.enable_deletion_protection
}

# This block executes only when `create_traefik2_load_balancer` is set to true
module "traefik2_load_balancer" {
  count = var.create_traefik2_load_balancer ? 1 : 0

  source                     = "../../module/load-balancer"
  name                       = "traefik2"
  internal                   = var.internal
  vpc_id                     = local.vpc_id
  cluster_name               = var.cluster_name
  subnets                    = local.public_subnet_ids
  ip_allow_list              = local.ip_allow_list
  ports                      = local.nlb_ports
  enable_deletion_protection = var.enable_deletion_protection
}

module "traefik3_load_balancer" {
  count = var.create_traefik3_load_balancer ? 1 : 0

  source                     = "../../module/load-balancer"
  name                       = "traefik3"
  internal                   = var.internal
  vpc_id                     = local.vpc_id
  cluster_name               = var.cluster_name
  subnets                    = local.public_subnet_ids
  ip_allow_list              = local.ip_allow_list
  ports                      = local.vpro_ports
  enable_deletion_protection = var.enable_deletion_protection
}

# This block executes only when `create_argocd_load_balancer` is set to true
# Dedicated load balancer is necessary for integration env
module "argocd_load_balancer" {
  count = var.create_argocd_load_balancer ? 1 : 0

  source                     = "../../module/application-load-balancer"
  name                       = "argocd"
  internal                   = var.internal
  vpc_id                     = local.vpc_id
  cluster_name               = var.cluster_name
  subnets                    = local.public_subnet_ids
  ip_allow_list              = local.ip_allow_list
  listeners                  = local.listeners
  target_groups              = local.infra_service_target_groups
  enable_deletion_protection = var.enable_deletion_protection
}

# This block executes only when `create_target_group_binding` is set to true
module "traefik_lb_target_group_binding" {
  count = var.create_target_group_binding ? 1 : 0

  source = "../../module/aws-lb-target-group-binding"
  bindings = {
    "traefik-https" : {
      serviceNamespace = "orch-gateway"
      serviceName      = "traefik"
      servicePort      = 443
      target_id        = module.traefik_load_balancer.target_groups["default"].arn
    },
    "traefik-grpc" : {
      serviceNamespace = "orch-gateway"
      serviceName      = "traefik"
      servicePort      = 443
      target_id        = module.traefik_load_balancer.target_groups["grpc"].arn
    }
    "ingress-nginx-controller" : {
      serviceNamespace = "orch-boots"
      serviceName      = "ingress-nginx-controller"
      servicePort      = 443
      target_id        = module.traefik2_load_balancer[0].target_groups["https"].arn
    },
    "traefik-vpro" : {
      serviceNamespace = "orch-gateway"
      serviceName      = "traefik"
      servicePort      = 4433
      target_id        = module.traefik3_load_balancer[0].target_groups["vpro"].arn
    },
    "argocd" : {
      serviceNamespace = "argocd"
      serviceName      = "argocd-server"
      servicePort      = 443
      target_id        = module.argocd_load_balancer[0].target_groups["argocd"].arn
    },
    "gitea" : {
      serviceNamespace = "gitea"
      serviceName      = "gitea-http"
      servicePort      = 443
      target_id        = module.argocd_load_balancer[0].target_groups["gitea"].arn
    }
  }
}

module "aws_lb_security_group_roles" {
  source = "../../module/aws-lb-security-group-roles"
  eks_node_sg_id = data.terraform_remote_state.eks.outputs.eks_nodegroup_instance_sg_ids[0]
  lb_sg_ids = {
    "traefik": {
      port = 8443,
      security_group_id = module.traefik_load_balancer.lb_sg_id
    },
    "traefik2": {
      port = 443,
      security_group_id = module.traefik2_load_balancer[0].lb_sg_id
    },
    "argocd": {
      port = 8080,
      security_group_id = module.argocd_load_balancer[0].lb_sg_id
    },
    "gitea": {
      port = 3000,
      security_group_id = module.argocd_load_balancer[0].lb_sg_id
    },
    "vpro": {
      port = 4433,
      security_group_id = module.traefik3_load_balancer[0].lb_sg_id
    }
  }
}

module "wait_until_alb_ready" {
  source = "../../module/wait-until-alb-ready"
  traefik_alb_arn = module.traefik_load_balancer.lb_arn
  argocd_alb_arn = module.argocd_load_balancer[0].lb_arn
}

# WAF for load balancers
module "waf_web_acl_traefik" {
  depends_on = [ module.wait_until_alb_ready ]
  source                 = "../../module/waf-web-acl"
  name                   = "${var.cluster_name}-traefik"
  assiciate_resource_arn = module.traefik_load_balancer.lb_arn
}
module "waf_web_acl_argocd" {
  depends_on = [ module.wait_until_alb_ready ]
  count                  = var.create_argocd_load_balancer ? 1 : 0
  source                 = "../../module/waf-web-acl"
  name                   = "${var.cluster_name}-argocd"
  assiciate_resource_arn = module.argocd_load_balancer[0].lb_arn
}
