#!/bin/bash
# SPDX-FileCopyrightText: (C) 2026 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

cluster_fqdn=$1
project_name="project-2"
user_name="p-edge-onboarding-2"
password="Samp1ePassw@rd"
duration=600
echo "Fetching API token..."
API_TOKEN=$(curl -v -s -k -X POST \
  "https://keycloak.${cluster_fqdn}/realms/master/protocol/openid-connect/token" \
  -d "username=${user_name}" \
  -d "password=${password}" \
  -d "grant_type=password" \
  -d "client_id=system-client" \
  -d "scope=openid" \
  --fail-with-body | jq -r '.access_token')

if [[ -z "${API_TOKEN}" || "${API_TOKEN}" == "null" ]]; then
  echo "Failed to get API token"
  exit 1
fi

echo "Fetching Project ID..."
projectID=$(curl -v -k -s -X GET \
  "https://api.${cluster_fqdn}/v1/projects?member-role=true" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer ${API_TOKEN}" | jq -r '.[].status.projectStatus.uID')

echo "ProjectID=${projectID}"

start_time=$(date -u -d "${duration} minutes ago" +%s%N)
end_time=$(date -u +%s%N)

echo "starttime: ${start_time} endtime: ${end_time}"

# Stop port forwarding if already running
echo "Stopping existing port forwarding (if any)..."
kill $(lsof -t -i :8087) &>/dev/null || true

# Start port forwarding
echo "Starting port forwarding..."
kubectl port-forward -n orch-infra svc/edgenode-observability-loki-gateway 8087:80 &

PF_PID=$!
sleep 3

EN_LOKI_URL="localhost:8087"

echo "Querying Loki logs..."
curl -s -G "http://${EN_LOKI_URL}/loki/api/v1/query_range" \
  -H "Accept: application/json" \
  -H "X-Scope-OrgID: ${projectID}" \
  --data-urlencode "start=${start_time}" \
  --data-urlencode "end=${end_time}" \
  --data-urlencode "direction=forward" \
  --data-urlencode 'query={file_type="uOS_bootkitLogs"}' \
  | jq -r '.data.result[] | .values[] | .[]'

echo "End of uOS_bootkitLogs"

# Stop port forwarding
echo "Stopping port forwarding..."
kill ${PF_PID} &>/dev/null || true

echo "Done."
