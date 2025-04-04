# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "qemu_uri" {
  type        = string
  description = "The URI of the QEMU connection."
  default     = "qemu:///system"
}

variable "vm_create_timeout" {
  type        = string
  description = "The timeout for creating the VM."
  default     = "30m"
}

variable "vm_domain_type" {
  type        = string
  description = "The type of domain to use for the VM."
  default     = "kvm"
}

variable "storage_pool" {
  type        = string
  description = "The name of the storage pool to use for the VM."
  default     = "edge"
}

variable "vm_boot_disk_size" {
  type        = string
  description = "The size of the VM's boot disk. Accepts K for kibibytes, M for mebibytes, G for gibibytes, T for tibibytes."
  default     = "512G"
}

variable "os_disk_image_url" {
  type        = string
  description = "The URL of the OS disk image"
  default     = "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img"
}

variable "vm_name" {
  type        = string
  description = "The name of the VM."
  default     = "orch-tf"
}

variable "vm_user" {
  type        = string
  description = "The user name to use for the VM."
  default     = "ubuntu"
}

variable "vm_vcpu" {
  type        = number
  description = "The number of virtual CPUs to use for the VM."
  default     = 24
}

variable "vm_memory" {
  type        = number
  description = "The amount of memory to use for the VM."
  default     = 65536
}

variable "vm_enable_hugepages" {
  type        = bool
  description = "Whether to enable hugepages for the VM. Huge pages must be enabled on the host."
  default     = false
}

variable "vm_cpu_mode" {
  type        = string
  description = "The CPU mode to use for the VM."
  default     = "host-passthrough"
}

variable "ntp_server" {
  type        = string
  description = "NTP server the VM must use."
  default     = "ntp.ubuntu.com"
}

variable "network_name" {
  type        = string
  description = "The name of the network"
  default     = "edge"
}

variable "nameservers" {
  type        = list(string)
  description = "List of nameservers the VM must use. If null, the VM will use the default nameservers provided through DHCP."
  default     = null
}

variable "http_proxy" {
  type        = string
  description = "Sets the HTTP_PROXY environment variable in the VM."
  default     = ""
}

variable "https_proxy" {
  type        = string
  description = "Sets the HTTPS_PROXY environment variable in the VM."
  default     = ""
}

variable "no_proxy" {
  type        = string
  description = "Sets the NO_PROXY environment variable in the VM."
  default     = ""
}

variable "ftp_proxy" {
  type        = string
  description = "Sets the FTP_PROXY environment variable in the VM."
  default     = ""
}

variable "socks_proxy" {
  type        = string
  description = "Sets the SOCKS_PROXY environment variable in the VM."
  default     = ""
}

variable "ca_certificates" {
  type        = list(string)
  description = "List of CA certificates file paths for the VM to trust."
  default     = []
}

variable "install_docker" {
  type        = bool
  description = "Whether to install Docker on the VM."
  default     = false
}

variable "extra_packages" {
  type        = list(string)
  description = "List of additional packages to install on the VM."
  default     = []
}

variable "ssh_public_key" {
  type        = string
  description = "The public SSH key to use for authentication to the VM."
  default     = ""
}

variable "runcmd" {
  type        = list(string)
  description = "List of commands to run on the VM after the first boot."
  default     = []
}

variable "enable_auto_install" {
  type        = bool
  description = "Enable auto installation of Orchestrator."
  default     = true
}

variable "deploy_tag" {
  type        = string
  description = "Orchestrator release tag version. If empty, will use the default production tag."
  default     = "latest-main-dev"
}

variable "use_local_build_artifact" {
  type        = bool
  description = "Path to the local Orchestrator artifacts relative to working directory."
  default     = false
}

variable "local_installers_path" {
  type        = string
  description = "Path to the local Orchestrator installers relative to working directory."
  default     = "dist"
}

variable "local_repo_archives_path" {
  type        = string
  description = "Path to the local repo archive relative to working directory."
  default     = "repo_archives"
}

variable "working_directory" {
  type        = string
  description = "The directory where the Terraform files are located."
  default     = "."
}

variable "orch_profile" {
  type        = string
  description = "Orchestrator configuration profile. If empty, will use the default production profile."
  default     = "onprem"
}

variable "release_service_refresh_token" {
  type        = string
  description = "Release service OAuth 2.0 refresh token."
  default     = "TODO: Make this required when we have private Release service artifacts"
}

variable "rs_url" {
  type        = string
  description = "Orchestrator release service URL. If empty, will use the default development release service URL."
  default     = "registry-rs.edgeorchestration.intel.com"
}

variable "cluster_domain" {
  type        = string
  description = "Domain of the cluster"
  default     = "cluster.onprem"
}

variable "vm_ssh_port" {
  type        = number
  description = "The port to use for the VM SSH connection."
  default     = 22
}

variable "vm_ssh_user" {
  type        = string
  description = "The user name to use for the VM SSH connection."
  default     = "ubuntu"
}

variable "vm_ssh_password" {
  type        = string
  description = "The password to use for the VM SSH connection."
  default     = "ubuntu"
}

variable "override_flag" {
  type        = bool
  description = "Whether to use the --override flag for the onprem_installer.sh script."
  default     = false
}

variable "docker_username" {
  type        = string
  description = "Docker.io username"
  default     = ""
}

variable "docker_password" {
  type        = string
  description = "Docker.io password"
  default     = ""
}

variable "gitea_image_registry" {
  type        = string
  description = "Gitea image registry"
  default     = "docker.io"
}

variable "locally_built_artifacts" {
  type        = bool
  description = "Whether to use locally built artifacts"
  default     = false
}

variable "vm_boot_disk_name" {
  type        = string
  description = "The name of the VM's boot disk."
  default     = "ubuntu-volume-1"
}

variable "enable_cpu_pinning" {
  type        = bool
  description = "Whether to enable CPU pinning for the VM."
  default     = true
}

variable "vmnet_ips" {
  type        = list(string)
  description = "List of IP addresses with CIDR."
  default     = ["192.168.99.10/24", "192.168.99.20/24", "192.168.99.30/24", "192.168.99.40/24"]
}
