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

variable "network_domain" {
  type        = string
  description = "The domain of the network"
  default     = "cluster.onprem"
}

variable "network_subnet_cidrs" {
  type        = list(string)
  description = "The CIDRs of the network"
  default     = ["192.168.99.0/24"]
}

variable "dns_hosts" {
  type = list(object({
    hostname = string
    ip       = string
  }))
  description = "The list of DNS hosts"
  default = [
    { hostname = "alerting-monitor.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "api.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "app-orch.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "app-service-proxy.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "argocd.cluster.onprem", ip = "192.168.99.20" },
    { hostname = "attest-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "cluster-orch-edge-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "cluster-orch-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "cluster.onprem", ip = "192.168.99.30" },
    { hostname = "connect-gateway.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "fleet.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "infra-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "keycloak.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "license-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "log-query.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "logs-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "metadata.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "metrics-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "observability-admin.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "observability-ui.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "onboarding-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "onboarding-stream.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "mps-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "mps-webport-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "rps-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "rps-webport-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "registry-oci.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "registry.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "release.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "telemetry-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "tinkerbell-nginx.cluster.onprem", ip = "192.168.99.40" },
    { hostname = "tinkerbell-server.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "update-node.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "vault.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "vnc.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "web-ui.cluster.onprem", ip = "192.168.99.30" },
    { hostname = "ws-app-service-proxy.cluster.onprem", ip = "192.168.99.30" },
  ]
}

variable "dhcp_boot" {
  type        = string
  description = "The DHCP boot option"
  default     = "tag:efi-http,https://tinkerbell-nginx.cluster.onprem/tink-stack/signed_ipxe.efi"
}

variable "dns_resolvers" {
  type        = list(string)
  description = "The list of DNS resolvers dnsmasq will forward queries to. They must be specified as IP addresses."
}
