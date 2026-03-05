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
echo "Start uOS_bootkitLogs"
curl -s -G "http://${EN_LOKI_URL}/loki/api/v1/query_range" \
  -H 'Accept: application/json' \
  -H "X-Scope-OrgID: ${projectID}" \
  --data-urlencode "start=${start_time}" \
  --data-urlencode "end=${end_time}" \
  --data-urlencode "direction=forward" \
  --data-urlencode 'query={file_type="uOS_bootkitLogs"}' \
  | jq -r '.data.result[]?.values[]? | .[]' > uOS_bootkit.log

cat uOS_bootkit.log || true
echo "Start uOS_caddyLogs"
curl -s -G "http://${EN_LOKI_URL}/loki/api/v1/query_range" \
  -H 'Accept: application/json' \
  -H "X-Scope-OrgID: ${projectID}" \
  --data-urlencode "start=${start_time}" \
  --data-urlencode "end=${end_time}" \
  --data-urlencode "direction=forward" \
  --data-urlencode 'query={file_type="uOS_caddyLogs"}' \
  | jq -r '.data.result[]?.values[]? | .[]' > uOS_caddy.log
cat uOS_caddy.log || true

kill $(lsof -t -i :8087) 2>/dev/null || true
echo "Port forwarding stopped"
