# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_key_pair" "jumphost_instance_launch_key" {
  key_name   = "${var.vpc_id}-jumphost-key"
  public_key = var.jumphost_instance_ssh_key_pub
}

resource "aws_iam_role" "ec2" {
  name               = "${var.vpc_name}-jumphost"
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF
permissions_boundary = var.permissions_boundary != "" ? var.permissions_boundary : null
  tags = {
    Creator = "terraform"
    Module  = path.module
  }
}

data "aws_region" "current" {}

data "aws_iam_policy_document" "eks_cluster_access_policy_document" {
  statement {
    actions = ["eks:ListNodegroups",
              "eks:UntagResource",
              "eks:ListTagsForResource",
              "eks:DescribeNodegroup",
              "eks:TagResource",
              "eks:AccessKubernetesApi",
              "eks:DescribeCluster"]
    resources = ["arn:aws:eks:${data.aws_region.current.name}::cluster/${var.vpc_name}"]
    effect = "Allow"
  }
}

resource "aws_iam_policy" "eks_cluster_access_policy" {
   name        = "${var.vpc_name}-jump-eks-cluster-access-policy"
   description = "Allow permissions to access EKS cluster"
   policy      = data.aws_iam_policy_document.eks_cluster_access_policy_document.json
}

resource "aws_iam_role_policy_attachment" "eks_cluster_access" {
  role       = aws_iam_role.ec2.name
  policy_arn = aws_iam_policy.eks_cluster_access_policy.arn
}

resource "aws_iam_role_policy_attachment" "AmazonSSMManagedInstanceCore" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
  role       = aws_iam_role.ec2.name
}

resource "aws_iam_instance_profile" "ec2" {
  name = "${var.vpc_name}-jumphost"
  role = aws_iam_role.ec2.name
}

data "aws_subnet" "jumphost" {
  vpc_id            = var.vpc_id
  cidr_block        = var.subnet.cidr_block
  availability_zone = var.subnet.az
  tags = {
    Name : var.subnet.name
  }
}

resource "aws_instance" "jumphost" {
  ami                    = var.jumphost_ami_id
  instance_type          = var.jumphost_instance_type
  key_name               = aws_key_pair.jumphost_instance_launch_key.key_name
  subnet_id              = data.aws_subnet.jumphost.id
  vpc_security_group_ids = [aws_security_group.jumphost.id]
  iam_instance_profile   = aws_iam_instance_profile.ec2.name
  user_data              = templatefile(
    "${path.module}/jumphost_user_data.tftpl",
    {
      region                        = var.region
      production                    = var.production
      jumphost_instance_ssh_key_pub = var.jumphost_instance_ssh_key_pub
    }
  )
  root_block_device {
    encrypted = true
  }
  volume_tags = {
    VPC  = var.vpc_name
    Name = "${var.vpc_name}-jump-root"
  }
  tags = {
    VPC  = var.vpc_name
    Name = "${var.vpc_name}-jump"
  }
  metadata_options {
    http_tokens = "required"
  }
}

resource "aws_security_group" "jumphost" {
  description = "Main security group for the jump host"
  name_prefix = "${var.vpc_name}-jump-sg"
  vpc_id      = var.vpc_id

  lifecycle {
    create_before_destroy = true
  }

  tags = {
    VPC  = var.vpc_name
    Name = "${var.vpc_name}-jumphost"
  }
}

resource "aws_security_group_rule" "jumphost_egress_private" {
  for_each = var.egress_ip_allow_list
  type        = "egress"
  from_port   = 0
  to_port     = 0
  protocol    = -1
  cidr_blocks = [each.key]
  description = "Allow egress traffic only to private subnets"
  security_group_id = aws_security_group.jumphost.id
}

resource "aws_security_group_rule" "jumphost_egress_https" {
  type        = "egress"
  from_port   = 443
  to_port     = 443
  protocol    = "tcp"
  cidr_blocks = ["0.0.0.0/0"]
  description = "Allow traffic to the endpoints and repos"
  security_group_id = aws_security_group.jumphost.id
}

resource "aws_security_group_rule" "jumphost_ingress_ssh" {
  for_each = var.ip_allow_list
  type        = "ingress"
  from_port   = 22
  to_port     = 22
  protocol    = "tcp"
  cidr_blocks = [each.key]
  description = "Allow SSH access"
  security_group_id = aws_security_group.jumphost.id
}


resource "aws_eip" "jumphost" {
  domain = "vpc"
  tags = {
    Name = "${var.vpc_name}-jumphost"
    VPC  = var.vpc_name
  }
}

resource "aws_eip_association" "jumphost" {
  allocation_id = aws_eip.jumphost.id
  instance_id   = aws_instance.jumphost.id
}
