#!/bin/bash
# deploy-nexus-replacement-helmfile.sh — Build, deploy, and verify the nexus
# replacement on a HELMFILE-based orchestrator (k3s + helmfile flow under
# /edge-manageability-framework/{pre-orch,post-orch}).
#
# Scope: onprem-eim profile only.  This script does NOT build or deploy
# App-Orch (AO), Cluster-Orch (CO), or Observability (O11Y) components because
# the helmfile profile excludes them.  Only the 5 EIM-relevant components that
# touch the tenancy plane are built here.
#
# Workflow:
#   1. ensure_repos      clone/checkout *_nexus_replacement branches
#   2. build_images      build 5 custom images
#   3. tag_images        tag with full registry path
#   4. run_pre_orch      k3s + openebs + metallb + namespaces + secrets
#   5. load_images_k3s   load into k3s containerd
#   6. run_post_orch     helmfile sync (tenancy v2 always on)
#   7. run_mt_setup      org/project/users via orch-cli + Keycloak
#   8. verify_helmfile   releases, image tags, tenancy wiring, pod health
#
# Usage:
#   ./deploy-nexus-replacement-helmfile.sh [all|repos|build|tag|load|pre|deploy|setup|verify|uninstall]
#
# Note: the argocd flow companion script deploy-nexus-replacement.sh and the
# Go mage targets (mage/tenancy_rest.go, mage/tenant_utils.go) are NOT used by
# this helmfile path. Both flows hit the same tenancy-manager REST API; this
# script uses orch-cli + curl from bash instead of mage.

set -o pipefail

# ──────────────────────────────────────────────────────────────────────────────
# Configuration
# ──────────────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# If the script is executed from inside the edge-manageability-framework clone
# (a second copy lives there for convenience), treat its parent as REPO_ROOT
# so the sibling repos already present at /home/seu/* are reused instead of
# being re-cloned as untracked children of the EMF working tree.
if [[ "$(basename "${SCRIPT_DIR}")" == "edge-manageability-framework" ]]; then
    REPO_ROOT="${REPO_ROOT:-$(dirname "${SCRIPT_DIR}")}"
    HELMFILE_EMF_DIR="${HELMFILE_EMF_DIR:-${SCRIPT_DIR}}"
else
    REPO_ROOT="${REPO_ROOT:-${SCRIPT_DIR}}"
    HELMFILE_EMF_DIR="${HELMFILE_EMF_DIR:-${REPO_ROOT}/edge-manageability-framework}"
fi
PRE_ORCH_DIR="${HELMFILE_EMF_DIR}/pre-orch"
POST_ORCH_DIR="${HELMFILE_EMF_DIR}/post-orch"

ORCH_UTILS_PATH="${ORCH_UTILS_PATH:-${REPO_ROOT}/orch-utils}"
INFRA_CORE_PATH="${INFRA_CORE_PATH:-${REPO_ROOT}/infra-core}"
ORCH_METADATA_BROKER_PATH="${ORCH_METADATA_BROKER_PATH:-${REPO_ROOT}/orch-metadata-broker}"
ORCH_CLI_PATH="${ORCH_CLI_PATH:-${REPO_ROOT}/orch-cli}"

CUSTOM_TAG="${CUSTOM_TAG:-nexus-replacement-$(date +%Y%m%d)}"
HELMFILE_ENV="${HELMFILE_ENV:-onprem-vpro}"

TM_SERVICE_NAME="tenancy-manager"
TM_NAMESPACE="orch-iam"
TM_PORT=8080
REGISTRY_URL="registry-rs.edgeorchestration.intel.com/edge-orch"

export ORCH_DEFAULT_PASSWORD="${ORCH_DEFAULT_PASSWORD:-ChangeMeOn1stLogin!}"
export ASDF_GOLANG_VERSION="${ASDF_GOLANG_VERSION:-1.26.1}"

ORCH_CLI="${ORCH_CLI_PATH}/build/_output/orch-cli"

# ──────────────────────────────────────────────────────────────────────────────
# EIM-only image set
# ──────────────────────────────────────────────────────────────────────────────
IMAGE_NAMES=(
    "tenancy-manager"
    "keycloak-tenant-controller"
    "infra-tenant-controller"
    "metadata-broker"
    "auth-service"
)
IMAGE_PATHS=(
    "common/tenancy-manager"
    "common/keycloak-tenant-controller"
    "infra/tenant-controller"
    "orch-ui/metadata-broker"
    "common/auth-service"
)
IMAGE_REPO_DIRS=(
    "${ORCH_UTILS_PATH}"
    "${ORCH_UTILS_PATH}"
    "${INFRA_CORE_PATH}"
    "${ORCH_METADATA_BROKER_PATH}"
    "${ORCH_UTILS_PATH}"
)

NEXUS_REPOS=(
    "orch-utils|open-edge-platform/orch-utils|utils_nexus_replacement"
    "infra-core|open-edge-platform/infra-core|infra-nexus-replacement"
    "orch-metadata-broker|open-edge-platform/orch-metadata-broker|mdb-nexus-replacement"
    "orch-library|open-edge-platform/orch-library|gg/lib-nexus-replacement"
)
REPOS_ON_MAIN=(
    "orch-cli|open-edge-platform/orch-cli"
)

# ──────────────────────────────────────────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[$(date +%H:%M:%S)]${NC} $*"; }
warn() { echo -e "${YELLOW}[$(date +%H:%M:%S)] WARN:${NC} $*"; }
err()  { echo -e "${RED}[$(date +%H:%M:%S)] ERROR:${NC} $*" >&2; }
die()  { err "$@"; exit 1; }
step() { echo -e "\n${CYAN}━━━ $* ━━━${NC}"; }

# ──────────────────────────────────────────────────────────────────────────────
# Step 0a: Repos
# ──────────────────────────────────────────────────────────────────────────────
ensure_repos() {
    step "Ensuring repos on *_nexus_replacement branches"

    if [[ "${SKIP_REPOS_CHECK:-0}" == "1" ]]; then
        log "SKIP_REPOS_CHECK=1 — reporting branches only"
        for entry in "${NEXUS_REPOS[@]}"; do
            local dir="${entry%%|*}" path="${REPO_ROOT}/${dir}"
            if [[ -d "${path}" ]]; then
                log "  ${dir}: $(git -C "${path}" branch --show-current 2>/dev/null || echo DETACHED)"
            else
                warn "  ${dir}: NOT FOUND at ${path}"
            fi
        done
        return 0
    fi

    local had_error=0
    for entry in "${NEXUS_REPOS[@]}"; do
        local dir remote target_branch path
        dir=$(echo "${entry}" | cut -d'|' -f1)
        remote=$(echo "${entry}" | cut -d'|' -f2)
        target_branch=$(echo "${entry}" | cut -d'|' -f3)
        path="${REPO_ROOT}/${dir}"

        if [[ ! -d "${path}" ]]; then
            log "Cloning ${remote} → ${dir} (branch: ${target_branch})..."
            git clone -b "${target_branch}" "https://github.com/${remote}" "${path}" || { had_error=1; continue; }
        else
            local current_branch
            current_branch=$(git -C "${path}" branch --show-current 2>/dev/null || echo "DETACHED")
            if [[ "${current_branch}" != "${target_branch}" ]]; then
                if git -C "${path}" show-ref --verify --quiet "refs/heads/${target_branch}" 2>/dev/null; then
                    log "${dir}: switching '${current_branch}' → '${target_branch}'..."
                    git -C "${path}" checkout "${target_branch}"
                elif git -C "${path}" ls-remote --exit-code --heads origin "${target_branch}" &>/dev/null; then
                    log "${dir}: fetching '${target_branch}' from origin..."
                    git -C "${path}" fetch origin "${target_branch}"
                    git -C "${path}" checkout -b "${target_branch}" "origin/${target_branch}"
                else
                    warn "${dir}: branch '${target_branch}' not found"
                    had_error=1
                    continue
                fi
            else
                log "${dir}: already on '${target_branch}'"
            fi
            git -C "${path}" pull --ff-only origin "${target_branch}" 2>/dev/null \
                || warn "${dir}: pull failed"
        fi
    done

    for entry in "${REPOS_ON_MAIN[@]}"; do
        local dir="${entry%%|*}" remote="${entry##*|}" path="${REPO_ROOT}/${dir}"
        if [[ ! -d "${path}" ]]; then
            log "Cloning ${remote} → ${dir}..."
            git clone "https://github.com/${remote}" "${path}" \
                || { warn "${dir}: clone failed — will retry in build_orch_cli"; had_error=1; }
        else
            log "${dir}: present, pulling latest..."
            git -C "${path}" pull --ff-only 2>/dev/null || warn "${dir}: pull failed"
        fi
    done

    (( had_error )) && die "Some repos could not be set up — fix manually and re-run"
    log "All repos OK"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 0b: Prerequisites
# ──────────────────────────────────────────────────────────────────────────────
check_prerequisites() {
    step "Checking prerequisites"
    local missing=()
    for cmd in docker helmfile helm kubectl curl jq python3 go git; do
        command -v "$cmd" >/dev/null 2>&1 || missing+=("$cmd")
    done
    (( ${#missing[@]} == 0 )) || die "Missing tools: ${missing[*]}"

    [[ -d "${HELMFILE_EMF_DIR}" ]]                       || die "HELMFILE_EMF_DIR not found: ${HELMFILE_EMF_DIR}"
    [[ -f "${PRE_ORCH_DIR}/pre-orch.sh" ]]               || die "Missing ${PRE_ORCH_DIR}/pre-orch.sh"
    [[ -f "${POST_ORCH_DIR}/post-orch-deploy.sh" ]]      || die "Missing ${POST_ORCH_DIR}/post-orch-deploy.sh"
    [[ -d "${ORCH_UTILS_PATH}/charts/tenancy-manager" ]] || die "Missing ${ORCH_UTILS_PATH}/charts/tenancy-manager — run 'repos' first"

    log "All prerequisites OK  (tag: ${CUSTOM_TAG}, profile: ${HELMFILE_ENV})"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 1: Build the 5 EIM custom images
# ──────────────────────────────────────────────────────────────────────────────
_image_needs_rebuild() {
    local local_name="$1" repo_dir="$2"
    docker image inspect "${local_name}:${CUSTOM_TAG}" &>/dev/null || return 0
    local img_ts repo_ts
    img_ts=$(docker inspect "${local_name}:${CUSTOM_TAG}" --format '{{.Created}}' 2>/dev/null \
        | xargs date +%s -d 2>/dev/null || echo 0)
    repo_ts=$(git -C "${repo_dir}" log -1 --format='%ct' 2>/dev/null || echo 0)
    if (( repo_ts > img_ts )); then
        log "  ${local_name}: repo newer than image — rebuilding"
        return 0
    fi
    log "  ${local_name}: image current — skipping"
    return 1
}

build_images() {
    step "Building 5 EIM custom Docker images  (tag: ${CUSTOM_TAG})"

    if _image_needs_rebuild "tenancy-manager" "${ORCH_UTILS_PATH}"; then
        log "Building tenancy-manager..."
        ( cd "${ORCH_UTILS_PATH}/tenancy-manager" \
            && go mod vendor \
            && docker build -t "tenancy-manager:${CUSTOM_TAG}" . )
    fi

    if _image_needs_rebuild "keycloak-tenant-controller" "${ORCH_UTILS_PATH}"; then
        log "Building keycloak-tenant-controller..."
        ( cd "${ORCH_UTILS_PATH}/keycloak-tenant-controller" \
            && go mod vendor \
            && docker build -t "keycloak-tenant-controller:${CUSTOM_TAG}" -f images/Dockerfile . )
    fi

    if _image_needs_rebuild "infra-tenant-controller" "${INFRA_CORE_PATH}"; then
        log "Building infra-tenant-controller..."
        ( cd "${INFRA_CORE_PATH}/tenant-controller" \
            && go mod vendor \
            && cp ../common.mk ../version.mk . \
            && docker build -t "infra-tenant-controller:${CUSTOM_TAG}" \
                --build-arg REPO_URL=local \
                --build-arg VERSION="${CUSTOM_TAG}" \
                --build-arg REVISION=local \
                --build-arg BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
                . )
    fi

    if _image_needs_rebuild "metadata-broker" "${ORCH_METADATA_BROKER_PATH}"; then
        log "Building metadata-broker..."
        ( cd "${ORCH_METADATA_BROKER_PATH}" \
            && go mod vendor \
            && docker build -t "metadata-broker:${CUSTOM_TAG}" -f build/Dockerfile . )
    fi

    if _image_needs_rebuild "auth-service" "${ORCH_UTILS_PATH}"; then
        log "Building auth-service..."
        ( cd "${ORCH_UTILS_PATH}/auth-service" \
            && go mod vendor \
            && docker build -t "auth-service:${CUSTOM_TAG}" . )
    fi

    build_orch_cli
    log "All images built successfully"
}

build_orch_cli() {
    if [[ -x "${ORCH_CLI}" ]]; then
        log "orch-cli already built at ${ORCH_CLI}"
        return 0
    fi
    if [[ ! -d "${ORCH_CLI_PATH}" ]]; then
        log "Cloning orch-cli into ${ORCH_CLI_PATH}..."
        git clone https://github.com/open-edge-platform/orch-cli "${ORCH_CLI_PATH}" \
            || die "Failed to clone orch-cli — check network/proxy"
    fi
    log "Building orch-cli..."
    ( cd "${ORCH_CLI_PATH}" && make build ) || die "orch-cli build failed"
    [[ -x "${ORCH_CLI}" ]] || die "orch-cli build did not produce ${ORCH_CLI}"
    log "orch-cli built"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 2: Tag images with full registry path the charts pull from
# ──────────────────────────────────────────────────────────────────────────────
tag_images() {
    step "Tagging images with full registry paths"
    for i in "${!IMAGE_NAMES[@]}"; do
        local local_name="${IMAGE_NAMES[$i]}"
        local full_name="${REGISTRY_URL}/${IMAGE_PATHS[$i]}:${CUSTOM_TAG}"
        if ! docker image inspect "${local_name}:${CUSTOM_TAG}" &>/dev/null; then
            warn "Image ${local_name}:${CUSTOM_TAG} not found — skipping tag"
            continue
        fi
        log "Tagging ${local_name}:${CUSTOM_TAG} → ${full_name}"
        docker tag "${local_name}:${CUSTOM_TAG}" "${full_name}"
    done
    log "All images tagged"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 3: Load images into k3s containerd
# ──────────────────────────────────────────────────────────────────────────────
load_images_k3s() {
    step "Loading custom images into k3s containerd"

    local k3s_ctr=""
    if command -v k3s >/dev/null 2>&1; then
        k3s_ctr="sudo k3s ctr"
    elif sudo test -S /run/k3s/containerd/containerd.sock 2>/dev/null; then
        k3s_ctr="sudo ctr -a /run/k3s/containerd/containerd.sock"
    else
        die "k3s not found and /run/k3s/containerd/containerd.sock missing — has pre-orch run?"
    fi

    for i in "${!IMAGE_NAMES[@]}"; do
        local full_name="${REGISTRY_URL}/${IMAGE_PATHS[$i]}:${CUSTOM_TAG}"
        if ! docker image inspect "${full_name}" &>/dev/null; then
            warn "Image not found locally: ${full_name}"
            continue
        fi
        log "Loading ${full_name} into k3s containerd..."
        if ! docker save "${full_name}" | ${k3s_ctr} -n k8s.io images import - >/dev/null 2>&1; then
            warn "stream import failed for ${full_name}, retrying via tmpfile..."
            local tmp; tmp=$(mktemp --suffix=.tar)
            docker save -o "${tmp}" "${full_name}"
            ${k3s_ctr} -n k8s.io images import "${tmp}" || warn "import still failed for ${full_name}"
            rm -f "${tmp}"
        fi
    done

    log "All images loaded into k3s containerd"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 4: pre-orch
# ──────────────────────────────────────────────────────────────────────────────
run_pre_orch() {
    step "Running pre-orch.sh install"
    ( cd "${PRE_ORCH_DIR}" && ./pre-orch.sh install )
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 5: post-orch with tenancy v2 (always on)
# ──────────────────────────────────────────────────────────────────────────────
run_post_orch() {
    step "Running post-orch-deploy.sh install (tenancy v2; TENANCY_V2_TAG=${CUSTOM_TAG})"

    export TENANCY_V2_TAG="${CUSTOM_TAG}"
    export ORCH_UTILS_PATH

    load_images_k3s

    log "Invoking post-orch-deploy.sh with EMF_HELMFILE_ENV=${EMF_HELMFILE_ENV}"
    ( cd "${POST_ORCH_DIR}" && EMF_HELMFILE_ENV=onprem-vpro ./post-orch-deploy.sh install )
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 5b: Uninstall current deployment
# ──────────────────────────────────────────────────────────────────────────────
run_uninstall() {
    step "Uninstalling current helmfile deployment"
    export EMF_HELMFILE_ENV="${HELMFILE_ENV}"
    ( cd "${POST_ORCH_DIR}" && EMF_HELMFILE_ENV="${EMF_HELMFILE_ENV}" ./post-orch-deploy.sh uninstall ) || warn "post-orch uninstall returned non-zero"

    log "Removing residual orch-* namespaces..."
    local ns_list=( orch-iam orch-platform orch-gateway orch-infra orch-ui orch-secret \
                    orch-database orch-boots cattle-system )
    for ns in "${ns_list[@]}"; do
        if kubectl get ns "${ns}" --no-headers 2>/dev/null | grep -q .; then
            log "  deleting namespace ${ns}..."
            kubectl delete ns "${ns}" --wait=false --ignore-not-found 2>/dev/null || true
        fi
    done

    log "Cleaning up cluster-scoped CRDs (best-effort)..."
    kubectl get crd -o name 2>/dev/null \
        | grep -E '(cnpg|postgresql|cert-manager|traefik|kyverno|external-secrets|edge-orchestrator\.intel\.com|nexus\.com)' \
        | xargs -r kubectl delete --wait=false --ignore-not-found 2>/dev/null || true

    log "Uninstall complete (namespace deletions may continue in background)"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 6: MT setup — org/project/users via orch-cli + Keycloak Admin API
# ──────────────────────────────────────────────────────────────────────────────
install_orch_ca() {
    # Trust the cluster's Traefik CA so orch-cli (and curl) can talk TLS
    # to keycloak / api / web endpoints without --insecure.
    log "Installing orch-ca.crt into system trust store..."
    local tmp_crt
    tmp_crt="$(mktemp --suffix=.crt)"
    kubectl -n orch-gateway get secret tls-orch \
        -o jsonpath='{.data.tls\.crt}' | base64 -d > "${tmp_crt}" \
        || die "Failed to read tls-orch secret from orch-gateway"
    [[ -s "${tmp_crt}" ]] || die "tls-orch certificate is empty"
    sudo cp -f "${tmp_crt}" /usr/local/share/ca-certificates/orch-ca.crt \
        || die "Failed to copy CA cert (need passwordless sudo)"
    sudo update-ca-certificates -f >/dev/null \
        || die "update-ca-certificates failed"
    rm -f "${tmp_crt}"
    log "  orch CA trusted system-wide"
}

run_mt_setup() {
    local org="test-org"
    local project="test-project"
    local cli="${ORCH_CLI}"
    local pf_port=18080
    local tm_ep="http://localhost:${pf_port}"

    step "Creating default multi-tenant setup via orch-cli"

    [[ -x "${cli}" ]] || build_orch_cli

    if [[ -z "${ORCH_DOMAIN:-}" ]]; then
        ORCH_DOMAIN=$(kubectl get configmap -n orch-gateway orchestrator-domain \
            -o jsonpath='{.data.orchestratorDomainName}' 2>/dev/null || true)
    fi
    if [[ -z "${ORCH_DOMAIN:-}" ]]; then
        ORCH_DOMAIN=$(kubectl get ingressroute -A -o json 2>/dev/null \
            | grep -oP 'keycloak\.\K[^`"]+' | head -1 || true)
    fi
    [[ -n "${ORCH_DOMAIN}" ]] || die "Cannot determine orchestrator domain — set ORCH_DOMAIN and re-run"
    log "Using ORCH_DOMAIN=${ORCH_DOMAIN}"

    install_orch_ca

    "${cli}" config set api-endpoint "https://api.${ORCH_DOMAIN}"

    log "Retrieving Keycloak admin password..."
    local admin_password
    admin_password=$(kubectl -n orch-platform get secret platform-keycloak \
        -o jsonpath='{.data.admin-password}' | base64 -d | tr -d '\n')
    [[ -n "${admin_password}" ]] || die "Failed to fetch admin password from platform-keycloak"

    log "Logging in to Keycloak (retry up to 2 minutes)..."
    local login_ok=false
    for i in $(seq 1 12); do
        if "${cli}" login admin "${admin_password}" 2>&1; then login_ok=true; break; fi
        log "  Login attempt ${i}/12 failed — retry in 10s..."
        sleep 10
    done
    [[ "${login_ok}" == "true" ]] || die "Keycloak login failed"

    local kc_base="https://keycloak.${ORCH_DOMAIN}"
    local tm_token
    tm_token=$(curl -sk -X POST "${kc_base}/realms/master/protocol/openid-connect/token" \
        --data-urlencode "client_id=system-client" \
        --data-urlencode "username=admin" \
        --data-urlencode "password=${admin_password}" \
        --data-urlencode "grant_type=password" \
        --data-urlencode "scope=openid" \
        | grep -oP '"access_token"\s*:\s*"\K[^"]+')
    [[ -n "${tm_token}" ]] || die "Failed to acquire KC bearer token"

    log "Starting port-forward to tenancy-manager..."
    kubectl -n "${TM_NAMESPACE}" port-forward "svc/${TM_SERVICE_NAME}" "${pf_port}:${TM_PORT}" &
    local pf_pid=$!
    trap "kill ${pf_pid} 2>/dev/null || true" EXIT
    sleep 2

    log "Creating organization '${org}'..."
    "${cli}" create organization "${org}" --api-endpoint "${tm_ep}" 2>&1 \
        || die "Failed to create org '${org}'"

    log "Waiting for org '${org}' to become IDLE..."
    local org_uid="" org_status=""
    for i in $(seq 1 30); do
        local out
        out=$("${cli}" get organization "${org}" --api-endpoint "${tm_ep}" 2>&1 || true)
        org_uid=$(echo "${out}"   | grep '^UID:'    | sed 's/^UID:[[:space:]]*//')
        org_status=$(echo "${out}" | grep '^Status:' | head -1 | sed 's/^Status:[[:space:]]*//')
        if [[ -n "${org_uid}" && "${org_uid}" != "N/A" && "${org_status}" == *IDLE* ]]; then
            log "Org '${org}' ready (UID: ${org_uid})"; break
        fi
        log "  Attempt ${i}/30 — status: ${org_status:-unknown}"
        sleep 10
    done
    [[ -n "${org_uid}" && "${org_uid}" != "N/A" ]] || die "Org timeout"

    local pa_user="${org}-admin"
    log "Creating project admin user '${pa_user}'..."
    "${cli}" create user "${pa_user}" --email "${pa_user}@${org}.com" \
        --password="${ORCH_DEFAULT_PASSWORD}" 2>&1 || warn "User may exist"
    "${cli}" set user "${pa_user}" --add-group "${org_uid}_Project-Manager-Group" 2>&1 \
        || warn "Failed to add ${pa_user} to Project-Manager-Group"

    log "Creating project '${project}' under org '${org}'..."
    local create_resp
    create_resp=$(curl -s -X PUT "${tm_ep}/v1/projects/${project}?org=${org}" \
        -H "Content-Type: application/json" -H "Authorization: Bearer ${tm_token}" \
        -d "{\"description\": \"${project}\"}")
    echo "${create_resp}" | grep -q '"error"' && die "Project create failed: ${create_resp}"

    log "Waiting for project '${project}' to become IDLE..."
    local project_uid=""
    for i in $(seq 1 30); do
        local pj proj_status
        pj=$(curl -s -H "Authorization: Bearer ${tm_token}" "${tm_ep}/v1/projects/${project}" || true)
        project_uid=$(echo "${pj}"   | grep -oP '"uID"\s*:\s*"\K[^"]+'             | head -1)
        proj_status=$(echo "${pj}"  | grep -oP '"statusIndicator"\s*:\s*"\K[^"]+' | head -1)
        if [[ -n "${project_uid}" && "${project_uid}" != "N/A" && "${proj_status}" == *IDLE* ]]; then
            log "Project '${project}' ready (UID: ${project_uid})"; break
        fi
        log "  Attempt ${i}/30 — status: ${proj_status:-unknown}"
        sleep 10
    done
    [[ -n "${project_uid}" && "${project_uid}" != "N/A" ]] || die "Project timeout"

    # Edge-Infra users only (no AO/CO/O11Y groups in this profile)
    local -a ei_users=(
        "${project}-api-user|${project_uid}_Host-Manager-Group"
        "${project}-en-svc-account|${project_uid}_Edge-Node-M2M-Service-Account"
        "${project}-onboarding-user|${project_uid}_Edge-Onboarding-Group"
        "${project}-service-admin-api-user|${project_uid}_Host-Manager-Group,service-admin-group"
    )
    log "Creating edge-infra users..."
    for entry in "${ei_users[@]}"; do
        local user="${entry%%|*}" groups="${entry##*|}"
        "${cli}" create user "${user}" --email "${user}@${org}.com" \
            --password="${ORCH_DEFAULT_PASSWORD}" 2>&1 || warn "User ${user} may exist"
        IFS=',' read -ra group_list <<< "${groups}"
        for g in "${group_list[@]}"; do
            "${cli}" set user "${user}" --add-group "${g}" 2>&1 || warn "Failed to add ${user} to ${g}"
        done
    done

    log "Assigning project member realm role via Keycloak Admin API..."
    local kc_pf_port=18443
    kubectl -n orch-platform port-forward svc/platform-keycloak "${kc_pf_port}:8080" &
    local kc_pf_pid=$!
    sleep 2
    local kc_url="http://localhost:${kc_pf_port}"
    local kc_token
    kc_token=$(curl -s -X POST "${kc_url}/realms/master/protocol/openid-connect/token" \
        --data-urlencode "client_id=admin-cli" \
        --data-urlencode "username=admin" \
        --data-urlencode "password=${admin_password}" \
        --data-urlencode "grant_type=password" \
        | grep -oP '"access_token"\s*:\s*"\K[^"]+')
    if [[ -z "${kc_token}" ]]; then
        warn "Failed to get Keycloak admin token — skipping realm role assignment"
    else
        local search_json member_role role_id
        search_json=$(curl -s -H "Authorization: Bearer ${kc_token}" \
            "${kc_url}/admin/realms/master/roles?search=${project_uid}_m&max=10")
        member_role=$(echo "${search_json}" | grep -oP '"name"\s*:\s*"\K[^"]*_'"${project_uid}"'_m' | head -1)
        role_id=$(echo "${search_json}" | grep -B1 "\"${member_role}\"" | grep -oP '"id"\s*:\s*"\K[^"]+' | head -1)
        if [[ -z "${member_role}" || -z "${role_id}" ]]; then
            warn "Realm role '*_${project_uid}_m' not found yet"
        else
            log "Found realm role '${member_role}' (id: ${role_id})"
            local -a member_users=(
                "${org}-admin"
                "${project}-api-user"
                "${project}-en-svc-account"
                "${project}-onboarding-user"
                "${project}-service-admin-api-user"
            )
            for mu in "${member_users[@]}"; do
                local uj uid
                uj=$(curl -s -H "Authorization: Bearer ${kc_token}" \
                    "${kc_url}/admin/realms/master/users?username=${mu}&exact=true")
                uid=$(echo "${uj}" | grep -oP '"id"\s*:\s*"\K[^"]+' | head -1)
                [[ -z "${uid}" ]] && { warn "User ${mu} not found in KC"; continue; }
                local rc
                rc=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
                    -H "Authorization: Bearer ${kc_token}" -H "Content-Type: application/json" \
                    -d "[{\"id\":\"${role_id}\",\"name\":\"${member_role}\"}]" \
                    "${kc_url}/admin/realms/master/users/${uid}/role-mappings/realm")
                if [[ "${rc}" == "204" || "${rc}" == "200" ]]; then
                    log "  Assigned '${member_role}' to '${mu}'"
                else
                    warn "  Failed to assign '${member_role}' to '${mu}' (HTTP ${rc})"
                fi
            done
        fi
        kill "${kc_pf_pid}" 2>/dev/null || true
    fi

    kill "${pf_pid}" 2>/dev/null || true
    trap - EXIT

    log "MT setup complete"
    log "  Org:     ${org} (UID: ${org_uid})"
    log "  Project: ${project} (UID: ${project_uid})"
}

# ──────────────────────────────────────────────────────────────────────────────
# Step 7: Verify
# ──────────────────────────────────────────────────────────────────────────────
verify_helmfile() {
    step "Verifying helmfile deployment"
    local failed=0

    log "Checking helm releases..."
    # Releases that are always expected.
    local releases=( tenancy-manager keycloak-tenant-controller nexus-api-gw \
                     traefik-extra-objects infra-core )
    # auth-service and metadata-broker are only enabled in the EIM profile;
    # the vPro profile explicitly disables them.
    if [[ "${HELMFILE_ENV}" == "onprem-eim" ]]; then
        releases+=( auth-service metadata-broker )
    fi
    for r in "${releases[@]}"; do
        local status
        status=$(helm list -A -f "^${r}\$" --no-headers 2>/dev/null | awk '{print $8}' | head -1)
        if [[ "${status}" == "deployed" ]]; then
            log "  ${r}: deployed"
        else
            warn "  ${r}: status='${status:-not found}'"
            failed=1
        fi
    done

    log "Checking image tags contain '${CUSTOM_TAG}'..."
    local -a checks=(
        "orch-iam|tenancy-manager|deployment|tenancy-manager"
        "orch-platform|keycloak-tenant-controller-set|statefulset|keycloak-tenant-controller-pod"
        "orch-infra|tenant-controller|deployment|tenant-controller"
    )
    if [[ "${HELMFILE_ENV}" == "onprem-eim" ]]; then
        checks+=( "orch-gateway|auth-service|deployment|auth-service" )
    fi
    for entry in "${checks[@]}"; do
        IFS='|' read -r ns name kind container <<< "${entry}"
        local img
        img=$(kubectl -n "${ns}" get "${kind}" "${name}" \
            -o jsonpath="{.spec.template.spec.containers[?(@.name==\"${container}\")].image}" 2>/dev/null || echo "NOT_FOUND")
        if [[ "${img}" == *"${CUSTOM_TAG}"* ]]; then
            log "  ${ns}/${name}/${container}: ${img}"
        else
            warn "  ${ns}/${name}/${container}: ${img}  (missing ${CUSTOM_TAG})"
            failed=1
        fi
    done

    log "Checking tenancy-manager service..."
    if kubectl -n "${TM_NAMESPACE}" get svc "${TM_SERVICE_NAME}" &>/dev/null; then
        kubectl -n "${TM_NAMESPACE}" get svc "${TM_SERVICE_NAME}"
    else
        warn "tenancy-manager Service missing in ${TM_NAMESPACE}"
        failed=1
    fi

    log "Checking tenancy-manager bootstrap (default org/project)..."
    local bootstrap_expected="${EMF_DEFAULT_TENANCY:-false}"
    local pg_pod="postgresql-cluster-1"
    if kubectl -n orch-database get pod "${pg_pod}" &>/dev/null; then
        local org_count proj_count
        org_count=$(kubectl -n orch-database exec "${pg_pod}" -c postgres -- \
            psql -U postgres -d orch-iam-iam-tenancy -tAc \
            "SELECT count(*) FROM orgs WHERE name='default';" 2>/dev/null | tr -d '[:space:]')
        proj_count=$(kubectl -n orch-database exec "${pg_pod}" -c postgres -- \
            psql -U postgres -d orch-iam-iam-tenancy -tAc \
            "SELECT count(*) FROM projects WHERE name='default';" 2>/dev/null | tr -d '[:space:]')
        if [[ "${bootstrap_expected}" == "true" ]]; then
            if [[ "${org_count}" == "1" && "${proj_count}" == "1" ]]; then
                log "  default org+project bootstrapped by tenancy-manager"
            else
                warn "  bootstrap missing (orgs='${org_count}', projects='${proj_count}') \
— check EMF_DEFAULT_TENANCY and tenancy-manager logs"
                failed=1
            fi
        else
            log "  EMF_DEFAULT_TENANCY=false — bootstrap intentionally skipped (orgs='${org_count}', projects='${proj_count}')"
        fi
    else
        if [[ "${bootstrap_expected}" == "true" ]]; then
            warn "postgresql-cluster-1 pod missing in orch-database — cannot verify bootstrap"
            failed=1
        else
            log "  EMF_DEFAULT_TENANCY=false — skipping bootstrap probe"
        fi
    fi

    log "Checking apiv2 nexusAPIURL..."
    local url
    url=$(kubectl -n orch-infra get deployment apiv2 -o yaml 2>/dev/null \
        | grep -oE -- '-nexusAPIURL=[^ "]*' | head -1 || true)
    if [[ "${url}" == *tenancy-manager.orch-iam* ]]; then
        log "  apiv2: ${url}"
    else
        warn "  apiv2 nexusAPIURL not pointing at tenancy-manager (got: ${url:-none})"
    fi

    log "Checking pod readiness across orch-* namespaces..."
    local not_ready
    not_ready=$(kubectl get pods -A --no-headers 2>/dev/null \
        | awk '$1 ~ /^(orch-|cattle-system|cert-manager|postgresql-operator|orch-secret)/ \
              && $4 !~ /Running|Completed/ {print}')
    if [[ -n "${not_ready}" ]]; then
        warn "Pods not in Running/Completed:"
        echo "${not_ready}"
        failed=1
    else
        log "  All orch-* pods are Running/Completed"
    fi

    echo ""
    if (( failed )); then warn "Some checks failed — review output above"
    else log "All verification checks passed"
    fi
}

# ──────────────────────────────────────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────────────────────────────────────
main() {
    local cmd="${1:-all}"

    echo -e "${CYAN}╔════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║  Nexus Replacement — Helmfile Deploy (EIM-only)        ║${NC}"
    echo -e "${CYAN}║  Tag:     ${CUSTOM_TAG}$(printf '%*s' $((28 - ${#CUSTOM_TAG})) '')${CYAN}║${NC}"
    echo -e "${CYAN}║  Profile: ${HELMFILE_ENV}$(printf '%*s' $((28 - ${#HELMFILE_ENV})) '')${CYAN}║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════╝${NC}"

    case "${cmd}" in
        all)
            ensure_repos
            check_prerequisites
            build_images
            tag_images
            run_pre_orch
            run_post_orch
            run_mt_setup
            verify_helmfile
            ;;
        repos)     ensure_repos ;;
        build)     ensure_repos; check_prerequisites; build_images; tag_images ;;
        tag)       tag_images ;;
        load)      load_images_k3s ;;
        pre)       run_pre_orch ;;
        deploy)    check_prerequisites; run_post_orch ;;
        setup)     run_mt_setup ;;
        verify)    verify_helmfile ;;
        uninstall) run_uninstall ;;
        *)
            cat <<EOF
Usage: $0 {all|repos|build|tag|load|pre|deploy|setup|verify|uninstall}

Steps:
  all        Full workflow: repos → build → tag → pre-orch → post-orch
             (tenancy v2 charts) → MT setup → verify
  repos      Clone repos and switch to *_nexus_replacement branches
  build      Build the 5 EIM custom Docker images + orch-cli
  tag        Tag images with full registry paths
  load       Load images into k3s containerd
  pre        Run pre-orch.sh install
  deploy     Run post-orch-deploy.sh install (tenancy v2 always on)
             (auto-loads images first)
  setup      Create default org/project/users via orch-cli + Keycloak Admin API
  verify     Verify helm releases, image tags, tenancy wiring, pod health
  uninstall  helmfile destroy + remove residual orch-* namespaces and CRDs

Environment variables:
  CUSTOM_TAG          Image tag (default: nexus-replacement-YYYYMMDD)
  HELMFILE_ENV        Helmfile environment (default: onprem-vpro)
  HELMFILE_EMF_DIR    Helmfile EMF clone (default: ${HELMFILE_EMF_DIR})
  ORCH_UTILS_PATH     orch-utils clone (default: ${ORCH_UTILS_PATH})
  INFRA_CORE_PATH     infra-core clone (default: ${INFRA_CORE_PATH})
  ORCH_METADATA_BROKER_PATH  orch-metadata-broker clone
  ORCH_CLI_PATH       orch-cli clone (default: ${ORCH_CLI_PATH})
  ORCH_DOMAIN         Override orchestrator domain auto-detection

Note: this script is for the helmfile (k3s) flow ONLY. The argocd (kind) flow
lives in deploy-nexus-replacement.sh and uses 'mage tenantUtils:createDefaultMtSetup'
(mage/tenancy_rest.go, mage/tenant_utils.go). Both flows hit the same
tenancy-manager REST API — this script just talks to it from bash via orch-cli.
EOF
            exit 1
            ;;
    esac
}

main "$@"
