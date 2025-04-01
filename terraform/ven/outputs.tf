output "vm_id" {
  value       = proxmox_vm_qemu.ven.vmid
  description = "The ENiVM Proxmox VM ID"
}

output "vm_name" {
  value       = proxmox_vm_qemu.ven.name
  description = "The ENiVM Proxmox VM Name"
}

output "vm_ip_address" {
  value       = data.external.vm_ip_address.result["vm_ssh_host"]
  description = "The SSH IPv4 address for the Orchestrator VM"
}