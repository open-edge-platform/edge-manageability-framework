provider "proxmox" {
  pm_api_url      = var.pm_api_url
  pm_debug        = true
  pm_tls_insecure = true
  pm_user         = var.pm_user
  pm_password     = var.pm_password
}
