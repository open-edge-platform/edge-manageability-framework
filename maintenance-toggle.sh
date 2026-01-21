#!/bin/bash
# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script to toggle Keycloak maintenance theme via Admin API
# Usage: ./maintenance-toggle.sh {enable|disable|status}

set -e

KEYCLOAK_URL="${KEYCLOAK_URL:-https://keycloak.orch-10-114-181-230.espdqa.infra-host.com}"
MODE="${1:-status}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get admin credentials
# Username is hardcoded in platform-keycloak.yaml (auth.adminUser: admin)
ADMIN_USER="admin"

# Password from Kubernetes secret
ADMIN_PASS=$(kubectl -n orch-platform get secret platform-keycloak -o jsonpath="{.data.admin-password}" | base64 -d)

if [ -z "$ADMIN_PASS" ]; then
  echo -e "${RED}Error: Failed to retrieve admin password from Kubernetes secret${NC}"
  exit 1
fi

# Function to get access token
#API referenced from https://www.keycloak.org/docs/latest/server_admin/index.html#_service_accounts
get_token() {
  local token
  token=$(curl -sk -X POST "${KEYCLOAK_URL}/realms/master/protocol/openid-connect/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "username=${ADMIN_USER}" \
    -d "password=${ADMIN_PASS}" \
    -d "grant_type=password" \
    -d "client_id=admin-cli" 2>/dev/null | jq -r '.access_token')
  
  if [ -z "$token" ] || [ "$token" = "null" ]; then
    echo -e "${RED}Error: Failed to get access token from Keycloak${NC}"
    echo "Keycloak URL: ${KEYCLOAK_URL}"
    exit 1
  fi
  
  echo "$token"
}

# Function to get current theme
get_current_theme() {
  local token=$1
  curl -sk "${KEYCLOAK_URL}/admin/realms/master" \
    -H "Authorization: Bearer ${token}" 2>/dev/null | jq -r '.loginTheme // "keycloak"'
}

# Function to update theme
#API referenced from https://www.keycloak.org/docs-api/latest/rest-api/index.html
update_theme() {
  local token=$1
  local theme=$2
  
  local response
  response=$(curl -sk -w "\n%{http_code}" -X PUT "${KEYCLOAK_URL}/admin/realms/master" \
    -H "Authorization: Bearer ${token}" \
    -H "Content-Type: application/json" \
    -d "{\"loginTheme\": \"${theme}\"}" 2>/dev/null)
  
  local http_code=$(echo "$response" | tail -n1)
  
  if [ "$http_code" -eq 204 ] || [ "$http_code" -eq 200 ]; then
    return 0
  else
    echo -e "${RED}Error: Failed to update theme (HTTP ${http_code})${NC}"
    return 1
  fi
}

# Main logic
echo "Connecting to Keycloak at: ${KEYCLOAK_URL}"
TOKEN=$(get_token)

case "$MODE" in
  enable)
    echo "Enabling maintenance mode..."
    if update_theme "$TOKEN" "maintenance"; then
      echo -e "${GREEN}✅ Maintenance mode ENABLED${NC}"
      echo "Users will now see the maintenance page when trying to log in."
    else
      exit 1
    fi
    ;;
  
  disable)
    echo "Disabling maintenance mode..."
    if update_theme "$TOKEN" "keycloak"; then
      echo -e "${GREEN}✅ Maintenance mode DISABLED${NC}"
      echo "Normal login is now restored."
    else
      exit 1
    fi
    ;;
  
  status)
    CURRENT=$(get_current_theme "$TOKEN")
    echo "Current login theme: ${CURRENT}"
    if [ "$CURRENT" = "maintenance" ]; then
      echo -e "${YELLOW}⚠️  Maintenance mode is ACTIVE${NC}"
    else
      echo -e "${GREEN}✅ Normal operation (maintenance mode is OFF)${NC}"
    fi
    ;;
  
  *)
    echo "Usage: $0 {enable|disable|status}"
    echo ""
    echo "Commands:"
    echo "  enable   - Switch to maintenance theme (blocks user logins)"
    echo "  disable  - Switch back to normal theme (restores logins)"
    echo "  status   - Check current theme status"
    echo ""
    echo "Environment variables:"
    echo "  KEYCLOAK_URL - Keycloak base URL (default: https://api.your-domain.com)"
    exit 1
    ;;
esac
