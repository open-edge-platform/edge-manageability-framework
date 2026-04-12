#!/bin/bash
# SPDX-FileCopyrightText: (C) 2026 Intel Corporation
# SPDX-License-Identifier: Apache-2.0
#
# Helmfile variant of dnsmasq-setup.sh
# All DNS entries point directly to the host IP (no LoadBalancer lookup).
# ArgoCD entry removed — not used in helmfile deployment.
set -x

# Check if at least action argument is provided
if [ -z "$2" ] && [ -z "$1" ]; then
    echo "Usage: $0 [cluster.fqdn] {setup|config}"
    exit 1
fi

# If only one arg, treat it as action and use default FQDN
if [ -z "$2" ]; then
    CLUSTER_FQDN="cluster.onprem"
    ACTION="$1"
else
    CLUSTER_FQDN="$1"
    ACTION="$2"
fi

# Get interface name with 10 network IP
interface_name=$(ip -o -4 addr show | awk '$4 ~ /^10\./ {print $2}')

# Check if any interfaces were found
if [ -n "$interface_name" ]; then
    echo "Interfaces with IP addresses starting with 10.:"
    echo "$interface_name"
else
    echo "No interfaces found with IP addresses starting with 10."
    ip -o -4 addr show
    exit 1
fi

# Get the IP address of the specified interface
ip_address=$(ip -4 addr show "$interface_name" | grep -oP '(?<=inet\s)\d+(\.\d+){3}')
if [ -z "$ip_address" ]; then
    echo "No IP address found for $interface_name. Exiting."
    exit 1
fi

echo "Using host IP: $ip_address for all DNS entries"

function setup_dns() {
    sudo apt update -y
    resolvectl status
    dns_server_ip=$(resolvectl status | awk '/Current DNS Server/ {print $4}')
    sudo apt install -y dnsmasq
    sudo systemctl disable systemd-resolved
    sudo systemctl stop systemd-resolved

    # Backup the original dnsmasq configuration file
    echo "Backing up the original dnsmasq configuration..."
    sudo cp /etc/dnsmasq.conf /etc/dnsmasq.conf.bak

    # Get the current hostname
    current_hostname=$(hostname)
    echo "Adding hostname '$current_hostname' to /etc/hosts..."
    echo "$ip_address $current_hostname" | sudo tee -a /etc/hosts > /dev/null

    # Unlink and recreate /etc/resolv.conf
    echo "Configuring /etc/resolv.conf..."
    sudo unlink /etc/resolv.conf
    cat <<EOL | sudo tee /etc/resolv.conf
nameserver 127.0.0.1
options trust-ad
EOL

    # Configure dnsmasq
    echo "Configuring dnsmasq..."
    cat <<EOL | sudo tee /etc/dnsmasq.conf
interface=$interface_name
bind-interfaces
log-queries
log-facility=/var/log/dnsmasq.log
dhcp-option=interface:$interface_name,option:dns-server,$ip_address
server=$ip_address
server=$dns_server_ip
server=8.8.8.8
EOL
}

function update_host_dns() {
    # All entries hardcoded to host IP — no LoadBalancer lookup needed
    cat <<EOL | sudo tee /etc/dnsmasq.d/cluster-hosts-dns.conf
address=/tinkerbell-haproxy.$CLUSTER_FQDN/$ip_address
address=/$CLUSTER_FQDN/$ip_address
address=/alerting-monitor.$CLUSTER_FQDN/$ip_address
address=/api.$CLUSTER_FQDN/$ip_address
address=/app-orch.$CLUSTER_FQDN/$ip_address
address=/app-service-proxy.$CLUSTER_FQDN/$ip_address
address=/cluster-orch-edge-node.$CLUSTER_FQDN/$ip_address
address=/cluster-orch-node.$CLUSTER_FQDN/$ip_address
address=/cluster-orch.$CLUSTER_FQDN/$ip_address
address=/connect-gateway.$CLUSTER_FQDN/$ip_address
address=/fleet.$CLUSTER_FQDN/$ip_address
address=/infra-node.$CLUSTER_FQDN/$ip_address
address=/infra.$CLUSTER_FQDN/$ip_address
address=/keycloak.$CLUSTER_FQDN/$ip_address
address=/license-node.$CLUSTER_FQDN/$ip_address
address=/log-query.$CLUSTER_FQDN/$ip_address
address=/logs-node.$CLUSTER_FQDN/$ip_address
address=/metadata.$CLUSTER_FQDN/$ip_address
address=/metrics-node.$CLUSTER_FQDN/$ip_address
address=/observability-admin.$CLUSTER_FQDN/$ip_address
address=/observability-ui.$CLUSTER_FQDN/$ip_address
address=/onboarding-node.$CLUSTER_FQDN/$ip_address
address=/onboarding-stream.$CLUSTER_FQDN/$ip_address
address=/onboarding.$CLUSTER_FQDN/$ip_address
address=/orchestrator-license.$CLUSTER_FQDN/$ip_address
address=/rancher.$CLUSTER_FQDN/$ip_address
address=/registry-oci.$CLUSTER_FQDN/$ip_address
address=/registry.$CLUSTER_FQDN/$ip_address
address=/release.$CLUSTER_FQDN/$ip_address
address=/telemetry-node.$CLUSTER_FQDN/$ip_address
address=/telemetry.$CLUSTER_FQDN/$ip_address
address=/tinkerbell-server.$CLUSTER_FQDN/$ip_address
address=/update-node.$CLUSTER_FQDN/$ip_address
address=/update.$CLUSTER_FQDN/$ip_address
address=/vault.$CLUSTER_FQDN/$ip_address
address=/vnc.$CLUSTER_FQDN/$ip_address
address=/web-ui.$CLUSTER_FQDN/$ip_address
address=/ws-app-service-proxy.$CLUSTER_FQDN/$ip_address
address=/mps.$CLUSTER_FQDN/$ip_address
address=/rps.$CLUSTER_FQDN/$ip_address
address=/mps-wss.$CLUSTER_FQDN/$ip_address
address=/rps-wss.$CLUSTER_FQDN/$ip_address
EOL
}

# Main execution logic
if [ "$ACTION" == "setup" ]; then
    setup_dns
    sudo systemctl restart dnsmasq
    sudo systemctl enable dnsmasq
    cat /etc/resolv.conf
    cat /etc/dnsmasq.conf

elif [ "$ACTION" == "config" ]; then
    update_host_dns
    sudo systemctl restart dnsmasq
    sudo systemctl enable dnsmasq
    echo "DNS config updated — all entries point to host IP: $ip_address"
    sudo cat /etc/dnsmasq.d/cluster-hosts-dns.conf
else
    echo "Invalid action: $ACTION"
    echo "Usage: $0 [cluster.fqdn] {setup|config}  (default FQDN: cluster.onprem)"
    exit 1
fi
