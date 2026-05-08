#!/bin/bash
# deploy-nexus-replacement.sh — Build, deploy, and verify the nexus replacement
# on a kind-based orchestrator in a single workflow.
#
# Usage:
#   ./deploy-nexus-replacement.sh [all|repos|build|tag|patch|deploy|load|verify]
#
#   all     — run every step (default)
#   repos   — clone missing repos and switch to nexus-replacement branch
#   build   — build custom Docker images + tag with full registry paths
#   tag     — tag built images with full registry paths (for kind loading)
#   patch   — patch ArgoCD templates with custom image tags and push to GitHub
#   deploy  — deploy the orchestrator (assumes build + tag + patch done)
#   load    — load images into kind only (useful if cluster exists but images aren't loaded)
#   verify  — run verification checks only

set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# Configuration — edit these if your layout differs
# ──────────────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${SCRIPT_DIR}"

CUSTOM_TAG="${CUSTOM_TAG:-nexus-replacement-$(date +%Y%m%d)}"
BASE_PRESET="${BASE_PRESET:-dev-internal-coder-autocert.yaml}"

# Normalize case-variant env vars (bash is case-sensitive; guard against
# common typos like DISABLE_O11y_PROFILE instead of DISABLE_O11Y_PROFILE).
[[ -n "${DISABLE_O11y_PROFILE:-}" && -z "${DISABLE_O11Y_PROFILE:-}" ]] && export DISABLE_O11Y_PROFILE="${DISABLE_O11y_PROFILE}"
[[ -n "${DISABLE_Co_PROFILE:-}" && -z "${DISABLE_CO_PROFILE:-}" ]]  && export DISABLE_CO_PROFILE="${DISABLE_Co_PROFILE}"
[[ -n "${DISABLE_Ao_PROFILE:-}" && -z "${DISABLE_AO_PROFILE:-}" ]]  && export DISABLE_AO_PROFILE="${DISABLE_Ao_PROFILE}"

# Pin Go version for asdf so repos without .tool-versions still work
export ASDF_GOLANG_VERSION="${ASDF_GOLANG_VERSION:-1.26.1}"

# EMF deployment environment — these must be set for mage deploy:kindPreset
export GIT_USER="${GIT_USER:-git}"
export GIT_TOKEN="${GIT_TOKEN:-$(grep -i password ~/.netrc 2>/dev/null | head -1 | cut -c 12-)}"
export AUTO_CERT="${AUTO_CERT:-1}"
export ORCH_DEFAULT_PASSWORD="${ORCH_DEFAULT_PASSWORD:-ChangeMeOn1stLogin!}"
export CODER_WORKSPACE_NAME="${CODER_WORKSPACE_NAME:-$(hostname)}"

# Tell ArgoCD to pull from our branch instead of HEAD/main
export DEPLOY_REPO_BRANCH="${DEPLOY_REPO_BRANCH:-nexus-replacement}"

SCORCH_DIR="${REPO_ROOT}/scorch"
EMF_DIR="${REPO_ROOT}/edge-manageability-framework"

# orch-cli binary (built from orch-cli/ repo, used for MT setup)
ORCH_CLI="${REPO_ROOT}/orch-cli/build/_output/orch-cli"

# Tenant Manager service details (must match the code defaults or overrides)
TM_SERVICE_NAME="tenancy-manager"
TM_NAMESPACE="orch-iam"
TM_PORT=8080
TM_URL="http://${TM_SERVICE_NAME}.${TM_NAMESPACE}.svc:${TM_PORT}"

# Container registry used by EMF charts (the value of argo.containerRegistryURL)
REGISTRY_URL="registry-rs.edgeorchestration.intel.com/edge-orch"

# Parallel arrays: local build name → registry sub-path.
# Full image ref = ${REGISTRY_URL}/${IMAGE_PATHS[i]}:${CUSTOM_TAG}
IMAGE_NAMES=(
    "tenancy-manager"
    "keycloak-tenant-controller"
    "app-orch-tenant-controller"
    "adm-controller"
    "adm-gateway"
    "infra-tenant-controller"
    "cluster-manager"
    "observability-tenant-controller"
    "metadata-broker"
    "auth-service"
)
IMAGE_PATHS=(
    "common/tenancy-manager"
    "common/keycloak-tenant-controller"
    "app/app-orch-tenant-controller"
    "app/adm-controller"
    "app/adm-gateway"
    "infra/tenant-controller"
    "cluster/cluster-manager"
    "o11y/observability-tenant-controller"
    "orch-ui/metadata-broker"
    "common/auth-service"
)

# ──────────────────────────────────────────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()   { echo -e "${GREEN}[$(date +%H:%M:%S)]${NC} $*"; }
warn()  { echo -e "${YELLOW}[$(date +%H:%M:%S)] WARN:${NC} $*"; }
err()   { echo -e "${RED}[$(date +%H:%M:%S)] ERROR:${NC} $*" >&2; }
die()   { err "$@"; exit 1; }
step()  { echo -e "\n${CYAN}━━━ $* ━━━${NC}"; }

# ──────────────────────────────────────────────────────────────────────────────
# Repos that must be on a nexus-replacement branch (produces custom images
# or is consumed as a Go module replace directive).
# Format: "local_dir|github_org/repo|branch_name"
# Each repo uses its own branch name convention.
# ──────────────────────────────────────────────────────────────────────────────
NEXUS_BRANCH="nexus-replacement"
REPOS_NEEDING_BRANCH=(
    "orch-utils|open-edge-platform/orch-utils|utils_nexus_replacement"
    "infra-core|open-edge-platform/infra-core|infra-nexus-replacement"
    "cluster-manager|open-edge-platform/cluster-manager|cluster_nexus_replacement"
    "o11y-tenant-controller|open-edge-platform/o11y-tenant-controller|o11y_nexus_replacement"
    "orch-metadata-broker|open-edge-platform/orch-metadata-broker|mdb-nexus-replacement"
    "orch-library|open-edge-platform/orch-library|gg/lib-nexus-replacement"
    "scorch|open-edge-platform/scorch|gg/no-proxy-orch-iam"
)

# Repos that should be present but stay on main (deployment infrastructure or
# not yet updated for nexus-replacement).
REPOS_ON_MAIN=(
    "edge-manageability-framework|open-edge-platform/edge-manageability-framework"
    "orch-cli|open-edge-platform/orch-cli"
    # app-orch repos stay on main; AO is disabled via DISABLE_AO_PROFILE=true
    "app-orch-tenant-controller|open-edge-platform/app-orch-tenant-controller"
    "app-orch-deployment|open-edge-platform/app-orch-deployment"
)

# ──────────────────────────────────────────────────────────────────────────────
# Step 0a: Ensure repos are checked out and on the correct branch
# ──────────────────────────────────────────────────────────────────────────────
ensure_repos() {
    step "Ensuring repos are checked out and on branch '${NEXUS_BRANCH}'"

    # When repos are already on the correct (possibly differently-named) branch,
    # set SKIP_REPOS_CHECK=1 to bypass branch switching and just report status.
    if [[ "${SKIP_REPOS_CHECK:-0}" == "1" ]]; then
        log "SKIP_REPOS_CHECK=1 — reporting repo branches only"
        for entry in "${REPOS_NEEDING_BRANCH[@]}"; do
            local dir="${entry%%|*}"
            local path="${REPO_ROOT}/${dir}"
            if [[ -d "${path}" ]]; then
                local current_branch
                current_branch=$(git -C "${path}" branch --show-current 2>/dev/null || echo "DETACHED")
                log "  ${dir}: on '${current_branch}'"
            else
                warn "  ${dir}: NOT FOUND at ${path}"
            fi
        done
        log "Repos check skipped"
        return 0
    fi

    local had_error=0

    for entry in "${REPOS_NEEDING_BRANCH[@]}"; do
        local dir remote target_branch path
        dir=$(echo "${entry}" | cut -d'|' -f1)
        remote=$(echo "${entry}" | cut -d'|' -f2)
        target_branch=$(echo "${entry}" | cut -d'|' -f3)
        # Fall back to the global NEXUS_BRANCH if no per-repo branch specified
        target_branch="${target_branch:-${NEXUS_BRANCH}}"
        path="${REPO_ROOT}/${dir}"

        if [[ ! -d "${path}" ]]; then
            log "Cloning ${remote} → ${dir} (branch: ${target_branch})..."
            git clone -b "${target_branch}" "https://github.com/${remote}" "${path}"
        else
            local current_branch
            current_branch=$(git -C "${path}" branch --show-current 2>/dev/null || echo "DETACHED")

            if [[ "${current_branch}" != "${target_branch}" ]]; then
                # Check if the branch exists locally or on the remote
                if git -C "${path}" show-ref --verify --quiet "refs/heads/${target_branch}" 2>/dev/null; then
                    log "${dir}: switching from '${current_branch}' to '${target_branch}'..."
                    git -C "${path}" checkout "${target_branch}"
                elif git -C "${path}" ls-remote --exit-code --heads origin "${target_branch}" &>/dev/null; then
                    log "${dir}: fetching and checking out '${target_branch}' from origin..."
                    git -C "${path}" fetch origin "${target_branch}"
                    git -C "${path}" checkout -b "${target_branch}" "origin/${target_branch}"
                else
                    warn "${dir}: branch '${target_branch}' not found locally or on origin (currently on '${current_branch}')"
                    had_error=1
                    continue
                fi
            else
                log "${dir}: already on '${target_branch}'"
            fi

            # Pull latest from origin
            log "${dir}: pulling latest..."
            git -C "${path}" pull --ff-only origin "${target_branch}" 2>/dev/null \
                || warn "${dir}: pull failed (local changes or upstream diverged?)"
        fi
    done

    # Repos that stay on main — just ensure they're checked out
    for entry in "${REPOS_ON_MAIN[@]}"; do
        local dir="${entry%%|*}"
        local remote="${entry##*|}"
        local path="${REPO_ROOT}/${dir}"

        if [[ ! -d "${path}" ]]; then
            log "Cloning ${remote} → ${dir}..."
            git clone "https://github.com/${remote}" "${path}"
        else
            local branch
            branch=$(git -C "${path}" branch --show-current 2>/dev/null || echo "?")
            log "${dir}: present (on '${branch}'), pulling latest..."
            git -C "${path}" pull --ff-only 2>/dev/null \
                || warn "${dir}: pull failed (local changes or upstream diverged?)"
        fi
    done

    if (( had_error )); then
        die "Some repos could not be switched to '${NEXUS_BRANCH}'. Fix manually and re-run."
    fi

    log "All repos OK"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 0b: Prerequisites (tools + preset file)
# ──────────────────────────────────────────────────────────────────────────────
check_prerequisites() {
    step "Checking prerequisites"

    local missing=()
    for cmd in docker kind kubectl mage go; do
        command -v "$cmd" >/dev/null 2>&1 || missing+=("$cmd")
    done
    if (( ${#missing[@]} )); then
        die "Missing required tools: ${missing[*]}"
    fi

    if [[ ! -f "${SCORCH_DIR}/presets/${BASE_PRESET}" ]]; then
        die "Base preset not found: ${SCORCH_DIR}/presets/${BASE_PRESET}"
    fi

    log "All prerequisites OK  (tag: ${CUSTOM_TAG})"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 1: Build custom Docker images
# ──────────────────────────────────────────────────────────────────────────────
# Returns 0 (true) if the image for the given repo needs a rebuild.
# Compares the repo's latest commit timestamp against the Docker image creation time.
# If the image doesn't exist yet, always returns 0.
_image_needs_rebuild() {
    local image_local_name="$1"   # e.g. "tenancy-manager"
    local repo_dir="$2"           # e.g. "${REPO_ROOT}/orch-utils"

    # If image doesn't exist, must build
    if ! docker image inspect "${image_local_name}:${CUSTOM_TAG}" &>/dev/null; then
        return 0
    fi

    # Get image creation time as epoch
    local img_ts
    img_ts=$(docker inspect "${image_local_name}:${CUSTOM_TAG}" \
        --format '{{.Created}}' 2>/dev/null | xargs date +%s -d 2>/dev/null || echo 0)

    # Get repo's latest commit time as epoch
    local repo_ts
    repo_ts=$(git -C "${repo_dir}" log -1 --format='%ct' 2>/dev/null || echo 0)

    if (( repo_ts > img_ts )); then
        log "  ${image_local_name}: repo has newer commits ($(git -C "${repo_dir}" log -1 --format='%ci')) than image — rebuilding"
        return 0
    else
        log "  ${image_local_name}: image is current (no new commits since last build) — skipping"
        return 1
    fi
}

build_images() {
    step "Step 1: Building custom Docker images  (tag: ${CUSTOM_TAG})"

    local skip_ao=false
    if [[ "${DISABLE_AO_PROFILE:-}" == "true" || "${DISABLE_AO_PROFILE:-}" == "1" ]]; then
        log "DISABLE_AO_PROFILE=true — skipping app-orch image builds"
        skip_ao=true
    fi

    local skip_co=false
    if [[ "${DISABLE_CO_PROFILE:-}" == "true" || "${DISABLE_CO_PROFILE:-}" == "1" ]]; then
        log "DISABLE_CO_PROFILE=true — skipping cluster-orch image builds"
        skip_co=true
    fi

    local skip_o11y=false
    if [[ "${DISABLE_O11Y_PROFILE:-}" == "true" || "${DISABLE_O11Y_PROFILE:-}" == "1" ]]; then
        log "DISABLE_O11Y_PROFILE=true — skipping o11y image builds"
        skip_o11y=true
    fi

    # tenancy-manager
    if _image_needs_rebuild "tenancy-manager" "${REPO_ROOT}/orch-utils"; then
        log "Building tenancy-manager..."
        ( cd "${REPO_ROOT}/orch-utils/tenancy-manager" \
            && go mod vendor \
            && docker build -t "tenancy-manager:${CUSTOM_TAG}" . )
    fi

    if [[ "${skip_ao}" == "false" ]]; then
        # app-orch-tenant-controller
        if _image_needs_rebuild "app-orch-tenant-controller" "${REPO_ROOT}/app-orch-tenant-controller"; then
            log "Building app-orch-tenant-controller..."
            ( cd "${REPO_ROOT}/app-orch-tenant-controller" \
                && go mod vendor \
                && docker build -t "app-orch-tenant-controller:${CUSTOM_TAG}" -f build/Dockerfile . )
        fi

        # app-deployment-manager (controller + gateway)
        if _image_needs_rebuild "adm-controller" "${REPO_ROOT}/app-orch-deployment"; then
            log "Building adm-controller + adm-gateway..."
            ( cd "${REPO_ROOT}/app-orch-deployment/app-deployment-manager" \
                && go mod vendor \
                && docker build -t "adm-controller:${CUSTOM_TAG}" . \
                && docker build -t "adm-gateway:${CUSTOM_TAG}" -f build/Dockerfile.gateway . )
        fi
    fi

    # observability-tenant-controller
    if [[ "${skip_o11y}" == "false" ]]; then
        if _image_needs_rebuild "observability-tenant-controller" "${REPO_ROOT}/o11y-tenant-controller"; then
            log "Building observability-tenant-controller..."
            ( cd "${REPO_ROOT}/o11y-tenant-controller" \
                && go mod vendor \
                && docker build -t "observability-tenant-controller:${CUSTOM_TAG}" . )
        fi
    else
        log "Skipping O11Y image build: observability-tenant-controller"
    fi

    # keycloak-tenant-controller (in orch-utils/keycloak-tenant-controller)
    if _image_needs_rebuild "keycloak-tenant-controller" "${REPO_ROOT}/orch-utils"; then
        log "Building keycloak-tenant-controller..."
        ( cd "${REPO_ROOT}/orch-utils/keycloak-tenant-controller" \
            && go mod vendor \
            && docker build -t "keycloak-tenant-controller:${CUSTOM_TAG}" -f images/Dockerfile . )
    fi

    # infra-tenant-controller
    if _image_needs_rebuild "infra-tenant-controller" "${REPO_ROOT}/infra-core"; then
        log "Building infra-tenant-controller..."
        ( cd "${REPO_ROOT}/infra-core/tenant-controller" \
            && go mod vendor \
            && cp ../common.mk ../version.mk . \
            && docker build -t "infra-tenant-controller:${CUSTOM_TAG}" \
                --build-arg REPO_URL=local \
                --build-arg VERSION="${CUSTOM_TAG}" \
                --build-arg REVISION=local \
                --build-arg BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
                . )
    fi

    # cluster-manager
    if [[ "${skip_co}" == "false" ]]; then
        if _image_needs_rebuild "cluster-manager" "${REPO_ROOT}/cluster-manager"; then
            log "Building cluster-manager..."
            ( cd "${REPO_ROOT}/cluster-manager" \
                && go mod vendor \
                && docker build -t "cluster-manager:${CUSTOM_TAG}" -f deployment/images/Dockerfile.cluster-manager . )
        fi
    else
        log "Skipping CO image build: cluster-manager"
    fi

    # metadata-broker
    if _image_needs_rebuild "metadata-broker" "${REPO_ROOT}/orch-metadata-broker"; then
        log "Building metadata-broker..."
        ( cd "${REPO_ROOT}/orch-metadata-broker" \
            && go mod vendor \
            && docker build -t "metadata-broker:${CUSTOM_TAG}" -f build/Dockerfile . )
    fi

    # auth-service (in orch-utils/auth-service)
    if _image_needs_rebuild "auth-service" "${REPO_ROOT}/orch-utils"; then
        log "Building auth-service..."
        ( cd "${REPO_ROOT}/orch-utils/auth-service" \
            && go mod vendor \
            && docker build -t "auth-service:${CUSTOM_TAG}" . )
    fi

    # orch-cli (not a Docker image — native binary used for MT setup)
    build_orch_cli

    log "All images built successfully"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 1b: Tag images with the full registry path the charts expect
#
# The charts produce image refs like:
#   registry-rs.edgeorchestration.intel.com/edge-orch/common/tenancy-manager:tag
# We tag our locally-built images with that full name so that when kind loads
# them, kubelet finds them via imagePullPolicy: IfNotPresent.
# ──────────────────────────────────────────────────────────────────────────────
tag_images() {
    step "Step 1b: Tagging images with full registry paths"

    local skip_ao=false
    if [[ "${DISABLE_AO_PROFILE:-}" == "true" || "${DISABLE_AO_PROFILE:-}" == "1" ]]; then
        skip_ao=true
    fi

    local skip_co=false
    if [[ "${DISABLE_CO_PROFILE:-}" == "true" || "${DISABLE_CO_PROFILE:-}" == "1" ]]; then
        skip_co=true
    fi

    local skip_o11y=false
    if [[ "${DISABLE_O11Y_PROFILE:-}" == "true" || "${DISABLE_O11Y_PROFILE:-}" == "1" ]]; then
        skip_o11y=true
    fi

    for i in "${!IMAGE_NAMES[@]}"; do
        local local_name="${IMAGE_NAMES[$i]}"
        if [[ "${skip_ao}" == "true" ]] && \
           [[ "${local_name}" == "app-orch-tenant-controller" || "${local_name}" == "adm-controller" || "${local_name}" == "adm-gateway" ]]; then
            log "Skipping AO image tag: ${local_name}"
            continue
        fi
        if [[ "${skip_co}" == "true" ]] && \
           [[ "${local_name}" == "cluster-manager" ]]; then
            log "Skipping CO image tag: ${local_name}"
            continue
        fi
        if [[ "${skip_o11y}" == "true" ]] && \
           [[ "${local_name}" == "observability-tenant-controller" ]]; then
            log "Skipping O11Y image tag: ${local_name}"
            continue
        fi
        local reg_path="${IMAGE_PATHS[$i]}"
        local full_name="${REGISTRY_URL}/${reg_path}:${CUSTOM_TAG}"
        log "Tagging ${local_name}:${CUSTOM_TAG} → ${full_name}"
        docker tag "${local_name}:${CUSTOM_TAG}" "${full_name}"
    done

    log "All images tagged"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 2: Patch ArgoCD application templates with custom image TAG only
#
# We only set the image TAG for each component — the registry and repository
# stay exactly as upstream defines them. Our images are tagged with the full
# registry/repo path and loaded into kind, so imagePullPolicy: IfNotPresent
# picks them up without needing to change registry or repository values.
#
# We also wire up TENANT_MANAGER_URL / nexus-api-url for controllers that
# need to talk to the new tenancy-manager REST API.
# ──────────────────────────────────────────────────────────────────────────────
CUSTOM_DIR="${EMF_DIR}/argocd/applications/custom"

patch_argocd_templates() {
    step "Step 2: Patching ArgoCD application templates (tag-only)"

    # If SKIP_PATCH_STEP=1 OR the custom tag is already present in the tpl files,
    # the branch has already been patched — skip the destructive git reset.
    # This protects our working nexus-replacement branch from being overwritten.
    local already_patched=false
    if [[ "${SKIP_PATCH_STEP:-0}" == "1" ]]; then
        already_patched=true
        log "SKIP_PATCH_STEP=1 — skipping ArgoCD template patching"
    elif grep -q "${CUSTOM_TAG}" "${CUSTOM_DIR}/tenancy-manager.tpl" 2>/dev/null; then
        already_patched=true
        log "Custom tag '${CUSTOM_TAG}' already present in templates — skipping patch step"
        log "  (set SKIP_PATCH_STEP=0 to force re-patch)"
    fi

    if [[ "${already_patched}" == "true" ]]; then
        # Ensure current branch is pushed so ArgoCD can pull it
        local current_emf_branch
        current_emf_branch=$(git -C "${EMF_DIR}" branch --show-current 2>/dev/null)
        if [[ "${current_emf_branch}" != "${DEPLOY_REPO_BRANCH}" ]]; then
            warn "EMF is on '${current_emf_branch}' not '${DEPLOY_REPO_BRANCH}' — ArgoCD may pull wrong branch"
        fi
        git -C "${EMF_DIR}" push -u origin "${DEPLOY_REPO_BRANCH}" 2>/dev/null \
            || warn "EMF push failed (may already be up-to-date)"
        return 0
    fi

    # SAFETY: Never reset the working branch. Update tag in-place with sed
    # so the full commit history (mage changes, tpl overrides, etc.) is preserved.
    local current_emf_branch
    current_emf_branch=$(git -C "${EMF_DIR}" branch --show-current 2>/dev/null)
    if [[ "${current_emf_branch}" != "${DEPLOY_REPO_BRANCH}" ]]; then
        log "Switching EMF to branch '${DEPLOY_REPO_BRANCH}'..."
        git -C "${EMF_DIR}" checkout "${DEPLOY_REPO_BRANCH}" 2>/dev/null \
            || git -C "${EMF_DIR}" checkout -b "${DEPLOY_REPO_BRANCH}" origin/main
    fi

    # Detect previous tag so we can replace it in all tpl files.
    # Match the full tag including optional -HHMM time suffix (e.g. nexus-replacement-20260430-0546).
    local prev_tag
    prev_tag=$(grep -oP 'nexus-replacement-\d{8}(-\d{4})?' "${CUSTOM_DIR}/tenancy-manager.tpl" 2>/dev/null | head -1 || true)
    if [[ -n "${prev_tag}" && "${prev_tag}" != "${CUSTOM_TAG}" ]]; then
        log "Updating image tags: ${prev_tag} → ${CUSTOM_TAG}"
        find "${CUSTOM_DIR}" -name "*.tpl" -exec sed -i "s/${prev_tag}/${CUSTOM_TAG}/g" {} +
    fi

    log "All ArgoCD templates patched (tag-only)"

    # Commit and push the updated templates so ArgoCD can pull them
    log "Committing tag update to '${DEPLOY_REPO_BRANCH}' branch..."
    git -C "${EMF_DIR}" add argocd/applications/custom/
    if git -C "${EMF_DIR}" diff --cached --quiet; then
        log "No changes to commit (templates already up to date on branch)"
    else
        git -C "${EMF_DIR}" commit -m "nexus-replacement: update image tags to ${CUSTOM_TAG}"
    fi
    git -C "${EMF_DIR}" push -u origin "${DEPLOY_REPO_BRANCH}"
    log "Pushed to origin/${DEPLOY_REPO_BRANCH}"
}

# For backwards compatibility, keep preset as an alias
create_preset() {
    patch_argocd_templates
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 3a: Load images into kind
# ──────────────────────────────────────────────────────────────────────────────
load_images() {
    step "Loading custom images into kind"

    # Skip app-orch images if AO is disabled; skip cluster-orch images if CO is disabled
    local skip_ao=false
    if [[ "${DISABLE_AO_PROFILE:-}" == "true" || "${DISABLE_AO_PROFILE:-}" == "1" ]]; then
        skip_ao=true
    fi

    local skip_co=false
    if [[ "${DISABLE_CO_PROFILE:-}" == "true" || "${DISABLE_CO_PROFILE:-}" == "1" ]]; then
        skip_co=true
    fi

    local skip_o11y=false
    if [[ "${DISABLE_O11Y_PROFILE:-}" == "true" || "${DISABLE_O11Y_PROFILE:-}" == "1" ]]; then
        skip_o11y=true
    fi

    for i in "${!IMAGE_NAMES[@]}"; do
        local name="${IMAGE_NAMES[$i]}"
        if [[ "${skip_ao}" == "true" ]] && \
           [[ "${name}" == "app-orch-tenant-controller" || "${name}" == "adm-controller" || "${name}" == "adm-gateway" ]]; then
            log "Skipping AO image: ${name}"
            continue
        fi
        if [[ "${skip_co}" == "true" ]] && \
           [[ "${name}" == "cluster-manager" ]]; then
            log "Skipping CO image: ${name}"
            continue
        fi
        if [[ "${skip_o11y}" == "true" ]] && \
           [[ "${name}" == "observability-tenant-controller" ]]; then
            log "Skipping O11Y image: ${name}"
            continue
        fi
        local full_name="${REGISTRY_URL}/${IMAGE_PATHS[$i]}:${CUSTOM_TAG}"
        log "Loading ${full_name}..."
        # kind v0.17 cannot detect modern containerd snapshotter — use ctr import.
        if ! docker save "${full_name}" | docker exec -i kind-control-plane ctr -n=k8s.io images import - >/dev/null; then
            warn "ctr import failed for ${full_name}, falling back to kind load"
            kind load docker-image "${full_name}"
        fi
    done

    log "All images loaded"
}

# ──────────────────────────────────────────────────────────────────────────────
# Build orch-cli (used for multi-tenant setup)
# ──────────────────────────────────────────────────────────────────────────────
build_orch_cli() {
    step "Building orch-cli"
    if [[ -x "${ORCH_CLI}" ]]; then
        log "orch-cli already built at ${ORCH_CLI}"
        return 0
    fi
    ( cd "${REPO_ROOT}/orch-cli" && make build )
    log "orch-cli built successfully"
}

# ──────────────────────────────────────────────────────────────────────────────
# Create default multi-tenant setup using orch-cli
#
# Replaces the old `mage tenantUtils:createDefaultMtSetup` which used Nexus
# CRDs.  Uses orch-cli for:
#   - Org/project creation via the tenancy-manager REST API (port-forwarded)
#   - Keycloak user creation and group assignment via the Keycloak Admin API
#
# The old setup created:  sample-org, sample-project, project-admin user,
# edge-infra users (api-user, en-svc-account, onboarding-user,
# service-admin-api-user), and cluster-orch users (edge-op, edge-mgr).
# ──────────────────────────────────────────────────────────────────────────────
create_default_mt_setup() {
    local org="test-org"
    local project="test-project"
    local cli="${ORCH_CLI}"
    local pf_port=18080
    local tm_ep="http://localhost:${pf_port}"

    step "Creating default multi-tenant setup via orch-cli"

    # Ensure orch-cli is built
    if [[ ! -x "${cli}" ]]; then
        build_orch_cli
    fi

    # --- Determine ORCH_DOMAIN ---
    # Auto-detect from the orchestrator if not set.  The domain is stored in
    # the orch-configs configmap written by the mage deploy tooling.
    if [[ -z "${ORCH_DOMAIN:-}" ]]; then
        ORCH_DOMAIN=$(kubectl get configmap -n orch-gateway orchestrator-domain \
            -o jsonpath='{.data.orchestratorDomainName}' 2>/dev/null || true)
    fi
    if [[ -z "${ORCH_DOMAIN:-}" ]]; then
        # Fallback: derive from any Keycloak IngressRoute host
        ORCH_DOMAIN=$(kubectl get ingressroute -A -o json 2>/dev/null \
            | grep -oP 'keycloak\.\K[^`"]+' | head -1 || true)
    fi
    if [[ -z "${ORCH_DOMAIN:-}" ]]; then
        die "Cannot determine orchestrator domain. Set ORCH_DOMAIN and re-run."
    fi
    log "Using ORCH_DOMAIN=${ORCH_DOMAIN}"

    # --- Configure orch-cli endpoint ---
    local api_ep="https://api.${ORCH_DOMAIN}"
    log "Setting orch-cli api-endpoint to ${api_ep}"
    "${cli}" config set api-endpoint "${api_ep}"

    # --- Get admin password from Keycloak K8s secret ---
    log "Retrieving Keycloak admin password from K8s secret..."
    local admin_password
    admin_password="$(kubectl -n orch-platform get secret platform-keycloak \
        -o jsonpath='{.data.admin-password}' | base64 -d | tr -d '\n')"
    if [[ -z "${admin_password}" ]]; then
        die "Failed to retrieve admin password from platform-keycloak secret"
    fi

    # --- Login to Keycloak ---
    # orch-cli derives the Keycloak endpoint from the api-endpoint automatically.
    # Keycloak may not be ready immediately after deploy — retry up to 2 minutes.
    # The orchestrator info check may return non-200 in our POC — that's fine,
    # feature flags default to enabled for backward compatibility.
    log "Logging in to Keycloak as admin (will retry up to 2 minutes)..."
    local login_ok=false
    for login_attempt in $(seq 1 12); do
        if "${cli}" login admin "${admin_password}" 2>&1; then
            login_ok=true
            break
        fi
        log "  Login attempt ${login_attempt}/12 failed — retrying in 10s..."
        sleep 10
    done
    if [[ "${login_ok}" != "true" ]]; then
        die "Failed to login to Keycloak after 12 attempts"
    fi
    log "Keycloak login successful"

    # --- Fetch raw KC bearer token for TM curl calls ---
    # orch-cli injects the token automatically, but curl calls need it explicitly.
    # Use system-client (not admin-cli) — it includes realm_access.roles in the
    # JWT, which the TM auth middleware uses to recognise the KC 'admin' role.
    local kc_base_url="https://keycloak.${ORCH_DOMAIN}"
    local tm_token
    tm_token=$(curl -sk -X POST \
        "${kc_base_url}/realms/master/protocol/openid-connect/token" \
        -d "client_id=system-client" \
        -d "username=admin" \
        -d "password=${admin_password}" \
        -d "grant_type=password" \
        -d "scope=openid" \
        | grep -oP '"access_token"\s*:\s*"\K[^"]+')
    if [[ -z "${tm_token}" ]]; then
        die "Failed to acquire KC bearer token for TM API calls"
    fi
    log "KC bearer token acquired (len=${#tm_token})"

    # --- Start port-forward to tenancy-manager ---
    # We port-forward directly to the tenancy-manager service because the API
    # gateway may not have routing configured for the new tenancy-manager.
    log "Starting port-forward to tenancy-manager..."
    kubectl -n "${TM_NAMESPACE}" port-forward "svc/${TM_SERVICE_NAME}" "${pf_port}:${TM_PORT}" &
    local pf_pid=$!
    trap "kill ${pf_pid} 2>/dev/null || true" EXIT
    sleep 2

    # --- Create org ---
    log "Creating organization '${org}'..."
    "${cli}" create organization "${org}" \
        --api-endpoint "${tm_ep}" 2>&1 \
        || die "Failed to create org '${org}'"

    # Wait for org to be ready (controllers process it) and extract UID.
    # The tenancy-manager reports STATUS_INDICATION_IDLE once all controllers
    # have finished.
    log "Waiting for org '${org}' to become ready..."
    local org_uid=""
    for attempt in $(seq 1 30); do
        local output
        output=$("${cli}" get organization "${org}" \
            --api-endpoint "${tm_ep}" 2>&1 || true)
        org_uid=$(echo "${output}" | grep "^UID:" | sed 's/^UID:[[:space:]]*//')
        local org_status
        org_status=$(echo "${output}" | grep "^Status:" | head -1 | sed 's/^Status:[[:space:]]*//')

        if [[ -n "${org_uid}" && "${org_uid}" != "N/A" && "${org_status}" == *"IDLE"* ]]; then
            log "Org '${org}' ready (UID: ${org_uid})"
            break
        fi
        log "  Attempt ${attempt}/30 — status: ${org_status:-unknown}"
        sleep 10
    done

    if [[ -z "${org_uid}" || "${org_uid}" == "N/A" ]]; then
        die "Timed out waiting for org '${org}' to become ready"
    fi

    # --- Create project admin user ---
    local pa_user="${org}-admin"
    log "Creating project admin user '${pa_user}'..."
    "${cli}" create user "${pa_user}" \
        --email "${pa_user}@${org}.com" \
        --password="${ORCH_DEFAULT_PASSWORD}" 2>&1 \
        || warn "User '${pa_user}' may already exist"

    log "Adding '${pa_user}' to Project-Manager-Group..."
    "${cli}" set user "${pa_user}" \
        --add-group "${org_uid}_Project-Manager-Group" 2>&1 \
        || warn "Failed to add '${pa_user}' to group (group may not exist yet)"

    # --- Create project ---
    # Use curl directly so we can pass ?org= (orch-cli doesn't support --org).
    log "Creating project '${project}' under org '${org}'..."
    local create_resp
    create_resp=$(curl -s -X PUT "${tm_ep}/v1/projects/${project}?org=${org}" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${tm_token}" \
        -d "{\"description\": \"${project}\"}")
    if echo "${create_resp}" | grep -q '"error"'; then
        die "Failed to create project '${project}': ${create_resp}"
    fi
    log "  Response: ${create_resp}"

    # Wait for project to be ready and extract UID.
    log "Waiting for project '${project}' to become ready..."
    local project_uid=""
    for attempt in $(seq 1 30); do
        local proj_json
        proj_json=$(curl -s -H "Authorization: Bearer ${tm_token}" "${tm_ep}/v1/projects/${project}" || true)
        project_uid=$(echo "${proj_json}" | grep -oP '"uID"\s*:\s*"\K[^"]+' | head -1)
        local proj_status
        proj_status=$(echo "${proj_json}" | grep -oP '"statusIndicator"\s*:\s*"\K[^"]+' | head -1)

        if [[ -n "${project_uid}" && "${project_uid}" != "N/A" && "${proj_status}" == *"IDLE"* ]]; then
            log "Project '${project}' ready (UID: ${project_uid})"
            break
        fi
        log "  Attempt ${attempt}/30 — status: ${proj_status:-unknown}"
        sleep 10
    done

    if [[ -z "${project_uid}" || "${project_uid}" == "N/A" ]]; then
        die "Timed out waiting for project '${project}' to become ready"
    fi

    # --- Create edge infra users ---
    # Format: "username|group1,group2,..."
    local -a ei_users=(
        "${project}-api-user|${project_uid}_Host-Manager-Group"
        "${project}-en-svc-account|${project_uid}_Edge-Node-M2M-Service-Account"
        "${project}-onboarding-user|${project_uid}_Edge-Onboarding-Group"
        "${project}-service-admin-api-user|${project_uid}_Host-Manager-Group,service-admin-group"
    )

    log "Creating edge infra users..."
    for entry in "${ei_users[@]}"; do
        local user="${entry%%|*}"
        local groups="${entry##*|}"

        "${cli}" create user "${user}" \
            --email "${user}@${org}.com" \
            --password="${ORCH_DEFAULT_PASSWORD}" 2>&1 \
            || warn "User '${user}' may already exist"

        IFS=',' read -ra group_list <<< "${groups}"
        for group in "${group_list[@]}"; do
            "${cli}" set user "${user}" --add-group "${group}" 2>&1 \
                || warn "Failed to add '${user}' to group '${group}'"
        done
    done

    # --- Create cluster orchestration users ---
    local -a co_users=(
        "${project}-edge-op|${project_uid}_Edge-Operator-Group"
        "${project}-edge-mgr|${project_uid}_Edge-Manager-Group,${project_uid}_Edge-Operator-Group,${project_uid}_Host-Manager-Group,${project_uid}_Edge-Onboarding-Group,${project_uid}_Edge-Node-M2M-Service-Account"
    )

    log "Creating cluster orchestration users..."
    for entry in "${co_users[@]}"; do
        local user="${entry%%|*}"
        local groups="${entry##*|}"

        "${cli}" create user "${user}" \
            --email "${user}@${org}.com" \
            --password="${ORCH_DEFAULT_PASSWORD}" 2>&1 \
            || warn "User '${user}' may already exist"

        IFS=',' read -ra group_list <<< "${groups}"
        for group in "${group_list[@]}"; do
            "${cli}" set user "${user}" --add-group "${group}" 2>&1 \
                || warn "Failed to add '${user}' to group '${group}'"
        done
    done

    # --- Assign project member realm role via Keycloak Admin API ---
    # orch-cli cannot assign realm roles; the old mage code assigned
    # {orgUID}_{projectUID}_m to every user except the project admin.
    # We port-forward to the internal Keycloak service to avoid corporate proxy.
    log "Assigning project member realm role via Keycloak Admin API..."

    local kc_pf_port=18443
    kubectl -n orch-platform port-forward svc/platform-keycloak "${kc_pf_port}:8080" &
    local kc_pf_pid=$!
    sleep 2

    local kc_url="http://localhost:${kc_pf_port}"
    local kc_realm="master"

    # Get admin token
    local kc_token
    kc_token=$(curl -s -X POST "${kc_url}/realms/${kc_realm}/protocol/openid-connect/token" \
        -d "client_id=admin-cli" \
        -d "username=admin" \
        -d "password=${admin_password}" \
        -d "grant_type=password" | grep -oP '"access_token"\s*:\s*"\K[^"]+')
    if [[ -z "${kc_token}" ]]; then
        warn "Failed to get Keycloak admin token — skipping realm role assignment"
        kill "${kc_pf_pid}" 2>/dev/null || true
    else
        # The member role is {kcOrgUID}_{projectUID}_m — but the KTC may use a
        # different org UID than tenancy-manager reports.  Search by project UID
        # suffix to find the actual role name.
        log "Searching for project member realm role (*_${project_uid}_m)..."

        local member_role="" role_id=""
        local search_json
        search_json=$(curl -s -H "Authorization: Bearer ${kc_token}" \
            "${kc_url}/admin/realms/master/roles?search=${project_uid}_m&max=10")
        # Find the role whose name ends with _{projectUID}_m
        member_role=$(echo "${search_json}" | grep -oP '"name"\s*:\s*"\K[^"]*_'"${project_uid}"'_m' | head -1)
        role_id=$(echo "${search_json}" | grep -B1 "\"${member_role}\"" | grep -oP '"id"\s*:\s*"\K[^"]+' | head -1)

        if [[ -z "${member_role}" || -z "${role_id}" ]]; then
            warn "Realm role '*_${project_uid}_m' not found — controllers may not have created it yet"
        else
            log "Found realm role '${member_role}' (id: ${role_id})"

            # All users that need the member role (including project admin for safety)
            local -a member_users=(
                "${org}-admin"
                "${project}-api-user"
                "${project}-en-svc-account"
                "${project}-onboarding-user"
                "${project}-service-admin-api-user"
                "${project}-edge-op"
                "${project}-edge-mgr"
            )

            for mu in "${member_users[@]}"; do
                # Look up user ID
                local user_json
                user_json=$(curl -s -H "Authorization: Bearer ${kc_token}" \
                    "${kc_url}/admin/realms/${kc_realm}/users?username=${mu}&exact=true")
                local user_id
                user_id=$(echo "${user_json}" | grep -oP '"id"\s*:\s*"\K[^"]+' | head -1)

                if [[ -z "${user_id}" ]]; then
                    warn "User '${mu}' not found in Keycloak — skipping role assignment"
                    continue
                fi

                # Assign the realm role
                local assign_resp
                assign_resp=$(curl -s -o /dev/null -w "%{http_code}" \
                    -X POST -H "Authorization: Bearer ${kc_token}" \
                    -H "Content-Type: application/json" \
                    -d "[{\"id\":\"${role_id}\",\"name\":\"${member_role}\"}]" \
                    "${kc_url}/admin/realms/${kc_realm}/users/${user_id}/role-mappings/realm")

                if [[ "${assign_resp}" == "204" || "${assign_resp}" == "200" ]]; then
                    log "  Assigned '${member_role}' to '${mu}'"
                else
                    warn "  Failed to assign '${member_role}' to '${mu}' (HTTP ${assign_resp})"
                fi
            done
        fi

        # --- Assign project-specific service roles ---
        # Keycloak Tenant Controller creates project-scoped roles (alrt-r, alrt-rw,
        # cl-r, cl-rw, cl-tpl-r, cl-tpl-rw, etc.) but doesn't assign them to users.
        # We assign the roles needed for the alerting-monitor OPA policy and
        # cluster-manager access.  The inject-project-id Traefik middleware uses the
        # project UUID header so these role names match at authorization time.
        log "Assigning project-specific service roles (alrt-r, cl-r, cl-rw, ...) ..."

        local svc_role_suffixes=("alrt-r" "alrt-rw" "cl-r" "cl-rw" "cl-tpl-r" "cl-tpl-rw")
        local svc_roles_json="[]"

        # Build a JSON array of all matching role objects
        local all_roles_json
        all_roles_json=$(curl -s -H "Authorization: Bearer ${kc_token}" \
            "${kc_url}/admin/realms/${kc_realm}/roles?max=500")

        svc_roles_json=$(echo "${all_roles_json}" | python3 -c "
import json, sys
roles = json.load(sys.stdin)
project_uid = '${project_uid}'
wanted = [f'{project_uid}_{s}' for s in ['alrt-r','alrt-rw','cl-r','cl-rw','cl-tpl-r','cl-tpl-rw']]
res = [r for r in roles if r['name'] in wanted]
print(json.dumps(res))
")

        if [[ "${svc_roles_json}" == "[]" || -z "${svc_roles_json}" ]]; then
            warn "No project service roles found — controllers may not have created them yet"
        else
            local roles_count
            roles_count=$(echo "${svc_roles_json}" | python3 -c "import json,sys; print(len(json.load(sys.stdin)))")
            log "Found ${roles_count} service roles to assign"

            for mu in "${member_users[@]}"; do
                local user_json2
                user_json2=$(curl -s -H "Authorization: Bearer ${kc_token}" \
                    "${kc_url}/admin/realms/${kc_realm}/users?username=${mu}&exact=true")
                local user_id2
                user_id2=$(echo "${user_json2}" | grep -oP '"id"\s*:\s*"\K[^"]+' | head -1)
                if [[ -z "${user_id2}" ]]; then
                    continue
                fi
                local svc_resp
                svc_resp=$(curl -s -o /dev/null -w "%{http_code}" \
                    -X POST -H "Authorization: Bearer ${kc_token}" \
                    -H "Content-Type: application/json" \
                    -d "${svc_roles_json}" \
                    "${kc_url}/admin/realms/${kc_realm}/users/${user_id2}/role-mappings/realm")
                if [[ "${svc_resp}" == "204" || "${svc_resp}" == "200" ]]; then
                    log "  Assigned service roles to '${mu}'"
                else
                    warn "  Failed to assign service roles to '${mu}' (HTTP ${svc_resp})"
                fi
            done
        fi

        kill "${kc_pf_pid}" 2>/dev/null || true
    fi

    # --- Clean up ---
    kill "${pf_pid}" 2>/dev/null || true
    trap - EXIT

    log "Default multi-tenant setup complete"
    log "  Org: ${org} (UID: ${org_uid})"
    log "  Project: ${project} (UID: ${project_uid})"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 3: Deploy the orchestrator
# ──────────────────────────────────────────────────────────────────────────────
deploy_orchestrator() {
    step "Step 3: Deploying orchestrator"

    # 3a. Clean up defunct kind cluster if it exists without running containers.
    # 'kind create cluster' fails if a cluster entry already exists.
    if kind get clusters 2>/dev/null | grep -q "^kind$"; then
        if ! docker ps --filter name=kind-control-plane --filter status=running -q 2>/dev/null | grep -q .; then
            log "Defunct kind cluster detected (no running containers) — deleting stale entry..."
            kind delete cluster 2>/dev/null || true
        else
            log "Existing live kind cluster found — will reuse (delete manually with 'kind delete cluster' for a clean deploy)"
        fi
    fi

    # 3a. Start the deployment (creates kind cluster + submits ArgoCD apps)
    # Use the base preset — our image overrides are in the patched custom/*.tpl files
    local preset="${SCORCH_DIR}/presets/${BASE_PRESET}"
    log "Running mage deploy:kindPreset ${preset}..."
    ( cd "${EMF_DIR}" && mage -v deploy:kindPreset "${preset}" )

    # 3b. Load images into kind (pods will be in ImagePullBackOff until this completes;
    #     they recover automatically because imagePullPolicy: IfNotPresent)
    load_images

    # 3c. Wait for everything to sync and become healthy
    log "Waiting for deployment to complete..."
    ( cd "${EMF_DIR}" && mage deploy:waitUntilComplete )

    # 3d. tenancy-manager Service — now included in the Helm chart (service.yaml
    # was added to charts/tenancy-manager in the utils_nexus_replacement branch).
    # ArgoCD will create it automatically; nothing to do here.
    log "tenancy-manager Service is managed by the Helm chart — no manual creation needed"

    # 3e. Create IngressRoute to route tenancy API to our tenancy-manager
    # The old nexus-api-gw has a catch-all PathPrefix('/v') IngressRoute that
    # grabs all /v1/* requests.  We create more-specific routes for the
    # tenancy endpoints so Traefik sends them to our tenancy-manager instead.
    # Traefik prefers longer prefixes, so /v1/orgs and /v1/projects win over /v.
    log "Ensuring IngressRoutes for tenancy-manager..."
    local orch_domain
    orch_domain=$(kubectl get configmap -n orch-gateway orchestrator-domain \
        -o jsonpath='{.data.orchestratorDomainName}' 2>/dev/null)
    local api_host="api.${orch_domain}"

    # Use PathRegexp to match only the tenancy endpoints, NOT sub-resource
    # paths like /v1/projects/{id}/appdeployment (those go to other services).
    kubectl apply -f - <<INGRESSEOF
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: tenancy-manager-http
  namespace: orch-gateway
spec:
  entryPoints:
    - websecure
  routes:
    - kind: Rule
      match: "Host(\`${api_host}\`) && PathRegexp(\`^/v1/orgs(/[^/]+(/status)?)?\$\`)"
      middlewares:
        - name: validate-jwt
        - name: secure-headers
      services:
        - kind: Service
          name: tenancy-manager
          namespace: orch-iam
          port: 8080
    - kind: Rule
      match: "Host(\`${api_host}\`) && PathRegexp(\`^/v1/projects(/[^/]+(/status)?)?\$\`)"
      middlewares:
        - name: validate-jwt
        - name: secure-headers
      services:
        - kind: Service
          name: tenancy-manager
          namespace: orch-iam
          port: 8080
    - kind: Rule
      match: "Host(\`${api_host}\`) && PathRegexp(\`^/v1/folders(/[^/]+)?\$\`)"
      middlewares:
        - name: validate-jwt
        - name: secure-headers
      services:
        - kind: Service
          name: tenancy-manager
          namespace: orch-iam
          port: 8080
INGRESSEOF
    log "IngressRoute applied: tenancy-manager-http -> ${TM_SERVICE_NAME}.${TM_NAMESPACE}:${TM_PORT}"

    # 3e-ii. Create IngressRoutes to bypass nexus-api-gw for services that have
    # no rest-proxy sidecar.  These services relied entirely on the nexus-api-gw
    # API remapping layer; now that we've replaced Nexus we route directly to them
    # with Traefik path rewriting to strip the /projects/{projectName}/ segment.
    #
    # Services with rest-proxy sidecars (app-deployment-api, app-resource-manager,
    # app-orch-catalog) already handle project resolution via project-service-url.
    log "Creating IngressRoutes for non-rest-proxy services (bypass nexus-api-gw)..."

    # Generic middleware: strip /v{N}/projects/{name}/ → /v{N}/
    # Safe to reuse across all services since it's only applied per-route.
    kubectl apply -f - <<'MWEOF'
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: strip-project-prefix
  namespace: orch-gateway
spec:
  replacePathRegex:
    regex: "^/(v[0-9]+)/projects/[^/]+/(.*)"
    replacement: "/$1/$2"
MWEOF

    # NOTE: resolve-project-id is a ForwardAuth middleware that calls
    # auth-service /resolveproject to validate the JWT and resolve the project
    # name in the URL to its UUID via Tenant Manager. Traefik copies the
    # Activeprojectid / ActiveProjectID response headers into the upstream
    # request via authResponseHeaders so cluster-manager, alerting-monitor, and
    # metadata-broker receive the UUID they require.
    # X-Forwarded-Uri carries the original URL (before path rewrites), so the
    # project name is always available to auth-service regardless of ordering.
    kubectl apply -f - <<RPIDEOF
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: resolve-project-id
  namespace: orch-gateway
spec:
  forwardAuth:
    address: http://auth-service.orch-gateway.svc:8080/resolveproject
    authResponseHeaders:
      - Activeprojectid
      - ActiveProjectID
RPIDEOF
    log "  resolve-project-id middleware applied"

    # cluster-manager (orch-cluster:8080)
    # Handles: /v2/projects/{p}/clusters*, /v2/projects/{p}/templates*
    kubectl apply -f - <<CMEOF
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: cluster-manager-http
  namespace: orch-gateway
spec:
  entryPoints:
    - websecure
  routes:
    - kind: Rule
      match: "Host(\`${api_host}\`) && PathRegexp(\`^/v2/projects/[^/]+/(clusters|templates)\`)"
      priority: 50
      middlewares:
        - name: validate-jwt
        - name: resolve-project-id
        - name: strip-project-prefix
        - name: secure-headers
      services:
        - kind: Service
          name: cluster-manager
          namespace: orch-cluster
          port: 8080
CMEOF
    log "  cluster-manager IngressRoute applied"

    # alerting-monitor (orch-infra:8080)
    # Handles: /v1/projects/{p}/alerts*
    # Service expects /api/v1/alerts* (api/ prefix, same pattern as mps/rps)
    kubectl apply -f - <<'AMMWEOF'
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: rewrite-alerting-monitor
  namespace: orch-gateway
spec:
  replacePathRegex:
    regex: "^/v1/projects/[^/]+/(alerts.*)"
    replacement: "/api/v1/$1"
AMMWEOF

    kubectl apply -f - <<AMEOF
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: alerting-monitor-http
  namespace: orch-gateway
spec:
  entryPoints:
    - websecure
  routes:
    - kind: Rule
      match: "Host(\`${api_host}\`) && PathRegexp(\`^/v1/projects/[^/]+/alerts\`)"
      priority: 50
      middlewares:
        - name: validate-jwt
        - name: resolve-project-id
        - name: rewrite-alerting-monitor
        - name: secure-headers
      services:
        - kind: Service
          name: alerting-monitor
          namespace: orch-infra
          port: 8080
AMEOF
    log "  alerting-monitor IngressRoute applied"

    # metadata-broker (orch-ui:9988)
    # Handles: /v1/projects/{p}/metadata
    # Service is a gRPC-gateway that serves at /metadata.orchestrator.apis/v1/metadata
    kubectl apply -f - <<'MBMWEOF'
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: rewrite-metadata-broker
  namespace: orch-gateway
spec:
  replacePathRegex:
    regex: "^/v1/projects/[^/]+/metadata(.*)"
    replacement: "/metadata.orchestrator.apis/v1/metadata$1"
MBMWEOF

    kubectl apply -f - <<MBEOF2
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: metadata-broker-http
  namespace: orch-gateway
spec:
  entryPoints:
    - websecure
  routes:
    - kind: Rule
      match: "Host(\`${api_host}\`) && PathRegexp(\`^/v1/projects/[^/]+/metadata\`)"
      priority: 50
      middlewares:
        - name: validate-jwt
        - name: resolve-project-id
        - name: rewrite-metadata-broker
        - name: secure-headers
      services:
        - kind: Service
          name: metadata-broker-orch-metadata-broker-rest
          namespace: orch-ui
          port: 9988
MBEOF2
    log "  metadata-broker IngressRoute applied"

    # mps (orch-infra:3000) — Device Management Toolkit
    # Handles: /v1/projects/{p}/dm/amt/generalSettings/{guid}
    #          /v1/projects/{p}/dm/devices/{guid}
    # Rewrites: strip projects/{p}/dm/ → api/
    kubectl apply -f - <<'MPSMWEOF'
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: rewrite-mps
  namespace: orch-gateway
spec:
  replacePathRegex:
    regex: "^/v1/projects/[^/]+/dm/(.*)"
    replacement: "/api/v1/$1"
MPSMWEOF

    kubectl apply -f - <<MPSEOF
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: mps-http
  namespace: orch-gateway
spec:
  entryPoints:
    - websecure
  routes:
    - kind: Rule
      match: "Host(\`${api_host}\`) && PathRegexp(\`^/v1/projects/[^/]+/dm/(amt/generalSettings|devices)\`)"
      priority: 50
      middlewares:
        - name: validate-jwt
        - name: resolve-project-id
        - name: rewrite-mps
        - name: secure-headers
      services:
        - kind: Service
          name: mps
          namespace: orch-infra
          port: 3000
MPSEOF
    log "  mps IngressRoute applied"

    # rps (orch-infra:8081) — Device Management Toolkit
    # Handles: /v1/projects/{p}/dm/amt/admin/domains*
    # Rewrites: strip projects/{p}/dm/amt/ → api/
    kubectl apply -f - <<'RPSMWEOF'
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: rewrite-rps
  namespace: orch-gateway
spec:
  replacePathRegex:
    regex: "^/v1/projects/[^/]+/dm/amt/(.*)"
    replacement: "/api/v1/$1"
RPSMWEOF

    kubectl apply -f - <<RPSEOF
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: rps-http
  namespace: orch-gateway
spec:
  entryPoints:
    - websecure
  routes:
    - kind: Rule
      match: "Host(\`${api_host}\`) && PathRegexp(\`^/v1/projects/[^/]+/dm/amt/admin\`)"
      priority: 50
      middlewares:
        - name: validate-jwt
        - name: resolve-project-id
        - name: rewrite-rps
        - name: secure-headers
      services:
        - kind: Service
          name: rps
          namespace: orch-infra
          port: 8081
RPSEOF
    log "  rps IngressRoute applied"

    # app-deployment-api path rewrites: the nexus-api-gw remapped certain
    # external paths by inserting "appdeployment/" before forwarding to the
    # rest-proxy.  These services HAVE a rest-proxy sidecar, so the project
    # prefix stays — we just need the path segment insertion.
    #   /v1/projects/{p}/summary/*      → /v1/projects/{p}/appdeployment/summary/*
    #   /v1/projects/{p}/deployments/*  → /v1/projects/{p}/appdeployment/deployments/*
    #   /v1/projects/{p}/ui_extensions  → /v1/projects/{p}/appdeployment/ui_extensions
    kubectl apply -f - <<'ADMWEOF'
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: insert-appdeployment-prefix
  namespace: orch-gateway
spec:
  replacePathRegex:
    regex: "^(/v1/projects/[^/]+)/(summary|deployments|ui_extensions)(.*)"
    replacement: "$1/appdeployment/$2$3"
ADMWEOF

    kubectl apply -f - <<ADEOF
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: app-deployment-rewrite-http
  namespace: orch-gateway
spec:
  entryPoints:
    - websecure
  routes:
    - kind: Rule
      match: "Host(\`${api_host}\`) && PathRegexp(\`^/v1/projects/[^/]+/(summary|deployments|ui_extensions)\`)"
      priority: 50
      middlewares:
        - name: insert-appdeployment-prefix
        - name: validate-jwt
        - name: secure-headers
      services:
        - kind: Service
          name: app-deployment-api-rest-proxy
          namespace: orch-app
          port: 8081
ADEOF
    log "  app-deployment-api path rewrite IngressRoute applied"

    # 3f. Post-deploy: /etc/hosts, router, vault, certs.
    # Mirrors the standard kind deploy flow:
    #   mage gen:hostfileTraefik | sudo tee -a /etc/hosts
    #   mage Router:Stop Router:Start
    #   mage Vault:Unseal
    #   mage gen:orchCA deploy:orchCA vault:keys
    log "Updating /etc/hosts with Traefik ingress IP..."
    ( cd "${EMF_DIR}" && mage gen:hostfileTraefik ) | sudo tee -a /etc/hosts > /dev/null \
        && log "/etc/hosts updated" \
        || warn "Failed to update /etc/hosts — run manually: cd ${EMF_DIR} && mage gen:hostfileTraefik | sudo tee -a /etc/hosts"

    log "Starting router..."
    ( cd "${EMF_DIR}" && mage router:stop router:start )

    log "Unsealing Vault..."
    ( cd "${EMF_DIR}" && mage Vault:Unseal ) \
        || warn "Vault unseal failed — may already be unsealed or vault-keys.json missing"

    log "Deploying orchestrator CA and setting vault keys..."
    ( cd "${EMF_DIR}" && mage gen:orchCA deploy:orchCA vault:keys ) \
        || warn "orchCA/vault:keys step failed — check Vault status"

    log "Creating default multi-tenant setup..."
    create_default_mt_setup

    log "Deployment complete"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 4: Verify
# ──────────────────────────────────────────────────────────────────────────────
verify_deployment() {
    step "Step 4: Verification"
    local failed=0

    local skip_ao=false
    [[ "${DISABLE_AO_PROFILE:-}" == "true" || "${DISABLE_AO_PROFILE:-}" == "1" ]] && skip_ao=true
    local skip_co=false
    [[ "${DISABLE_CO_PROFILE:-}" == "true" || "${DISABLE_CO_PROFILE:-}" == "1" ]] && skip_co=true
    local skip_o11y=false
    [[ "${DISABLE_O11Y_PROFILE:-}" == "true" || "${DISABLE_O11Y_PROFILE:-}" == "1" ]] && skip_o11y=true

    # --- Pod status ---
    # Verified against live deployment on 2026-04-10. See ARGOCD-NOTES.md.
    log "Checking pod status..."
    local -a check_deployments=(
        "orch-iam|tenancy-manager|deployment"
        "orch-platform|keycloak-tenant-controller-set|statefulset"
        "orch-app|app-orch-tenant-controller|deployment|ao"
        "orch-app|app-deployment-manager|deployment|ao"
        "orch-app|app-deployment-api|deployment|ao"
        "orch-infra|tenant-controller|deployment"
        "orch-cluster|cluster-manager|deployment|co"
        "orch-platform|observability-tenant-controller|deployment|o11y"
        "orch-ui|metadata-broker-orch-metadata-broker|deployment"
    )

    for entry in "${check_deployments[@]}"; do
        local ns name kind profile_tag
        ns="${entry%%|*}"; local rest="${entry#*|}"
        name="${rest%%|*}"; rest="${rest#*|}"
        kind="${rest%%|*}"; profile_tag="${rest#*|}"
        # profile_tag may equal kind if there's no 4th field
        [[ "${profile_tag}" == "${kind}" ]] && profile_tag=""
        if [[ "${profile_tag}" == "ao" && "${skip_ao}" == "true" ]]; then
            log "  SKIPPED (AO disabled): ${ns}/${name}"
            continue
        fi
        if [[ "${profile_tag}" == "co" && "${skip_co}" == "true" ]]; then
            log "  SKIPPED (CO disabled): ${ns}/${name}"
            continue
        fi
        if [[ "${profile_tag}" == "o11y" && "${skip_o11y}" == "true" ]]; then
            log "  SKIPPED (O11Y disabled): ${ns}/${name}"
            continue
        fi
        local pods
        pods=$(kubectl -n "${ns}" get "${kind}" "${name}" --no-headers 2>/dev/null || true)
        if [[ -z "${pods}" ]]; then
            warn "${kind}/${name} not found in ${ns}"
            failed=1
        else
            log "  ${ns}/${name}: $(kubectl -n "${ns}" get "${kind}" "${name}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo '?') ready"
        fi
    done

    # --- Image verification ---
    echo ""
    log "Checking deployed images contain tag '${CUSTOM_TAG}'..."
    # Format: "namespace|resource_name|resource_kind|container_name|expected_repo_path|profile_tag"
    # profile_tag: ao=app-orch, co=cluster-orch, o11y=observability, empty=always
    local -a image_checks=(
        "orch-iam|tenancy-manager|deployment|tenancy-manager|common/tenancy-manager|"
        "orch-platform|keycloak-tenant-controller-set|statefulset|keycloak-tenant-controller-pod|common/keycloak-tenant-controller|"
        "orch-app|app-orch-tenant-controller|deployment|config-provisioner|app/app-orch-tenant-controller|ao"
        "orch-app|app-deployment-manager|deployment|controller|app/adm-controller|ao"
        "orch-app|app-deployment-api|deployment|app-deployment-api|app/adm-gateway|ao"
        "orch-infra|tenant-controller|deployment|tenant-controller|infra/tenant-controller|"
        "orch-cluster|cluster-manager|deployment|cluster-manager|cluster/cluster-manager|co"
        "orch-platform|observability-tenant-controller|deployment|observability-tenant-controller|o11y/observability-tenant-controller|o11y"
        "orch-ui|metadata-broker-orch-metadata-broker|deployment|orch-metadata-broker|orch-ui/metadata-broker|"
    )
    for entry in "${image_checks[@]}"; do
        IFS='|' read -r ns name kind container expected_path profile_tag <<< "${entry}"
        if [[ "${profile_tag}" == "ao" && "${skip_ao}" == "true" ]]; then
            log "  SKIPPED (AO disabled): ${name}/${container}"
            continue
        fi
        if [[ "${profile_tag}" == "co" && "${skip_co}" == "true" ]]; then
            log "  SKIPPED (CO disabled): ${name}/${container}"
            continue
        fi
        if [[ "${profile_tag}" == "o11y" && "${skip_o11y}" == "true" ]]; then
            log "  SKIPPED (O11Y disabled): ${name}/${container}"
            continue
        fi
        local img expected_img
        expected_img="${REGISTRY_URL}/${expected_path}:${CUSTOM_TAG}"
        img=$(kubectl -n "${ns}" get "${kind}" "${name}" \
            -o jsonpath="{.spec.template.spec.containers[?(@.name==\"${container}\")].image}" 2>/dev/null || echo "NOT FOUND")
        if [[ "${img}" == "${expected_img}" ]]; then
            log "  ${name}/${container}: ${img}"
        elif [[ "${img}" == *"${CUSTOM_TAG}"* ]]; then
            warn "  ${name}/${container}: ${img}  (has custom tag but unexpected path; expected ${expected_img})"
        else
            warn "  ${name}/${container}: ${img}  (expected ${expected_img})"
            failed=1
        fi
    done

    # --- Service check ---
    echo ""
    log "Checking tenancy-manager Service..."
    if kubectl -n "${TM_NAMESPACE}" get svc "${TM_SERVICE_NAME}" &>/dev/null; then
        kubectl -n "${TM_NAMESPACE}" get svc "${TM_SERVICE_NAME}"
    else
        warn "tenancy-manager Service not found in ${TM_NAMESPACE}!"
        failed=1
    fi

    # --- TENANT_MANAGER_URL check for app-deployment-manager ---
    if [[ "${skip_ao}" == "false" ]]; then
        echo ""
        log "Checking TENANT_MANAGER_URL in app-deployment-manager..."
        local tm_env
        tm_env=$(kubectl -n orch-app get deployment app-deployment-manager \
            -o jsonpath='{.spec.template.spec.containers[?(@.name=="controller")].env[?(@.name=="TENANT_MANAGER_URL")].value}' 2>/dev/null || true)
        if [[ -n "${tm_env}" ]]; then
            log "  TENANT_MANAGER_URL=${tm_env}"
        else
            warn "  TENANT_MANAGER_URL not set — app-deployment-manager will use hardcoded defaults"
            warn "  Poller default:           http://tenancy-manager.orch-iam:8080"
            warn "  Project resolver default: http://tenancy-manager.orch-platform.svc:8080"
        fi

        # --- nexus-api-url / project-service-url checks (all four rest-proxy services) ---
        local nexus_checks=(
            "app-deployment-api:orch-app"
            "app-resource-manager:orch-app"
            "apiv2:orch-infra"
            "app-orch-catalog:orch-app"
        )
        local entry dep ns
        for entry in "${nexus_checks[@]}"; do
            dep="${entry%%:*}"
            ns="${entry##*:}"
            log "Checking project-resolver URL in ${dep} (${ns})..."
            local svc_args
            svc_args=$(kubectl -n "${ns}" get deployment "${dep}" \
                -o json 2>/dev/null | grep -oE '"nexus-api-url=[^"]*"|"nexusAPIURL=[^"]*"|"project-service-url=[^"]*"' | head -1 || true)
            if [[ -n "${svc_args}" ]]; then
                log "  ${dep}: ${svc_args}"
            else
                warn "  Could not verify project-resolver URL in ${dep}"
            fi
        done
    else
        log "  SKIPPED (AO disabled): TENANT_MANAGER_URL and rest-proxy URL checks"
    fi

    echo ""
    if (( failed )); then
        warn "Some checks failed — review output above"
    else
        log "All verification checks passed"
    fi
}

# ──────────────────────────────────────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────────────────────────────────────
main() {
    local cmd="${1:-all}"

    echo -e "${CYAN}╔══════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║  Nexus Replacement — Deploy & Test               ║${NC}"
    echo -e "${CYAN}║  Tag: ${CUSTOM_TAG}$(printf '%*s' $((24 - ${#CUSTOM_TAG})) '')║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════╝${NC}"

    case "${cmd}" in
        all)
            ensure_repos
            check_prerequisites
            build_images
            tag_images
            patch_argocd_templates
            deploy_orchestrator
            verify_deployment
            ;;
        build)
            ensure_repos
            check_prerequisites
            build_images
            tag_images
            ;;
        preset|patch)
            ensure_repos
            check_prerequisites
            patch_argocd_templates
            ;;
        tag)
            tag_images
            ;;
        deploy)
            deploy_orchestrator
            ;;
        repos)
            ensure_repos
            ;;
        load)
            load_images
            ;;
        setup)
            create_default_mt_setup
            ;;
        verify)
            verify_deployment
            ;;
        *)
            echo "Usage: $0 {all|repos|build|tag|patch|deploy|load|setup|verify}"
            echo ""
            echo "Steps:"
            echo "  all     Run the full workflow (default)"
            echo "  repos   Clone missing repos and switch to '${NEXUS_BRANCH}' branch"
            echo "  build   Build custom Docker images"
            echo "  tag     Tag images with full registry paths (for kind loading)"
            echo "  patch   Patch ArgoCD app templates with custom image tags"
            echo "  deploy  Deploy orchestrator (assumes build + tag + patch done)"
            echo "  load    Load images into kind (standalone)"
            echo "  setup   Create default MT setup (org, project, users) via orch-cli"
            echo "  verify  Run verification checks"
            echo ""
            echo "Environment variables:"
            echo "  CUSTOM_TAG           Image tag (default: nexus-replacement-YYYYMMDD)"
            echo "  BASE_PRESET          Base scorch preset filename (default: dev-internal-coder-autocert.yaml)"
            echo "  DISABLE_AO_PROFILE   Set to 'true' to skip App Orchestration images and ArgoCD apps"
            echo "  DISABLE_CO_PROFILE   Set to 'true' to skip Cluster Orchestration images and ArgoCD apps"
            echo "  DISABLE_O11Y_PROFILE Set to 'true' to skip Observability tenant-controller image and ArgoCD apps"
            exit 1
            ;;
    esac
}

# Allow this file to be sourced as a library (functions only) by setting
# NR_LIB_ONLY=1 before sourcing. In that mode, do not invoke main automatically.
if [[ -z "${NR_LIB_ONLY:-}" ]]; then
    main "$@"
fi
