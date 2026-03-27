#!/bin/bash
set -euo pipefail

# ═══════════════════════════════════════════════════════════════
# helm-deploy.sh — Deploy helm charts using values_overwrite files
#
# Extracts chart info (repoURL, chart, version, namespace) from
# argocd/applications/templates/<app>.yaml and deploys using
# values_overwrite/values_<app>.yaml as the values override.
#
# Usage:
#   ./helm-deploy.sh install [app1 app2 ...]    # install specific apps
#   ./helm-deploy.sh uninstall [app1 app2 ...]  # uninstall specific apps
#   ./helm-deploy.sh install                     # install all from app list
#   ./helm-deploy.sh uninstall                   # uninstall all (reverse order)
#   ./helm-deploy.sh list                        # show parsed chart info
#   ./helm-deploy.sh dry-run [app1 ...]          # show helm commands only
# ═══════════════════════════════════════════════════════════════

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "$SCRIPT_DIR"

TEMPLATES_DIR="${REPO_ROOT}/argocd/applications/templates"
VALUES_DIR="values_overwrite"
APP_LIST_FILE="application-list-vpro"

# Default OCI registry (replaces {{ .Values.argo.chartRepoURL }})
RELEASE_SERVICE_URL="${RELEASE_SERVICE_URL:-registry-rs.edgeorchestration.intel.com/edge-orch}"

# Known public helm repos: repoURL → repo_alias
declare -A KNOWN_REPOS
KNOWN_REPOS=(
  ["https://charts.jetstack.io"]="jetstack"
  ["https://charts.external-secrets.io"]="external-secrets"
  ["https://istio-release.storage.googleapis.com/charts"]="istio"
  ["https://stakater.github.io/stakater-charts"]="stakater"
  ["https://metallb.github.io/metallb"]="metallb"
  ["https://kiali.org/helm-charts"]="kiali"
  ["https://prometheus-community.github.io/helm-charts"]="prometheus-community"
  ["https://helm.traefik.io/traefik"]="traefik"
  ["https://helm.releases.hashicorp.com"]="hashicorp"
  ["https://haproxytech.github.io/helm-charts"]="haproxytech"
  ["https://aws.github.io/eks-charts"]="eks"
  ["https://kubernetes.github.io/ingress-nginx"]="ingress-nginx"
  ["https://kubernetes-sigs.github.io/cluster-api-operator"]="capi-operator"
  ["https://kyverno.github.io/kyverno"]="kyverno"
  ["https://rancher.github.io/fleet-helm-charts/"]="fleet"
  ["https://helm.goharbor.io"]="harbor"
  ["https://kubernetes.github.io/autoscaler"]="autoscaler"
)

# ═══════════════════════════════════════════════════════════════
# Parse argocd template to extract: repoURL, chart, version, namespace
# ═══════════════════════════════════════════════════════════════
parse_template() {
  local app="$1"
  local tpl="${TEMPLATES_DIR}/${app}.yaml"

  if [[ ! -f "$tpl" ]]; then
    echo "❌ Template not found: $tpl" >&2
    return 1
  fi

  # Extract namespace
  T_NAMESPACE=$(grep -oP '\$namespace\s*:=\s*"\K[^"]+' "$tpl" 2>/dev/null || echo "")

  # Extract repoURL (first match, strip Helm template wrappers and quotes)
  local raw_repo
  raw_repo=$(grep -m1 'repoURL:' "$tpl" | sed 's/.*repoURL:\s*//' | tr -d '"' | xargs)

  # Extract chart name
  T_CHART=$(grep -m1 '^\s*chart:' "$tpl" | sed 's/.*chart:\s*//' | tr -d '"' | xargs)

  # Resolve chart template variables like {{$appName}}, {{$chartName}}
  T_CHART=$(echo "$T_CHART" | sed 's/{{\s*\$appName\s*}}/'"$app"'/g')
  # For $chartName, extract it from the template
  local chart_name
  chart_name=$(grep -oP '\$chartName\s*:=\s*"\K[^"]+' "$tpl" 2>/dev/null || echo "")
  if [[ -n "$chart_name" ]]; then
    T_CHART=$(echo "$T_CHART" | sed 's/{{\s*\$chartName\s*}}/'"$chart_name"'/g')
  fi

  # Extract version
  T_VERSION=$(grep -m1 'targetRevision:' "$tpl" | sed 's/.*targetRevision:\s*//' | tr -d '"' | xargs)

  # Determine chart type and resolve repoURL
  if [[ "$raw_repo" == *"chartRepoURL"* || "$raw_repo" == *"rsChartRepoURL"* ]]; then
    # OCI chart via argo registry → oci://<RELEASE_SERVICE_URL>/<chart>
    T_TYPE="oci"
    T_RESOLVED="oci://${RELEASE_SERVICE_URL}/${T_CHART}"
    T_CHART_FOR_HELM="$T_RESOLVED"
  elif [[ "$raw_repo" == *"ghcr.io"* || "$raw_repo" == *"gcr.io"* ]]; then
    # External OCI
    T_TYPE="oci-ext"
    T_CHART_FOR_HELM="oci://${raw_repo}/${T_CHART}"
  elif [[ "$raw_repo" == https://* ]]; then
    # Public helm repo
    T_TYPE="repo"
    local alias="${KNOWN_REPOS[$raw_repo]:-}"
    if [[ -z "$alias" ]]; then
      # Auto-generate alias from URL
      alias=$(echo "$raw_repo" | sed 's|https://||; s|/.*||; s|\..*||')
    fi
    T_REPO_ALIAS="$alias"
    T_REPO_URL="$raw_repo"
    T_CHART_FOR_HELM="${alias}/${T_CHART}"
  else
    echo "⚠️  Unknown repoURL pattern for ${app}: $raw_repo" >&2
    T_TYPE="unknown"
    T_CHART_FOR_HELM="$raw_repo/$T_CHART"
  fi
}

# ═══════════════════════════════════════════════════════════════
# Load application list
# ═══════════════════════════════════════════════════════════════
load_apps() {
  local apps=("$@")
  if [[ ${#apps[@]} -eq 0 ]]; then
    if [[ -f "$APP_LIST_FILE" ]]; then
      mapfile -t apps < "$APP_LIST_FILE"
    else
      echo "❌ No apps specified and $APP_LIST_FILE not found"
      exit 1
    fi
  fi
  echo "${apps[@]}"
}

# ═══════════════════════════════════════════════════════════════
# PRE-INSTALL HOOKS — app-specific cleanup before helm install
# ═══════════════════════════════════════════════════════════════
pre_install_hook() {
  local app="$1"
  local ns="$2"

  case "$app" in
    vault)
      # When reinstalling vault, the old vault-keys secret blocks secrets-config
      # from storing new init keys (it uses Create, not Update). Delete it so
      # secrets-config can create a fresh one after Vault reinitializes.
      if kubectl get secret vault-keys -n "$ns" >/dev/null 2>&1; then
        echo "   🔑 Removing stale vault-keys secret (will be recreated by secrets-config)"
        kubectl delete secret vault-keys -n "$ns" 2>/dev/null || true
      fi

      # If vault is already running and initialized, truncate PG tables to force
      # a clean reinit. Skip if vault pod doesn't exist or DB isn't reachable.
      local pg_ns="orch-database"
      local pg_pod
      pg_pod=$(kubectl get pods -n "$pg_ns" -l app.kubernetes.io/name=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
      if [[ -n "$pg_pod" ]]; then
        echo "   🗄️  Truncating vault tables in PostgreSQL for clean reinit"
        kubectl exec "$pg_pod" -n "$pg_ns" -c postgres -- \
          psql -U postgres -d vault -c "TRUNCATE vault_kv_store, vault_ha_locks;" 2>/dev/null || true
      fi
      ;;
    orch-utils)
      # secrets-config (part of orch-utils) creates the vault-keys secret.
      # Remove stale secret so the new secrets-config pod can store fresh keys.
      if kubectl get secret vault-keys -n "$ns" >/dev/null 2>&1; then
        echo "   🔑 Removing stale vault-keys secret (will be recreated by secrets-config)"
        kubectl delete secret vault-keys -n "$ns" 2>/dev/null || true
      fi
      ;;
  esac
}

# ═══════════════════════════════════════════════════════════════
# INSTALL
# ═══════════════════════════════════════════════════════════════
do_install() {
  local app="$1"

  if ! parse_template "$app"; then
    echo "⏭️  Skipping ${app}: no template"
    return 2
  fi

  local values_file="${VALUES_DIR}/values_${app}.yaml"
  local values_args=""
  if [[ -f "$values_file" ]]; then
    values_args="-f $values_file"
  else
    echo "⚠️  No values override: $values_file"
  fi

  # Add helm repo if needed
  if [[ "$T_TYPE" == "repo" ]]; then
    helm repo add "$T_REPO_ALIAS" "$T_REPO_URL" >/dev/null 2>&1 || true
    helm repo update >/dev/null 2>&1
  fi

  # Ensure namespace (best-effort; --create-namespace in helm is the fallback)
  if [[ -n "$T_NAMESPACE" ]]; then
    kubectl create ns "$T_NAMESPACE" --dry-run=client -o yaml 2>/dev/null | kubectl apply -f - 2>/dev/null || true
  fi

  # Run app-specific pre-install cleanup
  pre_install_hook "$app" "$T_NAMESPACE"

  echo "🚀 Installing ${app}"
  echo "   Chart    : ${T_CHART_FOR_HELM}"
  echo "   Version  : ${T_VERSION}"
  echo "   Namespace: ${T_NAMESPACE}"
  echo "   Values   : ${values_file}"

  # shellcheck disable=SC2086
  if helm upgrade --install "$app" "$T_CHART_FOR_HELM" \
    --version "$T_VERSION" \
    --namespace "$T_NAMESPACE" \
    --create-namespace \
    $values_args \
    --wait --timeout 10m; then
    echo "✅ ${app} installed"
  else
    echo "❌ ${app} FAILED"
    return 1
  fi
}

# ═══════════════════════════════════════════════════════════════
# UNINSTALL
# ═══════════════════════════════════════════════════════════════
do_uninstall() {
  local app="$1"

  if ! parse_template "$app"; then
    echo "⏭️  Skipping ${app}: no template"
    return 1
  fi

  echo "🗑️  Uninstalling ${app} from ${T_NAMESPACE}..."
  if helm status "$app" -n "$T_NAMESPACE" >/dev/null 2>&1; then
    helm uninstall "$app" -n "$T_NAMESPACE"
    echo "✅ ${app} uninstalled"
  else
    echo "ℹ️  ${app} not found in ${T_NAMESPACE}"
  fi
}

# ═══════════════════════════════════════════════════════════════
# DRY-RUN (show commands without executing)
# ═══════════════════════════════════════════════════════════════
do_dry_run() {
  local app="$1"

  if ! parse_template "$app"; then
    return 1
  fi

  local values_file="${VALUES_DIR}/values_${app}.yaml"
  local values_args=""
  [[ -f "$values_file" ]] && values_args="-f $values_file"

  local repo_add=""
  if [[ "$T_TYPE" == "repo" ]]; then
    repo_add="helm repo add ${T_REPO_ALIAS} ${T_REPO_URL}"
  fi

  echo "--- ${app} ---"
  [[ -n "$repo_add" ]] && echo "  $repo_add"
  echo "  helm upgrade --install ${app} ${T_CHART_FOR_HELM} \\"
  echo "    --version ${T_VERSION} \\"
  echo "    --namespace ${T_NAMESPACE} \\"
  echo "    --create-namespace \\"
  [[ -n "$values_args" ]] && echo "    ${values_args} \\"
  echo "    --wait --timeout 10m"
  echo ""
}

# ═══════════════════════════════════════════════════════════════
# LIST (show parsed info for all apps)
# ═══════════════════════════════════════════════════════════════
do_list() {
  printf "%-35s %-18s %-8s %-55s %s\n" "APPLICATION" "NAMESPACE" "VERSION" "CHART" "VALUES"
  printf "%-35s %-18s %-8s %-55s %s\n" "───────────" "─────────" "───────" "─────" "──────"

  local apps
  if [[ -f "$APP_LIST_FILE" ]]; then
    mapfile -t apps < "$APP_LIST_FILE"
  else
    # Fall back to all templates
    for f in "${TEMPLATES_DIR}"/*.yaml; do
      apps+=("$(basename "$f" .yaml)")
    done
  fi

  for app in "${apps[@]}"; do
    if parse_template "$app" 2>/dev/null; then
      local vf="${VALUES_DIR}/values_${app}.yaml"
      local has_vals="✅"
      [[ ! -f "$vf" ]] && has_vals="❌"
      printf "%-35s %-18s %-8s %-55s %s\n" "$app" "$T_NAMESPACE" "$T_VERSION" "$T_CHART_FOR_HELM" "$has_vals"
    else
      printf "%-35s %-18s %-8s %-55s %s\n" "$app" "?" "?" "NO TEMPLATE" "❌"
    fi
  done
}

# ═══════════════════════════════════════════════════════════════
# MAIN
# ═══════════════════════════════════════════════════════════════

ACTION="${1:-}"
shift || true

usage() {
  echo "Usage: $0 <action> [app1 app2 ...]"
  echo ""
  echo "Actions:"
  echo "  install [app...]     Install apps (all from app list if none specified)"
  echo "  uninstall [app...]   Uninstall apps (reverse order if none specified)"
  echo "  dry-run [app...]     Show helm commands without executing"
  echo "  list                 Show parsed chart info for all apps"
  echo ""
  echo "Chart info is extracted from: ${TEMPLATES_DIR}/<app>.yaml"
  echo "Values overrides from:        ${VALUES_DIR}/values_<app>.yaml"
  exit 1
}

case "$ACTION" in
  install)
    APPS=($(load_apps "$@"))
    TOTAL=${#APPS[@]}
    START_ALL=$(date +%s)
    success=0; fail=0; skipped=0
    SUCCEEDED_LIST=()
    FAILED_LIST=()
    SKIPPED_LIST=()
    TIMINGS=()

    for i in "${!APPS[@]}"; do
      app="${APPS[$i]}"
      idx=$((i + 1))

      echo ""
      echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
      echo "📦 [${idx}/${TOTAL}] ${app}"
      echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

      START=$(date +%s)
      if do_install "$app"; then
        success=$((success + 1))
        SUCCEEDED_LIST+=("$app")
      else
        rc=$?
        if [[ $rc -eq 2 ]]; then
          skipped=$((skipped + 1))
          SKIPPED_LIST+=("$app")
        else
          fail=$((fail + 1))
          FAILED_LIST+=("$app")
        fi
      fi
      END=$(date +%s)
      dur=$((END - START))
      TIMINGS+=("${app}:${dur}s")

      # Progress bar
      done_count=$((success + fail + skipped))
      pct=$((done_count * 100 / TOTAL))
      echo ""
      echo "  Progress: ${pct}% (${done_count}/${TOTAL}) ✅${success} ❌${fail} ⏭️${skipped}  ⏱️${dur}s"
    done

    END_ALL=$(date +%s)
    TOTAL_DURATION=$((END_ALL - START_ALL))

    # ════════════════════════════════════════
    # Summary Report
    # ════════════════════════════════════════
    echo ""
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║                    DEPLOYMENT SUMMARY                       ║"
    echo "╠══════════════════════════════════════════════════════════════╣"
    echo "║  Total Apps     : ${TOTAL}"
    echo "║  ✅ Succeeded   : ${success}"
    echo "║  ❌ Failed      : ${fail}"
    echo "║  ⏭️  Skipped     : ${skipped}"
    echo "║  ⏱️  Total Time  : ${TOTAL_DURATION}s"
    echo "╠══════════════════════════════════════════════════════════════╣"

    if [[ ${#SUCCEEDED_LIST[@]} -gt 0 ]]; then
      echo "║  ✅ Succeeded:"
      for a in "${SUCCEEDED_LIST[@]}"; do
        echo "║     • $a"
      done
    fi

    if [[ ${#FAILED_LIST[@]} -gt 0 ]]; then
      echo "║  ❌ Failed:"
      for a in "${FAILED_LIST[@]}"; do
        echo "║     • $a"
      done
    fi

    if [[ ${#SKIPPED_LIST[@]} -gt 0 ]]; then
      echo "║  ⏭️  Skipped:"
      for a in "${SKIPPED_LIST[@]}"; do
        echo "║     • $a"
      done
    fi

    echo "╠══════════════════════════════════════════════════════════════╣"
    echo "║  Timing breakdown:"
    for t in "${TIMINGS[@]}"; do
      echo "║     ${t}"
    done
    echo "╚══════════════════════════════════════════════════════════════╝"
    ;;
  uninstall)
    APPS=($(load_apps "$@"))
    # Reverse order for uninstall
    if [[ $# -eq 0 ]]; then
      REVERSED=()
      for (( i=${#APPS[@]}-1 ; i>=0 ; i-- )); do
        REVERSED+=("${APPS[$i]}")
      done
      APPS=("${REVERSED[@]}")
    fi
    for app in "${APPS[@]}"; do
      do_uninstall "$app" || true
      echo "────────────────────────────────────────"
    done
    echo "✅ Uninstall complete"
    ;;
  dry-run)
    APPS=($(load_apps "$@"))
    for app in "${APPS[@]}"; do
      do_dry_run "$app"
    done
    ;;
  list)
    do_list
    ;;
  *)
    usage
    ;;
esac


