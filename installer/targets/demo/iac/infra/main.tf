terraform {
  backend "s3" {}
}
variable "vpc_id" {
  type = string
  description = "VPC ID to deploy the infrastructure"
}
resource "null_resource" "main" {
  provisioner "local-exec" {
    command = <<EOT
      echo "Creates infra on ${var.vpc_id}!"
    EOT
  }
}

output "main" {
  value = "Hello from main!, vpc id is ${var.vpc_id}"
}
