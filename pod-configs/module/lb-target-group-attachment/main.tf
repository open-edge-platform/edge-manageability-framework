# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Create Target group attachment
resource "aws_lb_target_group_attachment" "main" {
  for_each         = var.eks_nodes

  port             = var.service_node_port
  target_group_arn = var.target_group_arn
  target_id        = each.key
}

# Add security group rule when target_group_attachement is created
resource "aws_security_group_rule" "node_sg_rule" {
  count             = var.eks_node_sg_id == "" ? 0 : 1
  type              = "ingress"
  from_port         = var.service_node_port
  to_port           = var.service_node_port
  protocol          = "tcp"
  cidr_blocks       = var.ip_allow_list
  security_group_id = var.eks_node_sg_id
  description       = "Ingress for ${var.name} nodePort"
}
