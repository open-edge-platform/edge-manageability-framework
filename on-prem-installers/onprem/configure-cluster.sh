#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# --- Check if ORCH_INSTALLER_PROFILE is set ---
case "$ORCH_INSTALLER_PROFILE" in
  onprem|onprem-1k|onprem-oxm|onprem-explicit-proxy)
    ;;  # ✅ Valid profiles — do nothing, execution continues
  *)
    echo "❌ Invalid ORCH_INSTALLER_PROFILE: ${ORCH_INSTALLER_PROFILE}"
    echo "Valid options: onprem | onprem-1k | onprem-oxm | onprem-explicit-proxy"
    exit 1  # ❌ Stop script on invalid value
    ;;
esac

TEMPLATE_FILE=./onprem_cluster.tpl
OUTPUT_FILE=cluster_${ORCH_INSTALLER_PROFILE}.yaml

echo "🔧 Using ORCH_INSTALLER_PROFILE=${ORCH_INSTALLER_PROFILE}"

# --- Default profile exports ---
export PLATFORM_PROFILE='- orch-configs/profiles/enable-platform.yaml'
export O11Y_PROFILE='- orch-configs/profiles/enable-o11y.yaml'
export O11Y_ONPREM_PROFILE='- orch-configs/profiles/o11y-onprem.yaml'
export KYVERNO_PROFILE='- orch-configs/profiles/enable-kyverno.yaml'
export EDGEINFRA_PROFILE='- orch-configs/profiles/enable-edgeinfra.yaml'
export FULL_UI_PROFILE='- orch-configs/profiles/enable-full-ui.yaml'
export ONPREM_PROFILE='- orch-configs/profiles/enable-onprem.yaml'
export SRE_PROFILE='- orch-configs/profiles/enable-sre.yaml'
export PROXY_NONE_PROFILE='- orch-configs/profiles/proxy-none.yaml'
export PROFILE_FILE_NAME='- orch-configs/profiles/profile-onprem.yaml'
export PROFILE_FILE_NAME_EXT=''
export EMAIL_PROFILE='- orch-configs/profiles/alerting-emails.yaml'
export ARTIFACT_RS_PROFILE='- orch-configs/profiles/artifact-rs-production-noauth.yaml'
export OSRM_MANUAL_PROFILE='- orch-configs/profiles/enable-osrm-manual-mode.yaml'
export RESOURCE_DEFAULT_PROFILE='- orch-configs/profiles/resource-default.yaml'
export ORCH_INSTALLER_FILE_NAME="- orch-configs/clusters/cluster_${ORCH_INSTALLER_PROFILE}.yaml"

# --- AO/CO profile conditions ---
if [ "${DISABLE_CO_PROFILE:-false}" = "true" ] || [ "${DISABLE_AO_PROFILE:-false}" = "true" ]; then
  export AO_PROFILE="#- orch-configs/profiles/enable-app-orch.yaml"
else
  export AO_PROFILE="- orch-configs/profiles/enable-app-orch.yaml"
fi

if [ "${DISABLE_CO_PROFILE:-false}" = "true" ]; then
  export CO_PROFILE="#- orch-configs/profiles/enable-cluster-orch.yaml"
  export AO_PROFILE="#- orch-configs/profiles/enable-app-orch.yaml"
else
  export CO_PROFILE="- orch-configs/profiles/enable-cluster-orch.yaml"
fi

# --- O11Y_PROFILE disable check ---
if [ "${DISABLE_O11Y_PROFILE:-false}" = "true" ]; then
  export O11Y_PROFILE="#- orch-configs/profiles/enable-o11y.yaml"
  export O11Y_ONPREM_PROFILE="#- orch-configs/profiles/o11y-onprem.yaml"
else
  export O11Y_PROFILE="- orch-configs/profiles/enable-o11y.yaml"
  export O11Y_ONPREM_PROFILE="- orch-configs/profiles/o11y-onprem.yaml"
fi

# --- Default values for optional environment variables ---
export PROJECT="${PROJECT:-onprem}"
export NAMESPACE="${NAMESPACE:-onprem}"
export CLUSTER_NAME="${CLUSTER_NAME:-onprem}"
export CLUSTER_DOMAIN="${CLUSTER_DOMAIN:-cluster.onprem}"

export SRE_TLS_ENABLED="${SRE_TLS_ENABLED:-false}"
export SRE_DEST_CA_CERT="${SRE_DEST_CA_CERT:-}"
export SMTP_SKIP_VERIFY="${SMTP_SKIP_VERIFY:-false}"

# --- Function: Validate IPv4 format ---
is_valid_ip() {
  local ip=$1
  if [[ $ip =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
    IFS='.' read -r -a octets <<< "$ip"
    for octet in "${octets[@]}"; do
      if (( octet < 0 || octet > 255 )); then
        return 1
      fi
    done
    return 0
  else
    return 1
  fi
}

# --- Function: Prompt for IP if not set or invalid ---
prompt_for_ip() {
  local var_name=$1
  local prompt_msg=$2
  local value="${!var_name}"

  while true; do
    if [ -z "$value" ]; then
      read -rp "Enter ${prompt_msg}: " value
    fi

    if is_valid_ip "$value"; then
      export "$var_name"="$value"
      break
    else
      echo "❌ Invalid IP address: $value"
      value=""
      unset "$var_name"
      read -rp "Do you want to retry? (y/n): " choice
      case "$choice" in
        [Yy]*) continue ;;
        [Nn]*) echo "Exiting."; exit 1 ;;
        *) echo "Please answer y or n." ;;
      esac
    fi
  done
}

# --- TLS and CA Secret configuration ---
if [[ "$SRE_TLS_ENABLED" == "true" ]]; then
  TLS_ENABLED=true
  CA_SECRET_ENABLED=false
  [[ -n "$SRE_DEST_CA_CERT" ]] && CA_SECRET_ENABLED=true
else
  TLS_ENABLED=false
  CA_SECRET_ENABLED=false
fi

# --- Switch-case for ORCH_INSTALLER_PROFILE ---
case "${ORCH_INSTALLER_PROFILE}" in
  onprem)
    echo "📦 Profile: Standard On-Prem Deployment"
    export ONPREM_PROFILE='- orch-configs/profiles/enable-onprem.yaml'
    export PROFILE_FILE_NAME='- orch-configs/profiles/profile-onprem.yaml'
    ;;

  onprem-1k)
    echo "📦 Profile: On-Prem 1K Deployment"
    export EDGEINFRA_PROFILE='- orch-configs/profiles/enable-edgeinfra-1k.yaml'
    if [ "${DISABLE_O11Y_PROFILE:-false}" = "true" ]; then
      export O11Y_ONPREM_PROFILE="#- orch-configs/profiles/o11y-onprem-1k.yaml"
    else
      export O11Y_ONPREM_PROFILE="- orch-configs/profiles/o11y-onprem-1k.yaml"
    fi
    ;;

  onprem-oxm)
    echo "📦 Profile: On-Prem with OXM Integration"
    export PROFILE_FILE_NAME_EXT='- orch-configs/profiles/profile-oxm.yaml'
    export EXPLICIT_PROXY_PROFILE='- orch-configs/profiles/enable-explicit-proxy.yaml'
    export O11Y_PROFILE="#- orch-configs/profiles/enable-o11y.yaml"
    export O11Y_ONPREM_PROFILE="#- orch-configs/profiles/o11y-onprem.yaml"
    export CO_PROFILE="#- orch-configs/profiles/enable-cluster-orch.yaml"
    export AO_PROFILE="#- orch-configs/profiles/enable-app-orch.yaml"
    ;;

  onprem-explicit-proxy)
    echo "📦 Profile: On-Prem with Explicit Proxy Configuration"
    export EXPLICIT_PROXY_PROFILE='- orch-configs/profiles/enable-explicit-proxy.yaml'
    ;;

  *)
    echo "❌ Invalid ORCH_INSTALLER_PROFILE: ${ORCH_INSTALLER_PROFILE}"
    echo "Valid options: onprem | onprem-1k | onprem-oxm | onprem-explicit-proxy"
    exit 1
    ;;
esac

# --- IP prompts ---
prompt_for_ip "ARGO_IP" "Argo IP"
prompt_for_ip "TRAEFIK_IP" "Traefik IP"
prompt_for_ip "NGINX_IP" "Nginx IP"

echo
echo "✅ Using the following valid IPs:"
echo "   ArgoIP:     $ARGO_IP"
echo "   TraefikIP:  $TRAEFIK_IP"
echo "   NginxIP:    $NGINX_IP"

# --- Generate cluster YAML ---
echo "🔧 Generating cluster config..."

envsubst < "$TEMPLATE_FILE" \
  | sed -E '/^[[:space:]]*#/d; /^[[:space:]]*#!/d; /^[[:space:]]*$/d' \
  > "$OUTPUT_FILE"

if [ "${ORCH_INSTALLER_PROFILE}" = "onprem-1k" ]; then
  echo "ℹ️  Using ONPREM-1K deployment profile (EdgeInfra + O11Y optional)"
  yq -i '.argo.postgresql.resourcesPreset |= "large"' "$OUTPUT_FILE"
fi

yq -i ".argo.o11y.sre.tls.enabled |= ${TLS_ENABLED}" "$OUTPUT_FILE"
yq -i ".argo.o11y.sre.tls.caSecretEnabled |= ${CA_SECRET_ENABLED}" "$OUTPUT_FILE"
yq -i ".argo.o11y.alertingMonitor.smtp.insecureSkipVerify |= ${SMTP_SKIP_VERIFY}" "$OUTPUT_FILE"

echo "✅ File generated: $OUTPUT_FILE"
