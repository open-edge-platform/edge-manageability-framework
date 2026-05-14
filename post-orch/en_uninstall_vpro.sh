#!/bin/bash

# SPDX-FileCopyrightText: (C) 2026 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

set -Eeuo pipefail

APP_NAME="Modular vPro"
DRY_RUN="${DRY_RUN:-0}"

sudo rpc deactivate --local || true

SCRIPT_DIR=$(pwd)
MANIFEST_CANDIDATES=(
  "${SCRIPT_DIR}/.success_install_status"
  "${SCRIPT_DIR}/install_pkgs_status"
  "${SCRIPT_DIR}/.base_pkg_install_done"
)

SERVICES=(
  "device-discovery-agent.service"
  "node-agent.service"
  "platform-manageability-agent.service"
  "caddy.service"
  "caddy-api.service"
)

# NOTE: lms.service is intentionally NOT in SERVICES list.
# LMS (Local Manageability Service) is required for AMT CIRA connectivity.
# Removing/masking it breaks AMT activation on next install.

FILES=(
  "/etc/apt/sources.list.d/edge-node.list"
  "/etc/apt/trusted.gpg.d/edge-node.asc"
  "/etc/apparmor.d/opt.edge-node.bin.platform-manageability-agent"
  "/etc/apparmor.d/opt.edge-node.bin.pm-agent"
  "/etc/apparmor.d/opt.edge-node.bin.node-agent"
  "/etc/apparmor.d/opt.edge-node.bin.device-discovery-agent"
  "/usr/bin/rpc"
  "/usr/local/bin/oras"
  "/usr/local/share/ca-certificates/orch-ca.crt"
)

DIRS=(
  "/etc/edge-node"
  "/etc/intel_edge_node"
  "/var/lib/edge-node"
  "/opt/edge-node"
  "/run/node-agent"
  "/run/platform-observability-agent"
)

APPARMOR_PROFILES=(
  "opt.edge-node.bin.pm-agent"
  "opt.edge-node.bin.node-agent"
  "opt.edge-node.bin.device-discovery-agent"
)

log() { printf '[%s] %s\n' "$(date '+%F %T')" "$*"; }
warn() { printf '[%s] WARN: %s\n' "$(date '+%F %T')" "$*" >&2; }

run() {
  if [[ "$DRY_RUN" == "1" ]]; then
    log "DRY_RUN: $*"
  else
    "$@"
  fi
}

require_root() {
  if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    warn "Run as root: sudo $0"
    exit 1
  fi
}

remove_path() {
  local p="$1"
  [[ -z "$p" || "$p" == "/" ]] && return 0
  if [[ -L "$p" || -f "$p" || -d "$p" ]]; then
    log "Removing $p"
    run rm -rf -- "$p"
  fi
}

stop_disable_service() {
  local svc="$1"
  command -v systemctl >/dev/null 2>&1 || return 0

  if systemctl list-unit-files --type=service --no-legend 2>/dev/null | awk '{print $1}' | grep -Fxq "$svc"; then
    log "Stopping service $svc"
    run systemctl stop "$svc" || true
    log "Disabling service $svc"
    run systemctl disable "$svc" || true
  fi

  remove_path "/etc/systemd/system/${svc}"
  remove_path "/lib/systemd/system/${svc}"
  remove_path "/usr/lib/systemd/system/${svc}"
}

unload_apparmor_profiles() {
  command -v apparmor_parser >/dev/null 2>&1 || return 0
  for profile in "${APPARMOR_PROFILES[@]}"; do
    if [[ -f "/etc/apparmor.d/${profile}" ]]; then
      log "Unloading AppArmor profile: $profile"
      run apparmor_parser -R "/etc/apparmor.d/${profile}" 2>/dev/null || true
    fi
  done
}

remove_manifest_entries() {
  local mf
  for mf in "${MANIFEST_CANDIDATES[@]}"; do
    [[ -f "$mf" ]] || continue
    log "Processing manifest: $mf"
    while IFS= read -r line; do
      line="${line#"${line%%[![:space:]]*}"}"
      line="${line%"${line##*[![:space:]]}"}"
      [[ -z "$line" || "${line:0:1}" == "#" ]] && continue
      remove_path "$line"
    done <"$mf"
    remove_path "$mf"
    return 0
  done
  return 1
}

# Ensure LMS (Intel Local Manageability Service) stays functional.
# LMS is required for AMT CIRA (Client Initiated Remote Access) to work.
# Without LMS, AMT activation succeeds but CIRA config fails, leaving
# RAS MPS Hostname empty and remote management broken.
ensure_lms_healthy() {
  command -v systemctl >/dev/null 2>&1 || return 0

  # Check if LMS package is installed
  if ! dpkg -l lms 2>/dev/null | grep -q '^ii'; then
    log "LMS package not installed, skipping LMS recovery"
    return 0
  fi

  log "Ensuring LMS (Local Manageability Service) remains functional"

  # Unmask if masked (this is the critical fix - masking prevents start)
  if systemctl is-enabled lms.service 2>&1 | grep -q "masked"; then
    log "LMS is masked, unmasking..."
    run systemctl unmask lms.service || true
  fi

  # Enable so it starts on boot
  run systemctl enable lms.service 2>/dev/null || true

  # Reload systemd to pick up any changes
  run systemctl daemon-reload || true

  # Start LMS
  run systemctl start lms.service 2>/dev/null || true

  # Verify
  if systemctl is-active --quiet lms.service 2>/dev/null; then
    log "LMS service is running"
  else
    warn "LMS service failed to start - AMT CIRA may not work on next install"
    warn "Manual fix: sudo systemctl unmask lms.service && sudo systemctl enable --now lms.service"
  fi
}

main() {
  require_root
  trap 'warn "Uninstall failed at line $LINENO"' ERR

  log "Starting ${APP_NAME} uninstall"

  # Stop and disable services (NOT lms - it must stay running)
  for svc in "${SERVICES[@]}"; do
    stop_disable_service "$svc"
  done

  run systemctl daemon-reload || true
  run systemctl reset-failed || true

  # Remove packages (NOT lms - it's a platform service, not an edge agent)
  log "Removing packages"
  run apt-get remove -y node-agent platform-manageability-agent device-discovery-agent caddy 2>/dev/null || true

  # Purge packages (removes config files too)
  log "Purging packages"
  run apt-get purge -y node-agent platform-manageability-agent device-discovery-agent caddy 2>/dev/null || true

  # Autoremove orphaned dependencies
  log "Removing orphaned dependencies"
  run apt-get autoremove -y 2>/dev/null || true

  if ! remove_manifest_entries; then
    warn "No install manifest found. Falling back to default cleanup list."
  fi

  # Unload AppArmor profiles before removing files
  unload_apparmor_profiles

  for f in "${FILES[@]}"; do
    remove_path "$f"
  done

  # Update CA certificates after removing orchestrator CA
  log "Updating CA certificates"
  run update-ca-certificates --fresh 2>/dev/null || true

  for d in "${DIRS[@]}"; do
    remove_path "$d"
  done

  # Remove agent user accounts and groups
  log "Removing agent user accounts and groups"
  run userdel -r pm-agent 2>/dev/null || true
  run userdel -r device-discovery-agent 2>/dev/null || true
  run userdel -r node-agent 2>/dev/null || true
  run userdel -r etcd 2>/dev/null || true
  run groupdel bm-agents 2>/dev/null || true

  # Remove sudoers files
  log "Removing sudoers files"
  remove_path "/etc/sudoers.d/pm-agent"
  remove_path "/etc/sudoers.d/device-discovery-agent"
  remove_path "/etc/sudoers.d/node-agent"

  # Final systemd reload
  run systemctl daemon-reload || true

  # CRITICAL: Ensure LMS stays healthy after uninstall.
  # apt-get autoremove or manifest cleanup may have affected LMS.
  # This step unmarks, enables, and starts LMS as a safety net.
  ensure_lms_healthy

  log "${APP_NAME} uninstall completed"
}

main "$@"
