# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "qemu_uri" {
  type        = string
  description = "The URI of the QEMU connection."
  default     = "qemu:///system"
}

variable "network_name" {
  type        = string
  description = "The name of the network"
  default     = "edge"
}

variable "network_mode" {
  type        = string
  description = "The mode of the network"
  default     = "nat"
}

variable "network_bridge" {
  type        = string
  description = "The bridge device defines the name of a bridge device which will be used to construct the virtual network."
  default     = "virbr1"
}

variable "network_subnet_cidrs" {
  type        = list(string)
  description = "The CIDRs of the network"
  default     = ["192.168.99.0/24"]
}

variable "dns_domain" {
  type        = string
  description = "The DNS domain"
  # TODO: Change this to the correct domain name
  # default     = "cluster.onprem"
  default     = "demo.onprem.espdqa.infra-host.com"
}

variable "dns_hosts" {
  type = list(object({
    hostname = string
    ip       = string
  }))
  description = "The list of DNS hosts"
  default = [
    { hostname = "alerting-monitor", ip = "192.168.99.30" },
    { hostname = "api", ip = "192.168.99.30" },
    { hostname = "app-orch", ip = "192.168.99.30" },
    { hostname = "app-service-proxy", ip = "192.168.99.30" },
    { hostname = "argocd", ip = "192.168.99.20" },
    { hostname = "attest-node", ip = "192.168.99.30" },
    { hostname = "cluster-orch-edge-node", ip = "192.168.99.30" },
    { hostname = "cluster-orch-node", ip = "192.168.99.30" },
    { hostname = "connect-gateway", ip = "192.168.99.30" },
    { hostname = "fleet", ip = "192.168.99.30" },
    { hostname = "infra-node", ip = "192.168.99.30" },
    { hostname = "keycloak", ip = "192.168.99.30" },
    { hostname = "license-node", ip = "192.168.99.30" },
    { hostname = "log-query", ip = "192.168.99.30" },
    { hostname = "logs-node", ip = "192.168.99.30" },
    { hostname = "metadata", ip = "192.168.99.30" },
    { hostname = "metrics-node", ip = "192.168.99.30" },
    { hostname = "observability-admin", ip = "192.168.99.30" },
    { hostname = "observability-ui", ip = "192.168.99.30" },
    { hostname = "onboarding-node", ip = "192.168.99.30" },
    { hostname = "onboarding-stream", ip = "192.168.99.30" },
    { hostname = "registry-oci", ip = "192.168.99.30" },
    { hostname = "registry", ip = "192.168.99.30" },
    { hostname = "release", ip = "192.168.99.30" },
    { hostname = "telemetry-node", ip = "192.168.99.30" },
    { hostname = "tinkerbell-nginx", ip = "192.168.99.40" },
    { hostname = "tinkerbell-server", ip = "192.168.99.30" },
    { hostname = "update-node", ip = "192.168.99.30" },
    { hostname = "vault", ip = "192.168.99.30" },
    { hostname = "vnc", ip = "192.168.99.30" },
    { hostname = "web-ui", ip = "192.168.99.30" },
    { hostname = "ws-app-service-proxy", ip = "192.168.99.30" },
  ]
}

variable "dns_resolvers" {
  type        = list(string)
  description = "The list of DNS resolvers dnsmasq will forward queries to. They must be specified as IP addresses."
}
