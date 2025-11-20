#!/usr/bin/env bash

set -euo pipefail

SN="$1"

if [ -z "$SN" ]; then
    echo "Usage: $0 <SERIAL_NUMBER>" >&2
    exit 1
fi

# Get Host ID
HOST_ID=$(orch-cli list host | grep "$SN" | awk '{print $1}' | head -1)

if [ -z "$HOST_ID" ]; then
    echo "No host found" >&2
    exit 1
fi

# Get UUID
UUID=$(orch-cli get host "$HOST_ID" | grep -E "^\s*-\s*UUID:" | awk '{print $3}')

if [ -z "$UUID" ]; then
    echo "No UUID found" >&2
    exit 1
fi

# Output ONLY the UUID
echo "$UUID"
