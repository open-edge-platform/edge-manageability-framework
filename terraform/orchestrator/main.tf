# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

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

data "local_file" "version_file" {
  filename = "${var.working_directory}/../../VERSION"
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
    mac            = "52:54:00:12:34:56"
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

resource "null_resource" "wait_for_cloud_init" {

  depends_on = [
    null_resource.resize_and_restart_vm
  ]

  connection {
    type     = "ssh"
    host     = local.vmnet_ip0
    port     = var.vm_ssh_port
    user     = var.vm_ssh_user
    password = var.vm_ssh_password
  }

  provisioner "remote-exec" {
    # The following uses concat and flatten to merge all profile overwrite commands into a single list for remote execution.
    inline = [
      "set -o errexit",
      "until [ -f /var/lib/cloud/instance/boot-finished ]; do echo 'Waiting for cloud-init...'; sleep 15; done",
      "echo 'cloud-init has finished!'",
    ]
    when = create
  }
}

resource "null_resource" "copy_files" {
  depends_on = [
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
    source      = "../../${var.working_directory}/on-prem-installers/onprem/cluster_onprem.tpl"
    destination = "/home/ubuntu/cluster_onprem.tpl"
    when        = create
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/on-prem-installers/onprem/onprem_orch_install.sh"
    destination = "/home/ubuntu/onprem_orch_install.sh"
    when        = create
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/on-prem-installers/onprem/onprem_pre_install.sh"
    destination = "/home/ubuntu/onprem_pre_install.sh"
    when        = create
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/on-prem-installers/onprem/onprem.env"
    destination = "/home/ubuntu/onprem.env"
    when        = create
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/installer/generate_cluster_yaml.sh"
    destination = "/home/ubuntu/generate_cluster_yaml.sh"
    when        = create
  } 

  provisioner "file" {
    source      = "../../${var.working_directory}/on-prem-installers/onprem/functions.sh"
    destination = "/home/ubuntu/functions.sh"
    when        = create
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/on-prem-installers/onprem/upgrade_postgres.sh"
    destination = "/home/ubuntu/upgrade_postgres.sh"
    when        = create
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/on-prem-installers/onprem/vault_unseal.sh"
    destination = "/home/ubuntu/vault_unseal.sh"
    when        = create
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/on-prem-installers/onprem/after_upgrade_restart.sh"
    destination = "/home/ubuntu/after_upgrade_restart.sh"
    when        = create
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/on-prem-installers/onprem/onprem_upgrade.sh"
    destination = "/home/ubuntu/onprem_upgrade.sh"
    when        = create
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/on-prem-installers/onprem/storage_backup.sh"
    destination = "/home/ubuntu/storage_backup.sh"
    when        = create
  }

  provisioner "file" {
    source      = "${var.working_directory}/scripts/access_script.tftpl"
    destination = "/home/ubuntu/access_script.sh"
    when        = create
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /home/ubuntu/uninstall_onprem.sh",
      "chmod +x /home/ubuntu/onprem_installer.sh",
      "chmod +x /home/ubuntu/onprem_pre_install.sh",
      "chmod +x /home/ubuntu/onprem_orch_install.sh",
      "chmod +x /home/ubuntu/onprem.env",
      "chmod +x /home/ubuntu/cluster_onprem.tpl",
      "chmod +x /home/ubuntu/generate_cluster_yaml.sh",
      "chmod +x /home/ubuntu/functions.sh",
      "chmod +x /home/ubuntu/access_script.sh",
      "chmod +x /home/ubuntu/upgrade_postgres.sh",
      "chmod +x /home/ubuntu/vault_unseal.sh",
      "chmod +x /home/ubuntu/after_upgrade_restart.sh",
      "chmod +x /home/ubuntu/storage_backup.sh",
      "chmod +x /home/ubuntu/onprem_upgrade.sh"
    ]
    when = create
  }

  provisioner "remote-exec" {
    inline = [
      "set -o errexit",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env RELEASE_SERVICE_URL ${var.rs_url}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env DEPLOY_VERSION ${var.deploy_tag}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env ORCH_INSTALLER_PROFILE ${var.orch_profile}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env CLUSTER_DOMAIN ${var.cluster_domain}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env DOCKER_USERNAME ${var.docker_username}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env DOCKER_PASSWORD ${var.docker_password}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env GITEA_IMAGE_REGISTRY ${var.gitea_image_registry}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env ARGO_IP ${local.vmnet_ip1}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env TRAEFIK_IP ${local.vmnet_ip2}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env NGINX_IP ${local.vmnet_ip3}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env ENABLE_EXPLICIT_PROXY ${var.enable_explicit_proxy}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env ORCH_HTTP_PROXY ${var.http_proxy}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env ORCH_HTTPS_PROXY ${var.https_proxy}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env ORCH_NO_PROXY ${var.no_proxy}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env EN_HTTP_PROXY ${var.en_http_proxy}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env EN_HTTPS_PROXY ${var.en_https_proxy}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env EN_FTP_PROXY ${var.ftp_proxy}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env EN_SOCKS_PROXY ${var.socks_proxy}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env EN_NO_PROXY ${var.no_proxy}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env GIT_PROXY ${var.git_proxy}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env OXM_PXE_SERVER_INT ${var.oxm_pxe_server_int}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env OXM_PXE_SERVER_IP ${var.oxm_pxe_server_ip}'",
      "bash -c 'source /home/ubuntu/functions.sh; update_config_variable /home/ubuntu/onprem.env OXM_PXE_SERVER_SUBNET ${var.oxm_pxe_server_subnet}'",
      "cat /home/ubuntu/onprem.env",
      "echo 'onprem.env updated successfully'"
    ]
    when = create
  }
}

resource "null_resource" "copy_local_orch_installer" {

  count = var.use_local_build_artifact ? 1 : 0

  depends_on = [
    null_resource.resize_and_restart_vm
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
      "mkdir /home/ubuntu/installers",
      "mkdir /home/ubuntu/repo_archives",
    ]
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/${var.local_installers_path}/onprem-argocd-installer_${var.deploy_tag}_amd64.deb"
    destination = "/home/ubuntu/installers/onprem-argocd-installer_${var.deploy_tag}_amd64.deb"
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/${var.local_installers_path}/onprem-config-installer_${var.deploy_tag}_amd64.deb"
    destination = "/home/ubuntu/installers/onprem-config-installer_${var.deploy_tag}_amd64.deb"
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/${var.local_installers_path}/onprem-gitea-installer_${var.deploy_tag}_amd64.deb"
    destination = "/home/ubuntu/installers/onprem-gitea-installer_${var.deploy_tag}_amd64.deb"
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/${var.local_installers_path}/onprem-ke-installer_${var.deploy_tag}_amd64.deb"
    destination = "/home/ubuntu/installers/onprem-ke-installer_${var.deploy_tag}_amd64.deb"
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/${var.local_installers_path}/onprem-orch-installer_${var.deploy_tag}_amd64.deb"
    destination = "/home/ubuntu/installers/onprem-orch-installer_${var.deploy_tag}_amd64.deb"
  }

  provisioner "file" {
    source      = "../../${var.working_directory}/${var.local_repo_archives_path}/onpremFull_edge-manageability-framework_${split("\n", data.local_file.version_file.content)[0]}.tgz"
    destination = "/home/ubuntu/repo_archives/onpremFull_edge-manageability-framework_${split("\n", data.local_file.version_file.content)[0]}.tgz"
  }
}

resource "null_resource" "exec_installer" {

  // Enable this resource if auto-install is enabled
  count = var.enable_auto_install ? 1 : 0

  depends_on = [
    null_resource.copy_local_orch_installer,
    null_resource.wait_for_cloud_init,
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
      "bash -c 'cd /home/ubuntu; source onprem.env; ./onprem_installer.sh --yes --trace ${var.use_local_build_artifact ? "--skip-download" : ""} -- --yes --trace | tee ./install_output.log; exit $${PIPESTATUS[0]}'",
    ]
    when = create
    # Increased timeout to 45 minutes to accommodate full DEB installation
    # including Gitea Helm installation (25 minutes) + buffer for other components
    timeout = "45m"
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
