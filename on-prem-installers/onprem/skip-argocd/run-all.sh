#!/bin/bash

set -e

ACTION=$1
shift

if [[ "$ACTION" != "install" && "$ACTION" != "uninstall" ]]; then
  echo "❌ Usage:"
  echo "  $0 install [component1 component2 ...]"
  echo "  $0 uninstall [component1 component2 ...]"
  exit 1
fi

# Format: "name:directory script"
COMPONENTS=(
  "metallb:metallb deploy-metallb.sh"
  "keycloak-operator:keycloak-operator keycloak-operator.sh"
  "postgresql-operator:postgresql-operator postgresql-operator.sh"
  "namespace-label:namespace-label namespace-label.sh"
  "cert-manager:cert-manager cert-manager.sh"
  "external-secrets:external-secrets external-secrets.sh"
#NA  "istio-base:istio-base istio-base.sh"
#NA  "k8s-metrics-server:k8s-metrics-server k8s-metrics-server.sh"
#NA  "prometheus-crd:prometheus-crd prometheus-crd.sh"
#NA  "istiod:istiod istiod.sh"
  "reloader:reloader deploy-reloader.sh"
#NA  "wait-istio-job:wait-istio-job wait-istio-job.sh"
  "postgresql-secrets:postgresql-secrets postgresql-secrets.sh"
  "postgresql-cluster:postgresql-cluster postgresql-cluster.sh"
#NA  "istio-policy:istio-policy istio-policy.sh"
#NA  "kiali:kiali kiali.sh"
#NA  "platform-autocert:platform-autocert platform-autocert.sh"
  "platform-keycloak:platform-keycloak platform-keycloak.sh"
#AWS/Coder  "cert-synchronizer:cert-synchronizer cert-synchronizer.sh"
  "secrets-config:secrets-config secrets-config.sh"
  "self-signed-cert:self-signed-cert self-signed-cert.sh"
  "vault:vault vault.sh"
  "rs-proxy:rs-proxy rs-proxy.sh"
  "secret-wait-tls-orch:secret-wait-tls-orch secret-wait-tls-orch.sh"
  "copy-ca-cert-gateway-to-cattle:copy-ca-cert-gateway-to-cattle copy-ca-cert-gateway-to-cattle.sh"
  "copy-ca-cert-gateway-to-infra:copy-ca-cert-gateway-to-infra copy-ca-cert-gateway-to-infra.sh"
  "copy-keycloak-admin-to-infra:copy-keycloak-admin-to-infra copy-keycloak-admin-to-infra.sh"
  "traefik-pre:traefik-pre traefik-pre.sh"
  "ingress-haproxy:ingress-haproxy ingress-haproxy.sh"
  "traefik:traefik traefik.sh"
  "haproxy-ingress-pxe-boots:haproxy-ingress-pxe-boots haproxy-ingress-pxe-boots.sh"
  "tenancy-datamodel:tenancy-datamodel tenancy-datamodel.sh"
  "nexus-api-gw:nexus-api-gw nexus-api-gw.sh"
  "tenancy-api-mapping:tenancy-api-mapping tenancy-api-mapping.sh"
#NA  "botkube:botkube botkube.sh"
)

# If user passed specific components
SELECTED=("$@")

run_component() {
  local name="$1"
  local dir="$2"
  local script="$3"

  if [[ -d "$dir" && -f "$dir/$script" ]]; then
    echo "🚀 [$name] Running: $dir/$script $ACTION"
    (
      cd "$dir" || exit 1
      chmod +x "$script"
      ./"$script" "$ACTION"
    )
  else
    echo "⚠️ [$name] Skipping (missing)"
  fi
}

should_run() {
  local name="$1"

  # If no selection → run all
  if [[ ${#SELECTED[@]} -eq 0 ]]; then
    return 0
  fi

  # Check if name is in selected list
  for sel in "${SELECTED[@]}"; do
    if [[ "$sel" == "$name" ]]; then
      return 0
    fi
  done

  return 1
}

# Build execution list
EXEC_LIST=()

if [[ "$ACTION" == "install" ]]; then
  EXEC_LIST=("${COMPONENTS[@]}")
else
  # Reverse order for uninstall
  for (( i=${#COMPONENTS[@]}-1 ; i>=0 ; i-- )); do
    EXEC_LIST+=("${COMPONENTS[i]}")
  done
fi

echo "👉 Action: $ACTION"
[[ ${#SELECTED[@]} -gt 0 ]] && echo "👉 Selected: ${SELECTED[*]}" || echo "👉 Selected: ALL"

# Execute
for item in "${EXEC_LIST[@]}"; do
  name=$(echo "$item" | cut -d':' -f1)
  rest=$(echo "$item" | cut -d':' -f2)
  dir=$(echo "$rest" | awk '{print $1}')
  script=$(echo "$rest" | awk '{print $2}')

  if should_run "$name"; then
    run_component "$name" "$dir" "$script"
  else
    echo "⏭️ Skipping [$name]"
  fi
done

echo "✅ Completed: $ACTION"
