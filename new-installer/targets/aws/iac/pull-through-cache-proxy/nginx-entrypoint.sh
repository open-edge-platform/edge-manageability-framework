#!/bin/sh
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

apk update
apk add --no-cache aws-cli
mkdir -p /usr/local/openresty/nginx/logs
touch /usr/local/openresty/nginx/logs/nginx.pid
touch /data/ecr_token

/usr/local/openresty/bin/openresty -c /data/nginx.conf

while true; do
  TOKEN=$(aws ecr get-authorization-token --query 'authorizationData[*].authorizationToken' --output text)
  if [ -n "$TOKEN" ]; then
    echo -n "$TOKEN" > /data/ecr_token
    echo "Info: Token updated"
    # Token expired in 12 hours, renew it every 10 hours
    sleep 36000
  else
    echo "Warn: Unable to get new token, wait and retry!"
    sleep 10
  fi
done
