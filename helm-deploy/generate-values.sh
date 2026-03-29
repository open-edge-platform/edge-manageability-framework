#!/bin/bash
set -euo pipefail

# =========================
# Source environment config
# =========================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${SCRIPT_DIR}/onprem.env"
if [[ -f "$ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  echo "🔹 Loaded env from $ENV_FILE"
else
  echo "⚠️  $ENV_FILE not found — using existing environment"
fi
rm -rf merge.yaml
# =========================
# Profile selection
# =========================
ORCH_INSTALLER_PROFILE="${ORCH_INSTALLER_PROFILE:-onprem-vpro}"

case "$ORCH_INSTALLER_PROFILE" in
  onprem-eim)
    APP_LIST_FILE="${SCRIPT_DIR}/application-list-eim"
    FILES=(
      "onprem-eim.yaml"
      "${REPO_ROOT}/orch-configs/profiles/enable-platform.yaml"
      "${REPO_ROOT}/orch-configs/profiles/enable-edgeinfra.yaml"
      "${REPO_ROOT}/orch-configs/profiles/enable-full-ui.yaml"
      "${REPO_ROOT}/orch-configs/profiles/enable-onprem.yaml"
      "${REPO_ROOT}/orch-configs/profiles/enable-sre.yaml"
      "${REPO_ROOT}/orch-configs/profiles/proxy-none.yaml"
      "${REPO_ROOT}/orch-configs/profiles/profile-onprem.yaml"
      "${REPO_ROOT}/orch-configs/profiles/alerting-emails.yaml"
      "${REPO_ROOT}/orch-configs/profiles/eim-noobb.yaml"
      "${REPO_ROOT}/orch-configs/profiles/resource-default.yaml"
      "${REPO_ROOT}/orch-configs/profiles/artifact-rs-production-noauth.yaml"
      "${REPO_ROOT}/argocd/applications/values.yaml"
    )
    ;;
  onprem-vpro)
    APP_LIST_FILE="${SCRIPT_DIR}/application-list-vpro"
    FILES=(
      "onprem-vpro.yaml"
      "${REPO_ROOT}/orch-configs/profiles/enable-platform-vpro.yaml"
      "${REPO_ROOT}/orch-configs/profiles/enable-edgeinfra-vpro.yaml"
      "${REPO_ROOT}/orch-configs/profiles/enable-onprem.yaml"
      "${REPO_ROOT}/orch-configs/profiles/enable-sre.yaml"
      "${REPO_ROOT}/orch-configs/profiles/proxy-none.yaml"
      "${REPO_ROOT}/orch-configs/profiles/profile-onprem.yaml"
      "${REPO_ROOT}/orch-configs/profiles/resource-default.yaml"
      "${REPO_ROOT}/orch-configs/profiles/artifact-rs-production-noauth.yaml"
      "${REPO_ROOT}/argocd/applications/values.yaml"
    )
    ;;
  *)
    echo "❌ Unknown ORCH_INSTALLER_PROFILE: $ORCH_INSTALLER_PROFILE"
    echo "   Supported: onprem-eim, onprem-vpro"
    exit 1
    ;;
esac

echo "🔹 Profile: $ORCH_INSTALLER_PROFILE"

# =========================
# Applications to process
# =========================
if [ $# -gt 0 ]; then
  APPLICATIONS=("$@")
elif [[ -f "$APP_LIST_FILE" ]]; then
  mapfile -t APPLICATIONS < "$APP_LIST_FILE"
else
  echo "❌ No arguments provided and $APP_LIST_FILE not found"
  exit 1
fi

# =========================
# Output files
# =========================
MERGED_FILE="merge.yaml"
OUTPUT_DIR="values_overwrite"
mkdir -p "$OUTPUT_DIR"

# =========================
# Validate tools
# =========================
yq --version | grep -Eq "v?4\." || { echo "❌ yq v4 is required"; exit 1; }
command -v helm >/dev/null || { echo "❌ helm is required"; exit 1; }

# =========================
# Validate input files
# =========================
echo "🔍 Validating profile files..."
for f in "${FILES[@]}"; do
  [[ -f "$f" ]] || { echo "❌ Missing file: $f"; exit 1; }
done

# =========================
# STEP 1: Merge YAMLs (once) — skip if merge.yaml already exists
# =========================
if [[ -f "$MERGED_FILE" ]]; then
  echo "🔹 Step 1: Reusing existing $MERGED_FILE (delete it to regenerate)"
else
  echo "🔹 Step 1: Merging profiles → $MERGED_FILE"
  yq eval-all '. as $item ireduce ({}; $item * . )' "${FILES[@]}" > "$MERGED_FILE"
  echo "✅ Created: $MERGED_FILE"
fi

# =========================
# Create a temp helm chart (once, reused per app)
# =========================
CHART_DIR=$(mktemp -d)
mkdir -p "${CHART_DIR}/templates"
cat > "${CHART_DIR}/Chart.yaml" <<EOF
apiVersion: v2
name: render
version: 0.1.0
EOF

# =========================
# STEP 2: Loop over applications
# =========================
success_count=0
skip_count=0

for application in "${APPLICATIONS[@]}"; do
  CONFIG_FILE="${REPO_ROOT}/argocd/applications/configs/${application}.yaml"
  TPL_FILE="${REPO_ROOT}/argocd/applications/custom/${application}.tpl"
  VALUES_FILE="${OUTPUT_DIR}/values_${application}.yaml"

  # Skip only if BOTH config and tpl are missing
  if [[ ! -f "$CONFIG_FILE" && ! -f "$TPL_FILE" ]]; then
    echo "⏭️  Skipping ${application}: missing both files"
    skip_count=$((skip_count + 1))
    continue
  fi

  # If no tpl, just merge config + profiles
  if [[ ! -f "$TPL_FILE" ]]; then
    if [[ -f "$CONFIG_FILE" ]]; then
      yq eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' "$CONFIG_FILE" "$MERGED_FILE" > "$VALUES_FILE"
    else
      cp "$MERGED_FILE" "$VALUES_FILE"
    fi
    # Remove comment lines
    sed -i '/^#/d' "$VALUES_FILE"
    echo "  ✅ ${application} → $VALUES_FILE (no template)"
    success_count=$((success_count + 1))
    continue
  fi

  echo "🔸 Processing: ${application}"

  # Copy tpl into chart templates
  cp "$TPL_FILE" "${CHART_DIR}/templates/${application}.yaml"

  # Build merged values file for this app
  MERGED_VALUES=$(mktemp)
  if [[ -f "$CONFIG_FILE" ]]; then
    yq eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' "$CONFIG_FILE" "$MERGED_FILE" > "$MERGED_VALUES"
  else
    cp "$MERGED_FILE" "$MERGED_VALUES"
  fi

  # Render with helm template
  RENDERED=$(mktemp)
  helm template render "${CHART_DIR}" -f "$MERGED_VALUES" 2>/dev/null \
    | sed '/^---$/d; /^# Source:/d; /^#/d; /^$/d' \
    > "$RENDERED" || true

  # Merge: base config + rendered tpl (matches ArgoCD mergeOverwrite behavior)
  if [[ -f "$CONFIG_FILE" && -s "$RENDERED" ]]; then
    yq eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' "$CONFIG_FILE" "$RENDERED" > "$VALUES_FILE"
  elif [[ -s "$RENDERED" ]]; then
    cp "$RENDERED" "$VALUES_FILE"
  elif [[ -f "$CONFIG_FILE" ]]; then
    cp "$CONFIG_FILE" "$VALUES_FILE"
  else
    touch "$VALUES_FILE"
  fi

  # Remove comment lines
  sed -i '/^#/d' "$VALUES_FILE"

  # Clean up for next iteration
  rm -f "${CHART_DIR}/templates/${application}.yaml" "$MERGED_VALUES" "$RENDERED"

  echo "  ✅ ${application} → $VALUES_FILE"
  success_count=$((success_count + 1))
done

rm -rf "$CHART_DIR"

# =========================
# Post-processing: fix nil keys at ALL depths (yq merge / tpl artifact)
# =========================
# When yq merges or helm tpl renders a key with no children (e.g. "hook:"),
# it becomes null. Helm charts that traverse these keys (e.g.
# .Values.stack.hook.port) crash with nil pointer errors.
# Recursively convert all null values → {} at every depth.
echo "🔧 Cleaning nil keys (all depths) in generated values files..."
nil_fix_count=0
for VF in "${OUTPUT_DIR}"/values_*.yaml; do
  [[ -f "$VF" ]] || continue
  before=$(yq '[.. | select(tag == "!!null")] | length' "$VF" 2>/dev/null || echo "0")
  if [[ "$before" -gt 0 ]]; then
    yq -i '(.. | select(tag == "!!null")) = {}' "$VF"
    echo "   🔧 Fixed ${before} nil key(s) in $(basename "$VF")"
    nil_fix_count=$((nil_fix_count + before))
  fi
done
if [[ $nil_fix_count -gt 0 ]]; then
  echo "   ✅ Fixed ${nil_fix_count} total nil key(s) → {}"
else
  echo "   ✅ No nil keys found"
fi

# =========================
# Post-processing: fix known array fields that were incorrectly set to {}
# =========================
# The nil→{} fix above converts ALL nulls to empty maps, but some Helm chart
# schema fields require arrays ([]). For example, global.imagePullSecrets
# must be an array or Helm will reject it with:
#   "got object, want array"
# Add any known array-typed keys to this list.
KNOWN_ARRAY_KEYS=(
  "imagePullSecrets"
  "tolerations"
)
echo "🔧 Fixing known array fields that were set to {} instead of []..."
array_fix_count=0
for VF in "${OUTPUT_DIR}"/values_*.yaml; do
  [[ -f "$VF" ]] || continue
  for key in "${KNOWN_ARRAY_KEYS[@]}"; do
    # Find all paths where this key is an empty map and convert to empty array
    matches=$(yq ".. | select(has(\"${key}\")) | .${key} | select(tag == \"!!map\" and length == 0) | path | join(\".\")" "$VF" 2>/dev/null | wc -l)
    if [[ "$matches" -gt 0 ]]; then
      yq -i "(.. | select(has(\"${key}\")).${key}) |= (select(tag == \"!!map\" and length == 0) | [] // .)" "$VF"
      echo "   🔧 Fixed ${key} → [] in $(basename "$VF")"
      array_fix_count=$((array_fix_count + matches))
    fi
  done
done
if [[ $array_fix_count -gt 0 ]]; then
  echo "   ✅ Fixed ${array_fix_count} array field(s) from {} → []"
else
  echo "   ✅ No array field fixes needed"
fi

# =========================
# Post-processing: handle Istio sidecar annotations for traefik
# =========================
# Priority: 1) ISTIO_ENABLED env var  2) app list file  3) APPLICATIONS array
if [[ -z "${ISTIO_ENABLED:-}" ]]; then
  # Env var not set — auto-detect from app list
  ISTIO_ENABLED=false
  if [[ -f "$APP_LIST_FILE" ]]; then
    if grep -qE '^(istiod|istio-base)$' "$APP_LIST_FILE" 2>/dev/null; then
      ISTIO_ENABLED=true
    fi
  else
    for app in "${APPLICATIONS[@]}"; do
      if [[ "$app" == "istiod" || "$app" == "istio-base" ]]; then
        ISTIO_ENABLED=true
        break
      fi
    done
  fi
  echo "ℹ️  ISTIO_ENABLED not set in env — auto-detected: ${ISTIO_ENABLED}"
else
  echo "ℹ️  ISTIO_ENABLED from env: ${ISTIO_ENABLED}"
fi

if [[ "$ISTIO_ENABLED" == "true" ]]; then
  echo "🔧 Istio enabled — ensuring traefik has excludeInboundPorts annotations"
  for tf in traefik traefik-pre traefik-boots; do
    VF="${OUTPUT_DIR}/values_${tf}.yaml"
    if [[ -f "$VF" ]]; then
      if ! grep -q 'traffic.sidecar.istio.io/excludeInboundPorts' "$VF" 2>/dev/null; then
        echo "   ⚠️  WARNING: ${tf} is missing traffic.sidecar.istio.io/excludeInboundPorts"
        echo "      Istio sidecar may intercept Traefik ports and break TLS!"
      else
        echo "   ✅ ${tf} has Istio exclude annotations"
      fi
    fi
  done
else
  echo "🔧 Istio disabled — setting sidecar.istio.io/inject=false for traefik"
  for tf in traefik traefik-pre traefik-boots; do
    VF="${OUTPUT_DIR}/values_${tf}.yaml"
    if [[ -f "$VF" ]]; then
      yq -i '(.deployment.podAnnotations) = {"sidecar.istio.io/inject": "false"}' "$VF" 2>/dev/null || true
      echo "   ✅ Set sidecar.istio.io/inject=false in values_${tf}.yaml"
    fi
  done
fi

echo ""
echo "🎉 Done! Processed: ${success_count}, Skipped: ${skip_count}"


