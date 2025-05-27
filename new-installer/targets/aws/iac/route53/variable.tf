# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "customer_tag" {
  description = "The customer tag to be used for the resources"
  type        = string
  default     = ""
}
variable "parent_zone" {
  description = "The route53 zone name of the parent"
}
variable "orch_name" {
  description = "The Orchestrator cluster name"
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
    "observability-admin",
    "observability-ui",
    "onboarding-node",
    "onboarding-stream",
    "registry",
    "registry-oci",
    "release",
    "telemetry-node",
    "tinkerbell-server",
    "update-node",
    "vault",
    "vault-edge-node",
    "vcm",
    "vnc",
    "web-ui"
  ]
}

# No host list varibale for the Infra LB is needed because "argocd" and "gitea" are only subdomains on that LB

variable "traefik2_hostname" {
  type    = list(string)
  default = [
    "tinkerbell-nginx"
  ]
}
