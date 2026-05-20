#!/bin/bash
# SPDX-FileCopyrightText: (C) 2026 Intel Corporation
# SPDX-License-Identifier: Apache-2.0
#
# Installs and configures libvirt + QEMU/KVM on the GitHub Actions runner
# so that VEN (virtual edge node) VMs can be booted via libvirt.
#
# Supports Ubuntu 22.04 (jammy) and 24.04 (noble).

set -euxo pipefail

# ── Detect environment ────────────────────────────────────────────────────────
if command -v systemd-detect-virt &>/dev/null; then
  env_type=$(systemd-detect-virt || true)
  if [ "$env_type" == "none" ]; then
    echo "Bare metal — continuing install"
  else
    echo "Running in a VM: $env_type (nested virt required)"
  fi
fi

# Source /etc/os-release to detect Ubuntu version
# shellcheck disable=SC1091
. /etc/os-release
UBUNTU_VERSION="${VERSION_ID:-}"
echo "Detected Ubuntu version: ${UBUNTU_VERSION}"

# ── Pick the right package set per Ubuntu version ─────────────────────────────
# Ubuntu 24.04 (Noble) dropped the 'qemu' meta-package; use qemu-system-x86 etc.
case "${UBUNTU_VERSION}" in
  "24.04")
    QEMU_PKGS=(qemu-system-x86 qemu-system-common qemu-utils qemu-kvm)
    ;;
  "22.04")
    QEMU_PKGS=(qemu qemu-kvm)
    ;;
  *)
    echo "Unsupported Ubuntu version: ${UBUNTU_VERSION}" >&2
    exit 1
    ;;
esac

LIBVIRT_PKGS=(
  libvirt-daemon-system
  libvirt-clients
  libvirt-dev
  ovmf
  pesign
  efitools
  xsltproc
  socat
  expect
)

# ── Install ───────────────────────────────────────────────────────────────────
sudo apt-get update
sudo apt-get install -y ca-certificates curl
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
  "${QEMU_PKGS[@]}" \
  "${LIBVIRT_PKGS[@]}"

# ── Configure libvirtd ────────────────────────────────────────────────────────
# Backup once (don't clobber an existing .bak on re-runs)
if [ ! -f /etc/libvirt/libvirtd.conf.bak ]; then
  sudo cp /etc/libvirt/libvirtd.conf /etc/libvirt/libvirtd.conf.bak
fi

# Uncomment unix_sock_group / unix_sock_rw_perms if present, append if not
sudo sed -i 's/^#\s*unix_sock_group\s*=\s*"libvirt"/unix_sock_group = "libvirt"/' /etc/libvirt/libvirtd.conf
sudo sed -i 's/^#\s*unix_sock_rw_perms\s*=\s*"0770"/unix_sock_rw_perms = "0770"/' /etc/libvirt/libvirtd.conf
grep -q '^unix_sock_group = "libvirt"' /etc/libvirt/libvirtd.conf \
  || echo 'unix_sock_group = "libvirt"' | sudo tee -a /etc/libvirt/libvirtd.conf
grep -q '^unix_sock_rw_perms = "0770"' /etc/libvirt/libvirtd.conf \
  || echo 'unix_sock_rw_perms = "0770"' | sudo tee -a /etc/libvirt/libvirtd.conf

# ── Add current user to libvirt/kvm groups ────────────────────────────────────
sudo usermod -aG libvirt "$USER"
sudo usermod -aG kvm "$USER"

# ── Enable + start the libvirt daemon ────────────────────────────────────────
sudo systemctl daemon-reload
sudo systemctl enable --now libvirtd.service
sleep 3

# ── Disable AppArmor profiles that interfere with libvirt VM launches ────────
if [ -f /etc/apparmor.d/usr.sbin.libvirtd ]; then
  sudo ln -sf /etc/apparmor.d/usr.sbin.libvirtd /etc/apparmor.d/disable/
  sudo apparmor_parser -R /etc/apparmor.d/usr.sbin.libvirtd || true
fi
if [ -f /etc/apparmor.d/usr.lib.libvirt.virt-aa-helper ]; then
  sudo ln -sf /etc/apparmor.d/usr.lib.libvirt.virt-aa-helper /etc/apparmor.d/disable/
  sudo apparmor_parser -R /etc/apparmor.d/usr.lib.libvirt.virt-aa-helper || true
fi
sudo systemctl reload apparmor || true

# ── Restart libvirtd to pick up config + apparmor changes ────────────────────
sudo systemctl restart libvirtd.service
sleep 2

# Loosen socket perms for CI convenience (best-effort)
sudo chmod 666 /var/run/libvirt/libvirt-sock || true
sudo chmod 666 /var/run/libvirt/libvirt-sock-ro || true

# ── Verification (these MUST succeed; pipefail will catch failures) ──────────
echo "Installed virtualization packages:"
dpkg -l | grep -E 'qemu|libvirt-daemon-system|libvirt-clients|ovmf|xsltproc' || true

echo "Checking KVM support..."
if ! kvm-ok; then
  echo "❌ KVM acceleration is NOT available on this runner. VEN cannot boot." >&2
  exit 1
fi

echo "libvirtd status:"
sudo systemctl --no-pager --full status libvirtd.service

echo "Existing domains / pools / networks:"
sudo virsh list --all
sudo virsh pool-list --all
sudo virsh net-list --all

echo "✅ libvirt setup completed successfully on Ubuntu ${UBUNTU_VERSION}"
