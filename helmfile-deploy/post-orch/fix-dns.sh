#!/bin/bash
# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
# Fix DNS on EMF orchestrator node. Run as: sudo ./fix-dns.sh

set -euo pipefail

DNS="10.248.2.1 10.22.224.196"
FALLBACK="8.8.8.8 8.8.4.4"

mkdir -p /etc/systemd/resolved.conf.d
cat > /etc/systemd/resolved.conf.d/dns.conf <<EOF
[Resolve]
DNS=${DNS}
FallbackDNS=${FALLBACK}
Domains=~.
EOF
systemctl restart systemd-resolved
sed -i '/proxy-dmz.intel.com/d' /etc/hosts 2>/dev/null || true

echo "✅ DNS fixed (${DNS})"
