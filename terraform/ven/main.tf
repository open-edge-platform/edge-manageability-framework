// Randomly generates vmid
resource "random_integer" "next_vmid" {
  min = 2000
  max = 1000000
}

resource "local_file" "cloud_init_user_data_file" {
  content = templatefile(
    "${var.working_directory}/cloud-inits/cloud_config.tftpl",
    {
      ssh_key     = var.ssh_public_key,
      hostname    = join("-", [var.vm_name, tostring(local.vmid)]),
      ntp_server  = var.ntp_server,
      http_proxy  = var.http_proxy,
      https_proxy = var.https_proxy,
      no_proxy    = var.no_proxy,
      ca_certs    = [for ca_cert_paths in var.ca_certificates : indent(6, file(ca_cert_paths))] # Read CA certs into a list
      env_base64_content = base64encode(
        templatefile(
          "${var.working_directory}/scripts/env.tftpl",
          {
            vmid = tostring(local.vmid)
          },
        )
      ),
    },
  )
  filename = "${path.module}/files/user_data_${local.vmid}.cfg"
}

resource "local_file" "cloud_init_networking_data_file" {
  content = file(
    "${var.working_directory}/cloud-inits/network_config.tftpl",
  )
  filename = "${path.module}/files/network_data_${local.vmid}.cfg"
}

resource "null_resource" "ven_cloud_init_config_files" {
  depends_on = [
    local_file.cloud_init_user_data_file,
    local_file.cloud_init_networking_data_file
  ]

  connection {
    type     = "ssh"
    user     = var.pm_ssh_user
    password = var.pm_ssh_password
    host     = var.pm_ssh_host
    port     = var.pm_ssh_port
  }

  // FIXME: Delete when resources are destroyed
  provisioner "file" {
    source      = local_file.cloud_init_user_data_file.filename
    destination = "/var/lib/vz/snippets/user_data_vm-${local.vmid}.yml"
  }
  provisioner "file" {
    source      = local_file.cloud_init_networking_data_file.filename
    destination = "/var/lib/vz/snippets/network_data_vm-${local.vmid}.yml"
  }
}

/* Configure Cloud-Init User-Data with custom config file */
resource "proxmox_vm_qemu" "ven" {

  depends_on = [null_resource.ven_cloud_init_config_files]

  target_node = var.target_node
  vmid        = local.vmid
  name        = var.vm_name
  desc        = "Created with Terraform"

  os_type  = var.vm_os_type
  cicustom = "user=local:snippets/user_data_vm-${local.vmid}.yml,network=local:snippets/network_data_vm-${local.vmid}.yml"

  onboot   = false
  vm_state = "stopped" # VM will be started later
  memory   = var.vm_memory
  sockets  = var.vm_sockets
  cores    = var.vm_cores
  cpu      = "host"
  scsihw   = "virtio-scsi-pci"
  boot     = "order=scsi0;scsi1"

  agent         = 1
  skip_ipv4     = false
  agent_timeout = var.agent_timeout

  serial {
    id   = 0
    type = "socket"
  }

  ipconfig0 = "ip=dhcp" # no-op, will be overridden by custom cloud-init template
  network {
    model   = "virtio"
    bridge  = var.vm_net0_bridge
    macaddr = var.vm_net0_mac_address
  }

  disks {
    scsi {
      // cloud-int drive
      scsi1 {
        cloudinit {
          storage = "local-lvm"
        }
      }
      // Boot disk will be modified later
      scsi0 {
        disk {
          size    = "10M"
          storage = "local-lvm"
        }
      }
    }
  }

  lifecycle {
    ignore_changes = [
      disks,
      vm_state,
    ]
  }
}

resource "null_resource" "import_boot_disk_from_file" {
  depends_on = [proxmox_vm_qemu.ven]

  connection {
    type     = "ssh"
    user     = var.pm_ssh_user
    password = var.pm_ssh_password
    host     = var.pm_ssh_host
    port     = var.pm_ssh_port
  }

  provisioner "remote-exec" {
    inline = [
      "qm set ${local.vmid} --scsi0 local-lvm:0,import-from=${var.vm_boot_disk_image_file}",
      "qm resize ${local.vmid} scsi0 ${var.vm_boot_disk_size}",
      "qm start ${local.vmid}",
    ]
  }
}

resource "null_resource" "wait_for_network" {
  depends_on = [
    proxmox_vm_qemu.ven
  ]

  connection {
    type     = "ssh"
    host     = var.pm_ssh_host
    port     = var.pm_ssh_port
    user     = var.pm_ssh_user
    password = var.pm_ssh_password
  }

  provisioner "remote-exec" {
    inline = [
      "until qm agent ${local.vmid} ping; do sleep 10; done",
      "qm agent ${local.vmid} network-get-interfaces",
    ]
  }
}

data "external" "vm_ip_address" {
  depends_on = [
    null_resource.wait_for_network,
  ]

  program = ["bash", "${var.working_directory}/scripts/get-ven-ip.bash"]

  query = {
    sshpass_path    = "${path.module}/scripts/sshpass.bash"
    pm_ssh_host     = var.pm_ssh_host
    pm_ssh_port     = var.pm_ssh_port
    pm_ssh_user     = var.pm_ssh_user
    pm_ssh_password = var.pm_ssh_password
    vm_id           = proxmox_vm_qemu.ven.vmid
  }
}