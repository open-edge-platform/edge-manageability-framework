# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "name" {
  value = libvirt_network.edge_network.name
}

output "domain" {
  value = libvirt_network.edge_network.domain
}

output "bridge" {
  value = libvirt_network.edge_network.bridge
}

output "network_subnet_cidrs" {
  value = var.network_subnet_cidrs
}

output "dns_resolvers" {
  value = var.dns_resolvers
}
