#!/bin/bash
# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

# Check virtualization environment
if command -v systemd-detect-virt &>/dev/null; then
  env_type=$(systemd-detect-virt)
  if [ "$env_type" == "none" ]; then
    echo "Bare Metal continuing install"
  else
    echo "Running in a VM: $env_type"
  fi
else
  echo "systemd-detect-virt not found. Install or try another method."
fi

# Update package list
sudo apt-get update
sudo apt-get install -y ca-certificates curl

# TODO: Detect Ubuntu 22.04 or 24.04 and install packages accordingly

# Install virtualization packages (minimal set)
sudo apt-get install -y \
  qemu-kvm qemu-system-x86 \
  libvirt-daemon-system libvirt-clients \
  bridge-utils virtinst

# Start and enable libvirtd service
sudo systemctl start libvirtd
sudo systemctl enable libvirtd
sleep 3

# Add user to virtualization groups
sudo usermod -aG libvirt,kvm "$USER"

# Backup and configure libvirtd
sudo cp /etc/libvirt/libvirtd.conf /etc/libvirt/libvirtd.conf.bak

# Update the configuration file
sudo sed -i 's/^#unix_sock_group = "libvirt"/unix_sock_group = "libvirt"/' /etc/libvirt/libvirtd.conf
sudo sed -i 's/^#unix_sock_rw_perms = "0770"/unix_sock_rw_perms = "0770"/' /etc/libvirt/libvirtd.conf

# Ensure the settings are present in the file if they were not commented out
grep -q '^unix_sock_group = "libvirt"' /etc/libvirt/libvirtd.conf || echo 'unix_sock_group = "libvirt"' | sudo tee -a /etc/libvirt/libvirtd.conf
grep -q '^unix_sock_rw_perms = "0770"' /etc/libvirt/libvirtd.conf || echo 'unix_sock_rw_perms = "0770"' | sudo tee -a /etc/libvirt/libvirtd.conf

sudo systemctl restart libvirtd

# Disable apparmor profiles for libvirt (best-effort)
if command -v apparmor_parser &>/dev/null; then
  sudo mkdir -p /etc/apparmor.d/disable/
  sudo ln -sf /etc/apparmor.d/usr.sbin.libvirtd /etc/apparmor.d/disable/ || true
  sudo ln -sf /etc/apparmor.d/usr.lib.libvirt.virt-aa-helper /etc/apparmor.d/disable/ || true
  sudo apparmor_parser -R /etc/apparmor.d/usr.sbin.libvirtd || true
  sudo apparmor_parser -R /etc/apparmor.d/usr.lib.libvirt.virt-aa-helper || true
  sudo systemctl reload apparmor || true
fi

sleep 2
# Verify installations and display versions
echo "Installed applications and their versions:"
dpkg -l | grep -E 'qemu|libvirt|virtinst|bridge-utils|cpu-checker' || true

# Check KVM support
echo "Checking KVM support..."
if command -v kvm-ok &>/dev/null; then
  if kvm-ok; then
    echo "KVM acceleration is supported on this system."
  else
    echo "KVM acceleration is not supported or not enabled. Please check your BIOS/UEFI settings."
  fi
else
  echo "kvm-ok not found (package: cpu-checker). Skipping KVM check."
fi
sudo chmod 666 /var/run/libvirt/libvirt-sock || true
sudo chmod 666 /var/run/libvirt/libvirt-sock-ro || true

virsh list --all || true
virsh pool-list --all || true
