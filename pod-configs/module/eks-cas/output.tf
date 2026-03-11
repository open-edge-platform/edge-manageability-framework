# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "debug_cas_vars" {
  value = {
    hp  = var.CAS_HP
    hps = var.CAS_HPS
    np  = var.CAS_NP
  }
}

