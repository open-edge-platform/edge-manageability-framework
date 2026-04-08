#!/bin/bash
# SPDX-FileCopyrightText: (C) 2026 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

# Check arguments
if [ "$#" -ne 4 ]; then
  echo "Usage: $0 <cluster_fqdn> <project_name> <username> <password>"
  exit 1
fi

cluster_fqdn="$1"
project_name="$2"
user_name="$3"
password="$4"

duration=600
# Maximum number of log entries per Loki request (Loki default hard limit is 5000)
LOKI_BATCH_LIMIT=5000

echo "Cluster FQDN: $cluster_fqdn"
echo "Project Name: $project_name"
echo "Username: $user_name"

echo "Fetching API token..."

API_TOKEN=$(curl -s -k -X POST \
  "https://keycloak.${cluster_fqdn}/realms/master/protocol/openid-connect/token" \
  -d "username=${user_name}" \
  -d "password=${password}" \
  -d "grant_type=password" \
  -d "client_id=system-client" \
  -d "scope=openid" | jq -r '.access_token')

if [ -z "$API_TOKEN" ] || [ "$API_TOKEN" = "null" ]; then
  echo "ERROR: Failed to fetch API token"
  exit 1
fi

projectID=$(curl -s -X GET "https://api.${cluster_fqdn}/v1/projects?member-role=true" \
  -H 'Accept: application/json' \
  -H "Authorization: Bearer ${API_TOKEN}" | jq -r '.[].status.projectStatus.uID')

echo "ProjectID=$projectID"

start_time=$(date -u -d "$duration minutes ago" +%s%N)
end_time=$(date -u +%s%N)

echo "starttime:$start_time endtime:$end_time"

echo "Stop Port forwarding if already enabled"
kill $(lsof -t -i :8087) 2>/dev/null || true

kubectl port-forward -n orch-infra svc/edgenode-observability-loki-gateway 8087:80 >/dev/null 2>&1 &
echo "Port forwarding enabled"
sleep 3

EN_LOKI_URL="localhost:8087"

# fetch_loki_logs: Fetches all log entries from Loki for a given log stream,
# paginating through results if the response hits the batch size limit.
#
# Arguments:
#   $1 - log_type : Loki label selector value for file_type (e.g. "uOS_bootkitLogs")
#   $2 - output_file : path to the output file where logs will be written
#   $3 - range_start : start timestamp in nanoseconds (epoch)
#   $4 - range_end   : end timestamp in nanoseconds (epoch)
fetch_loki_logs() {
  local log_type="$1"
  local output_file="$2"
  local range_start="$3"
  local range_end="$4"

  local current_start="$range_start"
  local batch_num=0
  local total_entries=0

  # Truncate/create output file
  > "$output_file"

  echo "Fetching logs for log_type=${log_type} ..."

  while true; do
    batch_num=$((batch_num + 1))
    echo "  Fetching batch #${batch_num} (start=${current_start}, end=${range_end}, limit=${LOKI_BATCH_LIMIT})..."

    local response
    response=$(curl -s -G "http://${EN_LOKI_URL}/loki/api/v1/query_range" \
      -H 'Accept: application/json' \
      -H "X-Scope-OrgID: ${projectID}" \
      --data-urlencode "start=${current_start}" \
      --data-urlencode "end=${range_end}" \
      --data-urlencode "direction=forward" \
      --data-urlencode "limit=${LOKI_BATCH_LIMIT}" \
      --data-urlencode "query={file_type=\"${log_type}\"}")

    # Extract log lines and append to output file
    echo "$response" | jq -r '.data.result[]?.values[]? | .[]' >> "$output_file"

    # Count entries returned in this batch by counting timestamps across all streams
    local batch_count
    batch_count=$(echo "$response" | jq '[.data.result[]?.values[]?] | length')

    total_entries=$((total_entries + batch_count))
    echo "  Batch #${batch_num}: received ${batch_count} entries (total so far: ${total_entries})"

    # If fewer entries than the limit were returned, we have fetched all available data
    if [ "$batch_count" -lt "$LOKI_BATCH_LIMIT" ]; then
      echo "  All entries fetched for log_type=${log_type} (total: ${total_entries})"
      break
    fi

    # Advance start_time to last entry timestamp + 1 nanosecond to avoid re-fetching
    local last_ts
    last_ts=$(echo "$response" | jq -r '[.data.result[]?.values[]?] | last | .[0]')

    if [ -z "$last_ts" ] || [ "$last_ts" = "null" ]; then
      echo "  WARNING: Could not determine last timestamp in batch #${batch_num}, stopping pagination."
      break
    fi

    current_start=$((last_ts + 1))

    # Safety check: if the new start exceeds or equals end_time, stop
    if [ "$current_start" -ge "$range_end" ]; then
      echo "  Reached end of time range, stopping pagination."
      break
    fi
  done
}

echo "Start uOS_bootkitLogs"
fetch_loki_logs "uOS_bootkitLogs" "uOS_bootkit.log" "$start_time" "$end_time"
cat uOS_bootkit.log || true

echo "Start uOS_caddyLogs"
fetch_loki_logs "uOS_caddyLogs" "uOS_caddy.log" "$start_time" "$end_time"
cat uOS_caddy.log || true

kill $(lsof -t -i :8087) 2>/dev/null || true
echo "Port forwarding stopped"
