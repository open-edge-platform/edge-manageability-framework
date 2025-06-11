# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "parent_zone" {
  description = "The route53 zone name of the parent"
}
variable "orch_name" {
  description = "The Orchestrator cluster name"
}
variable "host_name" {
  description = "The host name of the root domain"
  default     = ""
}
variable "vpc_id" {
  description = "The VPC ID for the private route53 zone"
}
variable "vpc_region" {
  description = "The VPC region for the private route53 zone"
}
variable "hostname" {
  type    = list(string)
  default = [
    "alerting-monitor",
    "api",
    "api-proxy",  # Deprecated in the next version, see LPDF-512
    "app-orch",
    "app-service-proxy",
    "attest-node",
    "cluster-orch-edge-node",
    "cluster-orch-node",
    "connect-gateway",
    "fleet",
    "infra-node",
    "keycloak",
    "logs",
    "logs-node",
    "log-query",
    "metadata",
    "metrics-node",
    "mps-node",
    "mps-webport-node",
    "observability-admin",
    "observability-ui",
    "onboarding-node",
    "onboarding-stream",
    "registry",
    "registry-oci",
    "release",
    "rps-node",
    "rps-webport-node",
    "telemetry-node",
    "tinkerbell-server",
    "update-node",
    "vault",
    "vault-edge-node",
    "vcm",
    "vnc",
    "web-ui"]
}

# No host list variable for the LB of argocd is needed because "argocd" is the only subdomain on that LB

variable "traefik2_hostname" {
  type    = list(string)
  default = [
    "tinkerbell-nginx"]
}

variable "lb_created" {
  type        = bool
  description = "Whether the LBs for the Orchestrator are created. The CNAME of {orch_name}.{parent_zone} will be created if it is true."
  default     = false
}

variable "create_root_domain" {
  type        = bool
  description = "Whether to create the root_domain."
  default     = true
}

variable "enable_pull_through_cache_proxy" {
  type        = bool
  description = "Whether to enable the pull through cache proxy."
  default     = false
}
