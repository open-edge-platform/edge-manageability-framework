#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# -----------------------------------------------------------------------------
# Cluster Template Generation Script
# -----------------------------------------------------------------------------

if [ $# -lt 1 ]; then
    echo "Usage: $0 <deploy-type>"
    echo "Valid options: aws | onprem"
    exit 1
fi

DEPLOY_TYPE=$1
echo "🚀 Starting Cluster Template Generation"

# -----------------------------------------------------------------------------
# Environment setup based on deployment type
# -----------------------------------------------------------------------------
if [ "$DEPLOY_TYPE" = "aws" ]; then
    source "$HOME/.env"
    TEMPLATE_FILE="./cluster_aws.tpl"
    OUTPUT_FILE="${CLUSTER_NAME}.yaml"

elif [ "$DEPLOY_TYPE" = "onprem" ]; then
    # shellcheck disable=SC1091
    source "$(dirname "$0")/${DEPLOY_TYPE}.env"

    # Validate ORCH_INSTALLER_PROFILE
    if [[ "$ORCH_INSTALLER_PROFILE" =~ ^(onprem|onprem-oxm|onprem-vpro)$ ]]; then
        TEMPLATE_FILE="./cluster_onprem.tpl"
        OUTPUT_FILE="${ORCH_INSTALLER_PROFILE}.yaml"

        echo "🔧 Using ORCH_INSTALLER_PROFILE=${ORCH_INSTALLER_PROFILE}"
        export O11Y_ENABLE_PROFILE='- orch-configs/profiles/enable-o11y.yaml'
        export O11Y_PROFILE='- orch-configs/profiles/o11y-onprem.yaml'
        export ONPREM_PROFILE='- orch-configs/profiles/enable-onprem.yaml'
        export CLUSTER_NAME="${CLUSTER_NAME:-onprem}"
        export CLUSTER_DOMAIN="${CLUSTER_DOMAIN:-cluster.onprem}"
    else
        echo "❌ Invalid ORCH_INSTALLER_PROFILE: ${ORCH_INSTALLER_PROFILE}"
        echo "Valid options: onprem | onprem-oxm"
        exit 1
    fi

else
    echo "❌ Invalid deploy type: $DEPLOY_TYPE"
    echo "Valid options: aws | onprem"
    exit 1
fi


# -----------------------------------------------------------------------------
# Default profiles
# -----------------------------------------------------------------------------
export PLATFORM_PROFILE='- orch-configs/profiles/enable-platform.yaml'
export KYVERNO_PROFILE='- orch-configs/profiles/enable-kyverno.yaml'
export EDGEINFRA_PROFILE='- orch-configs/profiles/enable-edgeinfra.yaml'
export UI_PROFILE='- orch-configs/profiles/enable-full-ui.yaml'
export SRE_PROFILE='- orch-configs/profiles/enable-sre.yaml'
export PROXY_NONE_PROFILE='- orch-configs/profiles/proxy-none.yaml'
export PROFILE_FILE_NAME='- orch-configs/profiles/profile-onprem.yaml'
export PROFILE_FILE_NAME_EXT=''
export EMAIL_PROFILE='- orch-configs/profiles/alerting-emails.yaml'
export ARTIFACT_RS_PROFILE='- orch-configs/profiles/artifact-rs-production-noauth.yaml'
export OSRM_MANUAL_PROFILE='- orch-configs/profiles/enable-osrm-manual-mode.yaml'
export RESOURCE_DEFAULT_PROFILE='- orch-configs/profiles/resource-default.yaml'


if [[ "$ORCH_INSTALLER_PROFILE" == "onprem-vpro" ]]; then
  export DISABLE_AO_PROFILE=true
  export DISABLE_CO_PROFILE=true
  export DISABLE_O11Y_PROFILE=true
  export DISABLE_KYVERNO_PROFILE=true
  export DISABLE_UI_PROFILE=true
  export SRE_TLS_ENABLED=false
  export SINGLE_TENANCY_PROFILE=false
  export EMAIL_PROFILE='#- orch-configs/profiles/alerting-emails.yaml'
  export PLATFORM_PROFILE='- orch-configs/profiles/enable-platform.yaml'
fi

# -----------------------------------------------------------------------------
# Default environment variables
# -----------------------------------------------------------------------------
export SRE_TLS_ENABLED="${SRE_TLS_ENABLED:-false}"
export SRE_DEST_CA_CERT="${SRE_DEST_CA_CERT:-}"

# -----------------------------------------------------------------------------
# Function: Validate IPv4
# -----------------------------------------------------------------------------
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

# -----------------------------------------------------------------------------
# Function: Prompt for IP input
# -----------------------------------------------------------------------------
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

# -----------------------------------------------------------------------------
# TLS and CA secret configuration
# -----------------------------------------------------------------------------
if [[ "$SRE_TLS_ENABLED" == "true" ]]; then
    TLS_ENABLED=true
    CA_SECRET_ENABLED=false
    [[ -n "$SRE_DEST_CA_CERT" ]] && CA_SECRET_ENABLED=true
else
    TLS_ENABLED=false
    CA_SECRET_ENABLED=false
fi

# -----------------------------------------------------------------------------
# AO / CO profile logic
# -----------------------------------------------------------------------------
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

if [ "${DISABLE_KYVERNO_PROFILE:-false}" = "true" ]; then
    export KYVERNO_PROFILE="#- orch-configs/profiles/enable-kyverno.yaml"
else
    export KYVERNO_PROFILE="- orch-configs/profiles/enable-kyverno.yaml"
fi

if [ "${SINGLE_TENANCY_PROFILE:-false}" = "true" ]; then
    export SINGLE_TENANCY_PROFILE="- orch-configs/profiles/enable-singleTenancy.yaml"
else
    export SINGLE_TENANCY_PROFILE="#- orch-configs/profiles/enable-singleTenancy.yaml"
fi

if [ "${DISABLE_UI_PROFILE:-false}" = "true" ]; then
    export UI_PROFILE='#- orch-configs/profiles/enable-full-ui.yaml'
else
    export UI_PROFILE='- orch-configs/profiles/enable-full-ui.yaml'
fi

# -----------------------------------------------------------------------------
# Explicit proxy configuration
# -----------------------------------------------------------------------------
if [ "${ENABLE_EXPLICIT_PROXY:-false}" = "true" ]; then
    export EXPLICIT_PROXY_PROFILE="- orch-configs/profiles/enable-explicit-proxy.yaml"
else
    export EXPLICIT_PROXY_PROFILE="#- orch-configs/profiles/enable-explicit-proxy.yaml"
fi

# -----------------------------------------------------------------------------
# On-prem specific logic
# -----------------------------------------------------------------------------
if [ "$DEPLOY_TYPE" = "onprem" ]; then
    # Prompt for required IPs
    prompt_for_ip "ARGO_IP" "Argo IP"
    prompt_for_ip "TRAEFIK_IP" "Traefik IP"
    prompt_for_ip "NGINX_IP" "Nginx IP"

    echo
    echo "✅ Using the following valid IPs:"
    echo "   ArgoIP:     $ARGO_IP"
    echo "   TraefikIP:  $TRAEFIK_IP"
    echo "   NginxIP:    $NGINX_IP"
    echo

    # O11Y disable check
    if [ "${DISABLE_O11Y_PROFILE:-false}" = "true" ]; then
        export O11Y_ENABLE_PROFILE="#- orch-configs/profiles/enable-o11y.yaml"
        export O11Y_PROFILE="#- orch-configs/profiles/o11y-onprem.yaml"
    else
        export O11Y_ENABLE_PROFILE="- orch-configs/profiles/enable-o11y.yaml"
        export O11Y_PROFILE="- orch-configs/profiles/o11y-onprem.yaml"
    fi

    if [ "${CLUSTER_SCALE_PROFILE}" = "1ken" ]; then
        export EDGEINFRA_PROFILE='- orch-configs/profiles/enable-edgeinfra-1k.yaml'
        if [ "${DISABLE_O11Y_PROFILE:-false}" = "true" ]; then
            export O11Y_PROFILE="#- orch-configs/profiles/o11y-onprem-1k.yaml"
        else
            export O11Y_PROFILE="- orch-configs/profiles/o11y-onprem-1k.yaml"
        fi
    fi

    case "${ORCH_INSTALLER_PROFILE}" in
        onprem|onprem-vpro)
            echo "📦 Profile: Standard On-Prem Deployment"
            export PROFILE_FILE_NAME='- orch-configs/profiles/profile-onprem.yaml'
            ;;
        onprem-oxm)
            echo "📦 Profile: On-Prem with OXM Integration"            
            if [ -z "$OXM_PXE_SERVER_INT" ] || [ -z "$OXM_PXE_SERVER_IP" ] || [ -z "$OXM_PXE_SERVER_SUBNET" ]; then
              echo "❌ Error: Required environment variables not set!"
              echo "Please export:"
              echo "  OXM_PXE_SERVER_INT"
              echo "  OXM_PXE_SERVER_IP"
              echo "  OXM_PXE_SERVER_SUBNET"
              exit 1
            fi            
            export PROFILE_FILE_NAME_EXT='- orch-configs/profiles/profile-oxm.yaml'
            export EXPLICIT_PROXY_PROFILE='- orch-configs/profiles/enable-explicit-proxy.yaml'
            export O11Y_ENABLE_PROFILE="#- orch-configs/profiles/enable-o11y.yaml"
            export O11Y_PROFILE="#- orch-configs/profiles/o11y-onprem.yaml"
            export CO_PROFILE="#- orch-configs/profiles/enable-cluster-orch.yaml"
            export AO_PROFILE="#- orch-configs/profiles/enable-app-orch.yaml"            
            ;;
        *)
            echo "❌ Invalid ORCH_INSTALLER_PROFILE: ${ORCH_INSTALLER_PROFILE}"
            echo "Valid on-prem profiles: onprem | onprem-1k | onprem-oxm | onprem-explicit-proxy"
            exit 1
            ;;
    esac

# -----------------------------------------------------------------------------
# AWS specific logic
# -----------------------------------------------------------------------------
elif [ "$DEPLOY_TYPE" = "aws" ]; then
    export CLUSTER_SCALE_PROFILE=$(grep -oP '^# Profile: "\K[^"]+' ~/pod-configs/SAVEME/${AWS_ACCOUNT}-${CLUSTER_NAME}-profile.tfvar)

    # O11Y Profile
    if [ "${DISABLE_O11Y_PROFILE:-false}" = "true" ]; then
        export O11Y_ENABLE_PROFILE="#- orch-configs/profiles/enable-o11y.yaml"
        export O11Y_PROFILE="#- orch-configs/profiles/o11y-release.yaml"
    else
        export O11Y_ENABLE_PROFILE="- orch-configs/profiles/enable-o11y.yaml"
        export O11Y_PROFILE="- orch-configs/profiles/o11y-release.yaml"
        if [[ "$CLUSTER_SCALE_PROFILE" =~ ^(500en|1ken|10ken)$ ]]; then
            export O11Y_PROFILE="- orch-configs/profiles/o11y-release-large.yaml"
        fi
    fi

    # SRE Profile
    if [ -n "${SRE_BASIC_AUTH_USERNAME:-}" ] || [ -n "${SRE_BASIC_AUTH_PASSWORD:-}" ] || \
       [ -n "${SRE_DESTINATION_SECRET_URL:-}" ] || [ -n "${SRE_DESTINATION_CA_SECRET:-}" ]; then
        export SRE_PROFILE="- orch-configs/profiles/enable-sre.yaml"
    else
        export SRE_PROFILE="#- orch-configs/profiles/enable-sre.yaml"
    fi

    # Email Profile
    echo "ℹ️ SMTP_URL value is: ${SMTP_URL}"
    if [ -z "${SMTP_URL:-}" ]; then
        export EMAIL_PROFILE="#- orch-configs/profiles/alerting-emails.yaml"
    else
        if [ "${SMTP_DEV_MODE:-false}" = "true" ]; then
            export EMAIL_PROFILE="- orch-configs/profiles/alerting-emails-dev.yaml"
        else
        export EMAIL_PROFILE="- orch-configs/profiles/alerting-emails.yaml"
    fi
    fi

    # AutoCert Profile
    if [ -z "${AUTO_CERT:-}" ]; then
        export AUTOCERT_PROFILE="#- orch-configs/profiles/profile-autocert.yaml"
    else
        export AUTOCERT_PROFILE="- orch-configs/profiles/profile-autocert.yaml"
    fi

    # AWS Production Profile
    if [ "${DISABLE_AWS_PROD_PROFILE:-false}" = "true" ]; then
        export AWS_PROD_PROFILE="#- orch-configs/profiles/profile-aws-production.yaml"
    else
        export AWS_PROD_PROFILE="- orch-configs/profiles/profile-aws-production.yaml"
    fi
fi

# -----------------------------------------------------------------------------
# Generate Cluster YAML
# -----------------------------------------------------------------------------
echo "🔧 Generating cluster config..."
envsubst < "$TEMPLATE_FILE" \
    | sed -E '/^[[:space:]]*#/d; /^[[:space:]]*#!/d; /^[[:space:]]*$/d' \
    > "$OUTPUT_FILE"

# Onprem 1k post-processing
if [ "${CLUSTER_SCALE_PROFILE}" = "1ken" ]; then
    echo "ℹ️ Using ONPREM-1K deployment profile (EdgeInfra + O11Y optional)"
    yq -i '.argo.postgresql.resourcesPreset |= "large"' "$OUTPUT_FILE"
fi

# Update YAML parameters
yq -i ".argo.o11y.sre.tls.enabled |= ${TLS_ENABLED}" "$OUTPUT_FILE"
yq -i ".argo.o11y.sre.tls.caSecretEnabled |= ${CA_SECRET_ENABLED}" "$OUTPUT_FILE"

if [ -n "${SMTP_SKIP_VERIFY}" ]; then
  yq -i ".argo.o11y.alertingMonitor.smtp.insecureSkipVerify |= ${SMTP_SKIP_VERIFY}" "$OUTPUT_FILE"
fi

if [ "$ORCH_INSTALLER_PROFILE" = "onprem-oxm" ]; then
  yq -i "
  .argo.infra-onboarding.pxe-server.interface = \"${OXM_PXE_SERVER_INT}\" |
  .argo.infra-onboarding.pxe-server.bootServerIP = \"${OXM_PXE_SERVER_IP}\" |
  .argo.infra-onboarding.pxe-server.subnetAddress = \"${OXM_PXE_SERVER_SUBNET}\" |
  .postCustomTemplateOverwrite.traefik.deployment.podAnnotations.\"traffic.sidecar.istio.io/excludeInboundPorts\" = \"8000,8443,8080,4433,9000\"
" "$OUTPUT_FILE"
fi

# overwrite infra-onboarding configuration
if [ "${DISABLE_CO_PROFILE:-false}" = "true" ]; then
    yq -i '.argo.infra-onboarding.disableCoProfile = true' "$OUTPUT_FILE"
fi

if [ "${DISABLE_O11Y_PROFILE:-false}" = "true" ]; then
    yq -i '.argo.infra-onboarding.disableO11yProfile = true' "$OUTPUT_FILE"
fi

if [ "${ONPREM_UPGRADE_SYNC:-false}" = "true" ]; then
    yq -i '
  .argo.metadata.annotations."argocd.argoproj.io/hook" = "PostSync" |
  .argo.metadata.annotations."argocd.argoproj.io/hook-delete-policy" = "BeforeHookCreation"
' "$OUTPUT_FILE"
fi


# -----------------------------------------------------------------------------
# Proxy variable updates
# -----------------------------------------------------------------------------
update_proxy() {
    local yaml_key=$1
    local var_name=$2
    local value=${!var_name:-}

    # Skip if variable is undefined or empty
    if [ -z "$value" ]; then
        echo "⚠️ Skipping ${yaml_key} (env var ${var_name} is undefined or empty)"
        return
    fi

    export TMP_VALUE="$value"
    
    # Special handling for gitProxy - update at .argo.git location
    if [ "$yaml_key" = "gitProxy" ]; then
        echo "🛠️ Setting .argo.git.${yaml_key} = \"$value\""
        yq eval -i ".argo.git.${yaml_key} = strenv(TMP_VALUE)" "$OUTPUT_FILE"
    else
        echo "🛠️ Setting .argo.proxy.${yaml_key} = \"$value\""
        yq eval -i ".argo.proxy.${yaml_key} = strenv(TMP_VALUE)" "$OUTPUT_FILE"
    fi
    
    unset TMP_VALUE
}

update_proxy "httpProxy" "ORCH_HTTP_PROXY"
update_proxy "httpsProxy" "ORCH_HTTPS_PROXY"
update_proxy "noProxy" "ORCH_NO_PROXY"
update_proxy "enHttpProxy" "EN_HTTP_PROXY"
update_proxy "enHttpsProxy" "EN_HTTPS_PROXY"
update_proxy "enFtpProxy" "EN_FTP_PROXY"
update_proxy "enSocksProxy" "EN_SOCKS_PROXY"
update_proxy "enNoProxy" "EN_NO_PROXY"
update_proxy "gitProxy" "GIT_PROXY"

# -----------------------------------------------------------------------------
# Manual review (if PROCEED != yes)
# -----------------------------------------------------------------------------
if [[ -n "${PROCEED}" && "${PROCEED}" != "yes" ]]; then
    echo
    echo "=============================================================================="
    echo "Please review the cluster settings in the generated configuration and make any necessary updates."
    echo
    echo "Press any key to open your editor..."
    echo "=============================================================================="
    echo
    read -n 1 -s
    "${EDITOR:-vi}" "$OUTPUT_FILE"
fi

echo "✅ File generated: $OUTPUT_FILE"
cat "$OUTPUT_FILE"
chmod 644 "$OUTPUT_FILE"
