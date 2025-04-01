# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "local_file" "env_data_file" {
  content = templatefile(
    "${var.working_directory}/scripts/env.tftpl",
    {
      release_service_refresh_token = var.release_service_refresh_token,
      rs_url                        = var.rs_url,
      orch_profile                  = var.orch_profile,
      deploy_tag                    = var.deploy_tag,
      cluster_domain                = var.cluster_domain,
      docker_username               = var.docker_username,
      docker_password               = var.docker_password,
      gitea_image_registry          = var.gitea_image_registry,
      vmnet_ip1                     = local.vmnet_ip1,
      vmnet_ip2                     = local.vmnet_ip2,
      vmnet_ip3                     = local.vmnet_ip3,
    },
  )
  filename = "${path.module}/files/.env"

  lifecycle {
    prevent_destroy = false
  }
}

resource "local_file" "proxy_config_file" {
  content = templatefile(
    "${path.module}/templates/proxy_config.tftpl",
    {
      http_proxy  = var.http_proxy,
      https_proxy = var.https_proxy,
      no_proxy    = var.no_proxy,
      ftp_proxy   = var.ftp_proxy,
      socks_proxy = var.socks_proxy,
    },
  )
  filename = "${path.module}/files/proxy_config.yaml"

  lifecycle {
    prevent_destroy = false
  }
}

resource "local_file" "cloud_init_user_data_file" {
  content = templatefile(
    "${var.working_directory}/cloud-inits/cloud_config.tftpl",
    {
      ssh_key             = var.ssh_public_key,
      hostname            = var.vm_name,
      ntp_server          = var.ntp_server,
      http_proxy          = var.http_proxy,
      https_proxy         = var.https_proxy,
      no_proxy            = var.no_proxy,
      ca_certs            = [for ca_cert_paths in var.ca_certificates : indent(6, file(ca_cert_paths))], // Read CA certs into a list
      enable_auto_install = var.enable_auto_install,
    },
  )
  filename = "${path.module}/files/user_data.cfg"

  lifecycle {
    prevent_destroy = false
  }
}

resource "local_file" "cloud_init_networking_data_file" {
  content = templatefile(
    "${path.module}/cloud-inits/network_config.tftpl",
    {
      vmnet_ips = var.vmnet_ips,
    }
  )
  filename = "${path.module}/files/network_data.cfg"

  lifecycle {
    prevent_destroy = false
  }
}

resource "libvirt_cloudinit_disk" "cloud_init_config" {
  name           = "commoninit.iso"
  pool           = var.storage_pool
  user_data      = local_file.cloud_init_user_data_file.content
  network_config = local_file.cloud_init_networking_data_file.content
}

resource "libvirt_volume" "os_disk_image" {
  name   = "os-disk-image.img"
  pool   = var.storage_pool
  source = fileexists("${path.module}/files/os-disk-image.img") ? "${path.module}/files/os-disk-image.img" : var.os_disk_image_url
  format = "qcow2"

  // Cache the image locally
  provisioner "local-exec" {
    command = "if [ ! -f ${path.module}/files/os-disk-image.img ]; then curl -o ${path.module}/files/os-disk-image.img ${var.os_disk_image_url}; fi"
  }
}

resource "libvirt_volume" "root" {
  depends_on = [
    libvirt_volume.os_disk_image
  ]

  name           = var.vm_boot_disk_name
  pool           = var.storage_pool
  format         = "qcow2"
  base_volume_id = libvirt_volume.os_disk_image.id
}

resource "libvirt_domain" "orch-vm" {
  depends_on = [
    libvirt_cloudinit_disk.cloud_init_config,
    libvirt_volume.root,
  ]

  type = var.vm_domain_type

  name       = var.vm_name
  memory     = var.vm_memory
  vcpu       = var.vm_vcpu
  autostart  = true
  qemu_agent = false
  running    = false
  timeouts {
    create = var.vm_create_timeout
  }

  cpu {
    mode = var.vm_cpu_mode
  }

  cloudinit = libvirt_cloudinit_disk.cloud_init_config.id

  disk {
    volume_id = libvirt_volume.root.id
    scsi      = true
  }

  // Disable disk caching and set IO to native for better performance
  xml {
    xslt = templatefile("${path.module}/customize_domain.xsl.tftpl", {
      enable_cpu_pinning = var.enable_cpu_pinning,
      vm_vcpu            = var.vm_vcpu
    })
  }

  network_interface {
    network_name   = var.network_name
    hostname       = "vmnet"
    wait_for_lease = false
    addresses      = local.vmnet_ips
  }

  console {
    type        = "pty"
    target_port = "0"
    target_type = "serial"
  }
}

resource "null_resource" "resize_and_restart_vm" {
  depends_on = [
    libvirt_domain.orch-vm
  ]

  provisioner "local-exec" {
    command = <<-EOT
      sudo virsh vol-resize ${var.vm_boot_disk_name} --pool ${var.storage_pool} ${var.vm_boot_disk_size} &&
      sudo virsh start ${var.vm_name}
    EOT
  }
}

resource "null_resource" "copy_files" {
  depends_on = [
    local_file.env_data_file,
    null_resource.resize_and_restart_vm
  ]

  connection {
    type     = "ssh"
    host     = local.vmnet_ip0
    port     = var.vm_ssh_port
    user     = var.vm_ssh_user
    password = var.vm_ssh_password
  }

  provisioner "file" {
    source      = local_file.env_data_file.filename
    destination = "/home/ubuntu/.env"
    when        = create
  }

  provisioner "file" {
    source      = "${path.module}/files/proxy_config.yaml"
    destination = "/home/ubuntu/proxy_config.yaml"
    when        = create
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/on-prem-installers/onprem/uninstall_onprem.sh"
    destination = "/home/ubuntu/uninstall_onprem.sh"
    when        = create
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/on-prem-installers/onprem/onprem_installer.sh"
    destination = "/home/ubuntu/onprem_installer.sh"
    when        = create
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/on-prem-installers/onprem/functions.sh"
    destination = "/home/ubuntu/functions.sh"
    when        = create
  }

  provisioner "file" {
    source      = "${var.working_directory}/scripts/access_script.tftpl"
    destination = "/home/ubuntu/access_script.sh"
    when        = create
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /home/ubuntu/uninstall_onprem.sh /home/ubuntu/onprem_installer.sh /home/ubuntu/functions.sh /home/ubuntu/access_script.sh /home/ubuntu/.env",
    ]
    when = create
  }
}

resource "null_resource" "write_installer_config" {

  // Disable this resource if auto-install is disabled
  count = var.enable_auto_install ? 1 : 0

  depends_on = [
    null_resource.copy_files
  ]

  connection {
    type     = "ssh"
    host     = local.vmnet_ip0
    port     = var.vm_ssh_port
    user     = var.vm_ssh_user
    password = var.vm_ssh_password
  }

  provisioner "remote-exec" {
    inline = [
      "set -o errexit",
      "until [ -f /var/lib/cloud/instance/boot-finished ]; do echo 'Waiting for cloud-init...'; sleep 15; done",
      "echo 'cloud-init has finished!'",
      "bash -c 'cd /home/ubuntu; source .env; ./onprem_installer.sh --trace ${var.override_flag ? "--override" : ""} --write-config'",
      "mv /home/ubuntu/proxy_config.yaml /home/ubuntu/repo_archives/tmp/edge-manageability-framework/orch-configs/profiles/proxy-internal.yaml"
    ]
    when = create
  }
}

resource "null_resource" "exec_installer" {

  // Disable this resource if auto-install is disabled
  count = var.enable_auto_install ? 1 : 0

  depends_on = [
    null_resource.write_installer_config
  ]

  connection {
    type     = "ssh"
    host     = local.vmnet_ip0
    port     = var.vm_ssh_port
    user     = var.vm_ssh_user
    password = var.vm_ssh_password
  }

  provisioner "remote-exec" {
    inline = [
      "bash -c 'cd /home/ubuntu; source .env; env; ./onprem_installer.sh --trace --yes ${var.override_flag ? "--override" : ""} | tee ./install_output.log; exit $${PIPESTATUS[0]}'",
    ]
    when = create
  }
}

resource "null_resource" "wait_for_kubeconfig" {

  // Disable this resource if auto-install is disabled
  count = var.enable_auto_install ? 1 : 0

  depends_on = [
    null_resource.exec_installer
  ]

  connection {
    type     = "ssh"
    host     = local.vmnet_ip0
    port     = var.vm_ssh_port
    user     = var.vm_ssh_user
    password = var.vm_ssh_password
  }

  provisioner "remote-exec" {
    inline = [
      "until test -f /home/${var.vm_ssh_user}/.kube/config; do sleep 15; done", // This takes ~10 minutes - patience!
      "echo 'KUBECONFIG file exists!'",
    ]
    when = create
  }
}

resource "null_resource" "copy_kubeconfig" {

  // Disable this resource if auto-install is disabled
  count = var.enable_auto_install ? 1 : 0

  depends_on = [
    null_resource.wait_for_kubeconfig
  ]

  provisioner "local-exec" {
    command = "rm ${path.module}/files/kubeconfig || true"
    when    = create
  }

  provisioner "local-exec" {
    command = "SSH_PASSWORD='${var.vm_ssh_password}' ${path.module}/scripts/sshpass.bash scp -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -P 22 ${var.vm_ssh_user}@${local.vmnet_ip0}:/home/${var.vm_ssh_user}/.kube/config ${path.module}/files/kubeconfig"
    when    = create
  }

  provisioner "local-exec" {
    // Set the cluster URL to the VM's IP address so that kubectl can remotely connect to the cluster. Disable TLS
    // verification because the server name dialed does not match the certificate.
    command = "KUBECONFIG=${path.module}/files/kubeconfig kubectl config set-cluster default --server=https://${local.vmnet_ip0}:6443 --insecure-skip-tls-verify=true"
    when    = create
  }
}
