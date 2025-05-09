terraform {
  backend "s3" {}
}

variable "name" {
  description = "Name of the network"
  type        = string
  default     = "demo-network"
}

variable "cidr" {
  description = "CIDR block for the network"
  type        = string
  default     = ""
}

resource "null_resource" "network" {
  provisioner "local-exec" {
    command = <<EOT
      echo "Creating network ${var.name} with CIDR ${var.cidr}"
    EOT
  }
}

# We will pass this to next stage
output "vpc_id" {
  description = "VPC ID"
  value       = "vpc-12345678"
}
