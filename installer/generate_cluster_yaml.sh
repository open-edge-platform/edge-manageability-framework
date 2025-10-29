#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

if [ $# -lt 1 ]; then
    echo "Usage: $0 <deploy-type>"
    echo "Valid options: aws | onprem"
    exit 1
fi

DEPLOY_TYPE=$1
echo "🚀 Starting Cluster Template Generation"
if [ "$DEPLOY_TYPE" = "aws" ]; then
    source "./aws_cluster.env"
    TEMPLATE_FILE="./cluster_aws.tpl"
    OUTPUT_FILE="cluster_aws_${CLUSTER_NAME}.yaml"


elif [ "$DEPLOY_TYPE" = "onprem" ]; then
    # shellcheck disable=SC1091
    source "$(dirname "$0")/${DEPLOY_TYPE}.env"
    # Validate ORCH_INSTALLER_PROFILE
    if [[ "$ORCH_INSTALLER_PROFILE" =~ ^(onprem|onprem-1k|onprem-oxm|onprem-explicit-proxy)$ ]]; then
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
        echo "Valid options: onprem | onprem-1k | onprem-oxm | onprem-explicit-proxy"
        exit 1
    fi

else
    echo "❌ Invalid deploy type: $DEPLOY_TYPE"
    echo "Valid options: aws | onprem"
    exit 1
fi

# --- Default profile exports ---
export PLATFORM_PROFILE='- orch-configs/profiles/enable-platform.yaml'
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

# --- Default environment variables ---

export SRE_TLS_ENABLED="${SRE_TLS_ENABLED:-false}"
export SRE_DEST_CA_CERT="${SRE_DEST_CA_CERT:-}"
export SMTP_SKIP_VERIFY="${SMTP_SKIP_VERIFY:-false}"

# --- Function: Validate IPv4 ---
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

# --- Function: Prompt for IP ---
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


# --- AO/CO profile logic ---
if [ "${DISABLE_CO_PROFILE:-false}" = "true" ] || [ "${DISABLE_AO_PROFILE:-false}" = "true" ]; then
    export AO_PROFILE="#- orch-configs/profiles/enable-app-orch.yaml"
else
    export AO_PROFILE="- orch-configs/profiles/enable-app-orch.yaml"
fi

if [ "${SINGLE_TENANCY_PROFILE:-false}" = "true" ]; then
    export SINGLE_TENANCY_PROFILE="- orch-configs/profiles/enable-singleTenancy.yaml"
else
    export SINGLE_TENANCY_PROFILE="#- orch-configs/profiles/enable-singleTenancy.yaml"
fi

if [ "${DISABLE_CO_PROFILE:-false}" = "true" ]; then
    export CO_PROFILE="#- orch-configs/profiles/enable-cluster-orch.yaml"
    export AO_PROFILE="#- orch-configs/profiles/enable-app-orch.yaml"
else
    export CO_PROFILE="- orch-configs/profiles/enable-cluster-orch.yaml"
fi

# Check if EXPLICIT_PROXY is Enabled
if [ "${EXPLICIT_PROXY:-false}" = "true" ]; then
    export EXPLICIT_PROXY_PROFILE="- orch-configs/profiles/enable-explicit-proxy.yaml"
else
    export EXPLICIT_PROXY_PROFILE="#- orch-configs/profiles/enable-explicit-proxy.yaml"
fi

if [ "$DEPLOY_TYPE" = "onprem" ]; then

	# --- IP prompts ---
	prompt_for_ip "ARGO_IP" "Argo IP"
	prompt_for_ip "TRAEFIK_IP" "Traefik IP"
	prompt_for_ip "NGINX_IP" "Nginx IP"

	echo
	echo "✅ Using the following valid IPs:"
	echo "   ArgoIP:     $ARGO_IP"
	echo "   TraefikIP:  $TRAEFIK_IP"
	echo "   NginxIP:    $NGINX_IP"
	
	# --- O11Y_PROFILE disable check ---
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
        onprem)
            echo "📦 Profile: Standard On-Prem Deployment"
            export ONPREM_PROFILE='- orch-configs/profiles/enable-onprem.yaml'
            export PROFILE_FILE_NAME='- orch-configs/profiles/profile-onprem.yaml'
            ;;
        onprem-oxm)
            echo "📦 Profile: On-Prem with OXM Integration"
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

elif [ "$DEPLOY_TYPE" = "aws" ]; then

	# --- O11Y_PROFILE disable check ---
	export CLUSTER_SCALE_PROFILE=$(grep -oP '^# Profile: "\K[^"]+' ~/pod-configs/SAVEME/${AWS_ACCOUNT}-${CLUSTER_NAME}-profile.tfvar)
	if [ "${DISABLE_O11Y_PROFILE:-false}" = "true" ]; then
	    export O11Y_ENABLE_PROFILE="#- orch-configs/profiles/enable-o11y.yaml"
		export O11Y_PROFILE="#- orch-configs/profiles/o11y-release.yaml"
	else		
	    export O11Y_ENABLE_PROFILE="- orch-configs/profiles/enable-o11y.yaml"
		export O11Y_PROFILE="- orch-configs/profiles/o11y-release.yaml"
		if [[ "$CLUSTER_SCALE_PROFILE" == "500en" || "$CLUSTER_SCALE_PROFILE" == "1ken" || "$CLUSTER_SCALE_PROFILE" == "10ken" ]]; then
		  export O11Y_PROFILE="- orch-configs/profiles/o11y-release-large.yaml"
		fi
	fi

    # SRE Profile
    if [ -n "${SRE_BASIC_AUTH_USERNAME:-}" ] || [ -n "${SRE_BASIC_AUTH_PASSWORD:-}" ] || [ -n "${SRE_DESTINATION_SECRET_URL:-}" ] || [ -n "${SRE_DESTINATION_CA_SECRET:-}" ]; then
        export SRE_PROFILE="- orch-configs/profiles/enable-sre.yaml"
    else
        export SRE_PROFILE="#- orch-configs/profiles/enable-sre.yaml"
    fi

    # Email Profile
    if [ -z "${SMTP_URL:-}" ]; then
        export EMAIL_PROFILE="#- orch-configs/profiles/alerting-emails.yaml"
    else
        export EMAIL_PROFILE="- orch-configs/profiles/alerting-emails.yaml"
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

    # O11Y Profile override for large clusters
    if [[ "$CLUSTER_SCALE_PROFILE" == "500en" || "$CLUSTER_SCALE_PROFILE" == "1ken" || "$CLUSTER_SCALE_PROFILE" == "10ken" ]]; then
        export O11Y_PROFILE="- orch-configs/profiles/o11y-release-large.yaml"
    fi
fi

# --- Generate cluster YAML ---
echo "🔧 Generating cluster config..."
envsubst < "$TEMPLATE_FILE" \
    | sed -E '/^[[:space:]]*#/d; /^[[:space:]]*#!/d; /^[[:space:]]*$/d' \
    > "$OUTPUT_FILE"

# --- Post-processing for onprem-1k ---
if [ "${CLUSTER_SCALE_PROFILE}" = "1ken" ]; then
    echo "ℹ️  Using ONPREM-1K deployment profile (EdgeInfra + O11Y optional)"
    yq -i '.argo.postgresql.resourcesPreset |= "large"' "$OUTPUT_FILE"
fi

yq -i ".argo.o11y.sre.tls.enabled |= ${TLS_ENABLED}" "$OUTPUT_FILE"
yq -i ".argo.o11y.sre.tls.caSecretEnabled |= ${CA_SECRET_ENABLED}" "$OUTPUT_FILE"
yq -i ".argo.o11y.alertingMonitor.smtp.insecureSkipVerify |= ${SMTP_SKIP_VERIFY}" "$OUTPUT_FILE"


update_proxy() {
    local yaml_key=$1
    local var_name=$2
    local value=${!var_name:-}  # Get the env var or empty

    # If unset or empty, default to empty string
    if [ -z "$value" ]; then
        value=""
    fi

    # Export TMP_VALUE for strenv
    export TMP_VALUE="$value"
    echo "🛠️ Setting .argo.proxy.${yaml_key} = \"$value\""
    yq eval -i ".argo.proxy.${yaml_key} = strenv(TMP_VALUE)" "$OUTPUT_FILE"
    unset TMP_VALUE
}

# --- Update all proxy variables ---
update_proxy "httpProxy" "ORCH_HTTP_PROXY"
update_proxy "httpsProxy" "ORCH_HTTPS_PROXY"
update_proxy "noProxy" "ORCH_NO_PROXY"
update_proxy "enHttpProxy" "EN_HTTP_PROXY"
update_proxy "enHttpsProxy" "EN_HTTPS_PROXY"
update_proxy "enFtpProxy" "EN_FTP_PROXY"
update_proxy "enSocksProxy" "EN_SOCKS_PROXY"
update_proxy "enNoProxy" "EN_NO_PROXY"

# --- Review generated file if PROCEED is set and not 'yes' ---
if [[ -n "${PROCEED}" && "${PROCEED}" != "yes" ]]; then
    echo
    echo "=============================================================================="
    echo "Please review the cluster settings in the generated configuration and make any necessary updates."
    echo
    echo "Press any key to open your editor..."
    echo "=============================================================================="
    echo
    read -n 1 -s  # wait for a single key press silently
    "${EDITOR:-vi}" "$OUTPUT_FILE"
fi

echo "✅ File generated: $OUTPUT_FILE"
cat "$OUTPUT_FILE"

chmod 644 "$OUTPUT_FILE"
