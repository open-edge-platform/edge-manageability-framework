#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

sudo NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive apt-get update -y

sudo NEEDRESTART_MODE=a DEBIAN_FRONTEND=noninteractive apt-get -y install \
    libvirt-daemon-system \
    libvirt-clients \
    qemu-kvm \
    mkisofs \
    xsltproc \
    sshpass \
    pesign \
    virt-manager \
    ovmf \
    expect \
    minicom \
    socat \
    xterm \
    efitools \
    libvirt-dev

# Check if 'unix_sock_group' is set to 'libvirt' in libvirtd.conf, if not, set it
if ! grep -q '^unix_sock_group = "libvirt"$' /etc/libvirt/libvirtd.conf; then
    sudo sed -i 's/^#unix_sock_group = "libvirt"/unix_sock_group = "libvirt"/' /etc/libvirt/libvirtd.conf
fi

# Check if 'unix_sock_ro_perms' is set to '0777' in libvirtd.conf, if not, set it
if ! grep -q '^unix_sock_ro_perms = "0777"$' /etc/libvirt/libvirtd.conf; then
    sudo sed -i 's/^#unix_sock_ro_perms = "0777"/unix_sock_ro_perms = "0777"/' /etc/libvirt/libvirtd.conf
fi

# Check if 'unix_sock_rw_perms' is set to '0770' in libvirtd.conf, if not, set it
if ! grep -q '^unix_sock_rw_perms = "0770"$' /etc/libvirt/libvirtd.conf; then
    sudo sed -i 's/^#unix_sock_rw_perms = "0770"/unix_sock_rw_perms = "0770"/' /etc/libvirt/libvirtd.conf
fi

# Check if AppArmor configuration for libvirt includes permissions for /var/lib/libvirt/images/
# If not, add the necessary read and write permissions
if ! grep -q '"/var/lib/libvirt/images/" r,' /etc/apparmor.d/libvirt/TEMPLATE.qemu; then
    sudo sed -i '/#include <abstractions\/libvirt-qemu>/a \
    "/var/lib/libvirt/images/" r,\n  "/var/lib/libvirt/images/**" rwk,' /etc/apparmor.d/libvirt/TEMPLATE.qemu
fi

# Check if the current user is part of the 'libvirt' group, if not, add the user to 'libvirt' and 'kvm' groups
if ! groups $USER | grep -q '\blibvirt\b'; then
    sudo usermod -aG libvirt,kvm $USER
fi

# Check if the current user has read and write permissions for the libvirt socket
# If not, set the appropriate ACLs for the user
if ! getfacl /var/run/libvirt/libvirt-sock | grep -q "user:$USER:rw"; then
    sudo setfacl -m user:$USER:rw /var/run/libvirt/libvirt-sock
fi

# Restart libvirtd service to apply the changes
sudo systemctl restart libvirtd
