#!/usr/bin/env bash
set -euo pipefail

# Installs dependencies for tools/router/router.sh on Linux.
#
# Usage:
#   tools/router/install-deps.sh check
#   tools/router/install-deps.sh install
#
# Notes:
# - Installs OS packages where possible.
# - kubectl is installed via upstream release download if missing.
# - Docker installation varies widely by distro/company policy; this script tries distro packages
#   first and prints guidance if it cannot install.

need_cmd() {
  command -v "$1" >/dev/null 2>&1
}

as_root_prefix() {
  if [[ "$(id -u)" == "0" ]]; then
    echo ""
  elif need_cmd sudo; then
    echo "sudo"
  else
    echo "ERROR: need root or sudo to install packages" >&2
    exit 1
  fi
}

os_id() {
  if [[ -r /etc/os-release ]]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    echo "${ID:-}"
  fi
}

os_like() {
  if [[ -r /etc/os-release ]]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    echo "${ID_LIKE:-}"
  fi
}

pkg_install() {
  local pkgs=("$@")
  local rootp
  rootp="$(as_root_prefix)"

  local id like
  id="$(os_id)"
  like="$(os_like)"

  if need_cmd apt-get; then
    ${rootp} apt-get update -y
    ${rootp} apt-get install -y "${pkgs[@]}"
    return 0
  fi

  if need_cmd dnf; then
    ${rootp} dnf install -y "${pkgs[@]}"
    return 0
  fi

  if need_cmd yum; then
    ${rootp} yum install -y "${pkgs[@]}"
    return 0
  fi

  if need_cmd zypper; then
    ${rootp} zypper --non-interactive install "${pkgs[@]}"
    return 0
  fi

  echo "ERROR: unsupported distro/package manager (ID=${id}, LIKE=${like})" >&2
  return 1
}

install_kubectl() {
  if need_cmd kubectl; then
    return 0
  fi

  if ! need_cmd curl; then
    echo "Installing curl (required to download kubectl)" >&2
    pkg_install curl ca-certificates
  fi

  local rootp
  rootp="$(as_root_prefix)"

  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *)
      echo "ERROR: unsupported arch for kubectl download: $arch" >&2
      return 1
      ;;
  esac

  local version url
  version="$(curl -fsSL https://dl.k8s.io/release/stable.txt)"
  url="https://dl.k8s.io/release/${version}/bin/linux/${arch}/kubectl"

  echo "Downloading kubectl ${version}..." >&2
  curl -fsSL -o /tmp/kubectl "$url"
  ${rootp} install -m 0755 /tmp/kubectl /usr/local/bin/kubectl
  rm -f /tmp/kubectl

  echo "kubectl installed: $(kubectl version --client --output=yaml 2>/dev/null | head -n 3 || true)" >&2
}

install_docker_and_compose() {
  # This tries distro packages. If your environment uses Docker CE repos or rootless docker,
  # you may need to install manually.
  if need_cmd docker && (need_cmd docker-compose || docker compose version >/dev/null 2>&1); then
    return 0
  fi

  echo "Installing docker and compose (best-effort via distro packages)..." >&2

  # Debian/Ubuntu: docker.io + docker-compose (or docker-compose-plugin)
  if need_cmd apt-get; then
    pkg_install jq openssl docker.io docker-compose-plugin || pkg_install jq openssl docker.io docker-compose

    # Best-effort: ensure daemon is running
    local rootp
    rootp="$(as_root_prefix)"
    if need_cmd systemctl; then
      ${rootp} systemctl enable --now docker >/dev/null 2>&1 || true
    fi
    return 0
  fi

  # Fedora/RHEL family: docker or moby-engine may be present; compose plugin varies.
  if need_cmd dnf; then
    pkg_install jq openssl docker docker-compose-plugin || pkg_install jq openssl docker
    return 0
  fi

  if need_cmd yum; then
    pkg_install jq openssl docker docker-compose-plugin || pkg_install jq openssl docker
    return 0
  fi

  # Fallback: just ensure jq/openssl are installed
  pkg_install jq openssl
  echo "NOTE: Please install Docker + docker-compose manually for your distro." >&2
}

check() {
  local missing=0
  for cmd in jq openssl kubectl docker; do
    if ! need_cmd "$cmd"; then
      echo "MISSING: $cmd" >&2
      missing=1
    fi
  done

  if need_cmd docker-compose; then
    :
  elif docker compose version >/dev/null 2>&1; then
    echo "OK: docker compose plugin present" >&2
  else
    echo "MISSING: docker-compose (or docker compose plugin)" >&2
    missing=1
  fi

  if (( missing == 0 )); then
    echo "All dependencies present." >&2
  else
    echo "Some dependencies are missing." >&2
  fi
  return "$missing"
}

install() {
  # Base tools first
  pkg_install jq openssl ca-certificates
  install_kubectl
  install_docker_and_compose

  echo "Done. Re-run: tools/router/install-deps.sh check" >&2
}

main() {
  case "${1:-}" in
    check) check ;;
    install) install ;;
    -h|--help|help|"")
      sed -n '1,40p' "$0"
      ;;
    *)
      echo "ERROR: unknown command: ${1:-}" >&2
      echo "Usage: $0 check|install" >&2
      exit 2
      ;;
  esac
}
