# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "qemu_uri" {
  type        = string
  description = "The URI of the QEMU connection."
  default     = "qemu:///system"
}

variable "name" {
    type        = string
    description = "The name of the storage pool."
    default     = "edge"
}

variable "target_path" {
    type        = string
    description = "The target directory path for the storage pool."
    default     = "/var/lib/libvirt/images"
}
