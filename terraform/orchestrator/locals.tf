# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  vmnet_ips = [for ip in var.vmnet_ips : split("/", ip)[0]]
  vmnet_ip0 = local.vmnet_ips[0]
  vmnet_ip1 = local.vmnet_ips[1]
  vmnet_ip2 = local.vmnet_ips[2]
  vmnet_ip3 = local.vmnet_ips[3]

  is_proxy_set = (
    var.http_proxy != "" ||
    var.https_proxy != "" ||
    var.ftp_proxy != "" ||
    var.socks_proxy != "" ||
    var.no_proxy != "" ||
    var.en_http_proxy != "" ||
    var.en_https_proxy != ""
  )
}
