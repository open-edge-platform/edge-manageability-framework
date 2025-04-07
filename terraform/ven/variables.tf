variable "pm_api_url" {
  type        = string
  description = "The URL of the Proxmox VE API."
  default     = "https://localhost:8006/api2/json"
}

variable "pm_user" {
  type        = string
  description = "The user name to use for the Proxmox VE API connection."
  default     = "root@pam"
}

variable "pm_password" {
  type        = string
  description = "The password to use for the Proxmox VE API connection."
}

variable "pm_ssh_host" {
  type        = string
  description = "The host name to use for the Proxmox VE SSH connection."
  default     = "localhost"
}

variable "pm_ssh_port" {
  type        = string
  description = "The port to use for the Proxmox VE SSH connection."
  default     = "22"
}

variable "pm_ssh_user" {
  type        = string
  description = "The user name to use for the Proxmox VE SSH connection."
  default     = "root"
}

variable "pm_ssh_password" {
  type        = string
  description = "The password to use for the Proxmox VE SSH connection."
}

variable "target_node" {
  type        = string
  description = "The target Proxmox VE node to create to VMs on."
}

variable "working_directory" {
  type        = string
  description = "The directory where the Terraform files are located."
  default     = "."
}

variable "vm_boot_disk_size" {
  type        = string
  description = "The size of the VM's boot disk. Accepts K for kibibytes, M for mebibytes, G for gibibytes, T for tibibytes."
  default     = "128G"
}

variable "vm_boot_disk_image_file" {
  type        = string
  description = "The cloud-init image to import the boot disk from."
  default     = "/var/lib/vz/template/iso/jammy-server-cloudimg-amd64.img"
}

#variable "vm_data_disk_size" {
#  type        = string
#  description = "The size of the VM's data disk. Accepts K for kibibytes, M for mebibytes, G for gibibytes, T for tibibytes."
#  default     = "512G"
#}

variable "vm_sockets" {
  type        = number
  description = "The number of sockets to allocate to the VM."
  default     = 1
}

variable "vm_cores" {
  type        = number
  description = "The number of CPU cores to allocate to the VM."
  default     = 16
}

variable "vm_memory" {
  type        = number
  description = "The amount of memory (in MB) to allocate to the VM."
  default     = 32000
}

variable "vm_os_type" {
  type        = string
  description = "The operating system to install on the VM."
  default     = "cloud-init"
}

variable "vm_net0_bridge" {
  type        = string
  description = "The name of the bridge to attach the net0 ethernet interface to the VM."
  default     = "vmbr0"
}

variable "vm_net0_mac_address" {
  type        = string
  description = "The MAC address of the net0 ethernet interface. Leave blank to automatically generate one."
  default     = ""
}

variable "ntp_server" {
  type        = string
  description = "NTP server the VM must use."
  default     = "ntp.ubuntu.com"
}

variable "http_proxy" {
  type        = string
  description = "The HTTP proxy the VM must use."
  default     = ""
}

variable "https_proxy" {
  type        = string
  description = "The HTTPS proxy the VM must use."
  default     = ""
}

variable "no_proxy" {
  type        = string
  description = "Sets the no_proxy environment variable in the VM."
  default     = ""
}

variable "ca_certificates" {
  type        = list(string)
  description = "List of CA certificates file paths for the VM to trust."
  default     = []
}

variable "ssh_public_key" {
  type        = string
  description = "The public SSH key to use for authentication to the VM."
  default     = ""
}

variable "agent_timeout" {
  type        = number
  description = "Timeout in seconds to keep trying to obtain an IP address from the guest agent."
  default     = 300
}

variable "vm_name" {
  type        = string
  description = "The name of the VM."
  default     = "ven-tf"
}