# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "null_resource" "wait_traefik_alb" {
  triggers = {
    always = timestamp()
  }
  provisioner "local-exec" {
      command = <<EOT
echo "Waiting for ALB to be ready: traefik"
start_time=$(date +%s)
while true; do
    current_time=$(date +%s)
    if [ $((current_time - start_time)) -gt ${var.timeout} ]; then
        echo "Timeout waiting for ALB to be ready: traefik"
        exit 1
    fi
    status=$(aws elbv2 describe-load-balancers --load-balancer-arns ${var.traefik_alb_arn} --query "LoadBalancers[0].State.Code" --output text)
    if [ "$status" = "active" ]; then
        echo "ALB is ready: traefik"
        sleep 10
        break
    fi
    echo "Waiting for ALB to be ready: traefik"
    sleep 10
done
EOT
    }
}

resource "null_resource" "wait_argocd_alb" {
  triggers = {
    always = timestamp()
  }
  provisioner "local-exec" {
      command = <<EOT
echo "Waiting for ALB to be ready: argocd"
start_time=$(date +%s)
while true; do
    current_time=$(date +%s)
    if [ $((current_time - start_time)) -gt ${var.timeout} ]; then
        echo "Timeout waiting for ALB to be ready: argocd"
        exit 1
    fi
    status=$(aws elbv2 describe-load-balancers --load-balancer-arns ${var.argocd_alb_arn} --query "LoadBalancers[0].State.Code" --output text)
    if [ "$status" = "active" ]; then
        echo "ALB is ready: argocd"
        sleep 10
        break
    fi
    echo "Waiting for ALB to be ready: argocd"
    sleep 10
done
EOT
    }
}

# https://docs.aws.amazon.com/waf/latest/APIReference/API_CreateWebACL.html
resource "null_resource" "additional_wait" {
  triggers = {
    always = timestamp()
  }
  provisioner "local-exec" {
      command = <<EOT
echo "Wait 1 more minutes for the ALB to be ready"
sleep 60
EOT
  }
}
