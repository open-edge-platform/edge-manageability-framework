# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  # Use the generated VMID from the random_integer resource
  vmid = random_integer.next_vmid.result
}