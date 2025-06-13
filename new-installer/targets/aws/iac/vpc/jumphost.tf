# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  jumphost_instance_type = "t3.medium"
}

resource "aws_key_pair" "jumphost_instance_launch_key" {
  key_name   = "${var.name}-jumphost-key"
  public_key = var.jumphost_instance_ssh_key_pub
}

resource "aws_iam_role" "ec2" {
  name               = "${var.name}-jumphost"
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
  tags = {
    Creator = "terraform"
    Module  = path.module
    Name    = "${var.name}-jumphost"
    VPC     = var.name
  }
}

data "aws_iam_policy_document" "eks_cluster_access_policy_document" {
  statement {
    actions = [
      "eks:ListNodegroups",
      "eks:UntagResource",
      "eks:ListTagsForResource",
      "eks:DescribeNodegroup",
      "eks:TagResource",
      "eks:AccessKubernetesApi",
      "eks:DescribeCluster"
    ]
    resources = ["arn:aws:eks:${var.region}::cluster/${var.name}"]
    effect    = "Allow"
  }
}

resource "aws_iam_policy" "eks_cluster_access_policy" {
  name        = "${var.name}-jump-eks-cluster-access-policy"
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
  name = "${var.name}-jumphost"
  role = aws_iam_role.ec2.name
}

data "aws_ami" "jumphost" {
  most_recent = true
  owners      = ["099720109477"]

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }
}

resource "aws_instance" "jumphost" {
  ami                    = data.aws_ami.jumphost.id
  instance_type          = local.jumphost_instance_type
  key_name               = aws_key_pair.jumphost_instance_launch_key.key_name
  subnet_id              = aws_subnet.public_subnet[var.jumphost_subnet].id
  vpc_security_group_ids = [aws_security_group.jumphost.id]
  iam_instance_profile   = aws_iam_instance_profile.ec2.name
  user_data = templatefile(
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
    VPC  = var.name
    Name = "${var.name}-jump-root"
  }
  tags = {
    VPC  = var.name
    Name = "${var.name}-jump"
  }
  metadata_options {
    http_tokens = "required"
  }
}

resource "aws_security_group" "jumphost" {
  description = "Main security group for the jump host"
  name_prefix = "${var.name}-jump-sg"
  vpc_id      = aws_vpc.main.id

  lifecycle {
    create_before_destroy = true
  }

  tags = {
    VPC  = var.name
    Name = "${var.name}-jumphost"
  }
}

resource "aws_vpc_security_group_egress_rule" "jumphost_egress_private" {
  security_group_id = aws_security_group.jumphost.id
  from_port         = 0
  to_port           = 0
  ip_protocol       = "-1"
  cidr_ipv4         = var.cidr_block
  description       = "Allow egress traffic only to private subnets"
}

#trivy:ignore:AVD-AWS-0104
resource "aws_vpc_security_group_egress_rule" "jumphost_egress_https" {
  security_group_id = aws_security_group.jumphost.id
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
  cidr_ipv4         = "0.0.0.0/0"
  description       = "Allow traffic to the endpoints and repos"
}

resource "aws_vpc_security_group_ingress_rule" "jumphost_ingress_ssh" {
  for_each          = var.jumphost_ip_allow_list
  security_group_id = aws_security_group.jumphost.id
  from_port         = 22
  to_port           = 22
  ip_protocol       = "tcp"
  cidr_ipv4         = [each.value]
  description       = "Allow SSH access"
}

resource "aws_eip" "jumphost" {
  domain = "vpc"
  tags = {
    Name = "${var.name}-jumphost"
    VPC  = var.name
  }
}

resource "aws_eip_association" "jumphost" {
  allocation_id = aws_eip.jumphost.id
  instance_id   = aws_instance.jumphost.id
}
