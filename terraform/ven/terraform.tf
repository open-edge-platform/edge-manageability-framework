# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

terraform {
  required_version = ">= 1.8.5"
  required_providers {
    local = {
      source  = "hashicorp/local"
      version = "2.5.1"
    }

    null = {
      source  = "hashicorp/null"
      version = "3.2.2"
    }

    proxmox = {
      source  = "Telmate/proxmox"
      version = "3.0.1-rc3"
    }
  }
}
