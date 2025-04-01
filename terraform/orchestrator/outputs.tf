# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "vm_name" {
  value       = libvirt_domain.orch-vm.name
  description = "The name of the Orchestrator VM"
}

output "vm_ip_address" {
  value       = local.vmnet_ip0
  description = "The SSH IPv4 address for the Orchestrator VM"
}
