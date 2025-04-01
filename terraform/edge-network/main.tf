# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "libvirt_network" "edge_network" {
  name = var.network_name

  mode = var.network_mode

  autostart = true

  domain = var.network_domain

  addresses = var.network_subnet_cidrs

  bridge = var.network_bridge

  dhcp {
    enabled = true
  }

  dns {
    enabled = true

    // Enable DNS forwarding to the host machine if a record is not found
    local_only = false

    dynamic "hosts" {
      for_each = var.dns_hosts
      content {
        hostname = hosts.value.hostname
        ip       = hosts.value.ip
      }
    }
  }

  dnsmasq_options {
    options {
      option_name  = "interface"
      option_value = var.network_bridge
    }

    // Do not forward DNS queries for the local network domain
    options {
      option_name  = "local"
      option_value = "/${var.network_domain}/"
    }

    // Forward queries to the configured DNS resolvers if the local domain is not matched
    options {
      option_name = "no-resolv"
    }
    options {
      option_name = "no-hosts"
    }
    options {
      option_name = "domain-needed"
    }
    dynamic "options" {
      for_each = var.dns_resolvers
      content {
        option_name  = "server"
        option_value = options.value
      }

    }

    // Configure DHCP options for Edge Node PXE boot
    options {
      option_name  = "dhcp-vendorclass"
      option_value = "set:efi-http,HTTPClient:Arch:00016"
    }
    options {
      option_name  = "dhcp-option-force"
      option_value = "tag:efi-http,60,HTTPClient"
    }
    options {
      option_name  = "dhcp-match"
      option_value = "set:ipxe,175"
    }
    options {
      option_name  = "dhcp-boot"
      option_value = var.dhcp_boot
    }
  }
}
