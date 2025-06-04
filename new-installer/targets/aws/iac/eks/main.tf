# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

data "aws_caller_identity" "current" {}

data "aws_ami" "eks_node_ami" {
  most_recent = true
  owners      = ["602401143452"] # Amazon EKS AMI owner ID
  filter {
    name   = "name"
    values = ["amazon-eks-node-${var.eks_version}-*"]
  }
}

data "aws_subnet" "eks_subnet" {
  for_each = var.subnet_ids
  id       = each.key
}

locals {
  eks_nodegroup_role_name = "eks-node-${var.name}"
  kube_config_path = "/tmp/kubeconfig-${var.name}"
  aws_account_id = data.aws_caller_identity.current.account_id
  cas_namespace = "kube-system"
  cas_service_account = "cluster-autoscaler"
}

resource "aws_iam_role" "iam_role_eks_cluster" {
  name               = "eks-${var.name}"
  assume_role_policy = <<EOF
{
 "Version": "2012-10-17",
 "Statement": [
   {
   "Effect": "Allow",
   "Principal": {
    "Service": "eks.amazonaws.com"
   },
   "Action": "sts:AssumeRole"
   }
  ]
 }
EOF
}

resource "aws_iam_role_policy_attachment" "eks_cluster_AmazonEKSClusterPolicy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = aws_iam_role.iam_role_eks_cluster.name
}

resource "aws_iam_role_policy_attachment" "eks_cluster_AmazonEKSServicePolicy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSServicePolicy"
  role       = aws_iam_role.iam_role_eks_cluster.name
}

resource "aws_security_group" "eks_cluster" {
  name   = "eks-${var.name}"
  vpc_id = var.vpc_id
  description = "Security group for EKS cluster ${var.name}"
  tags = {
    Name     = "eks-${var.name}"
    environment = var.name
    customer = var.customer_tag
  }
}

# Allow HTTPS traffic from EKS subnets
resource "aws_security_group_rule" "eks_cluster_ingress" {
  for_each = data.aws_subnet.eks_subnet
  type              = "ingress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  security_group_id = aws_security_group.eks_cluster.id
  cidr_blocks       = each.value.cidr_block
  description       = "Allow HTTPS traffic from VPC"
}

# Allow EKS to connect to the internet(e.g., release service)
#trivy:ignore:AVD-AWS-0104
resource "aws_security_group_rule" "eks_cluster_egress" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "tcp"
  security_group_id = aws_security_group.eks_cluster.id
  cidr_blocks       = ["0.0.0.0/0"]
  description       = "Allow HTTPS traffic from VPC"
}

#trivy:ignore:AVD-AWS-0039 TODO: enable secret encryption
#trivy:ignore:AVD-AWS-0038 Disable control plane logging
resource "aws_eks_cluster" "eks_cluster" {
  name                      = var.name
  role_arn                  = aws_iam_role.iam_role_eks_cluster.arn
  version                   = var.eks_version
  enabled_cluster_log_types = ["api", "audit"]
  vpc_config { # Configure EKS with vpc and network settings
    security_group_ids      = [aws_security_group.eks_cluster.id]
    subnet_ids              = var.subnet_ids
    endpoint_private_access = true
    endpoint_public_access  = false
  }
  depends_on = [
    aws_iam_role_policy_attachment.eks_cluster_AmazonEKSClusterPolicy,
    aws_iam_role_policy_attachment.eks_cluster_AmazonEKSServicePolicy,
  ]
}

resource "null_resource" "wait_eks_complete" {
  provisioner "local-exec" {
    command = <<EOT
        set -eu

        # Wait the eks cluster to be available in 200 seconds.
        for i in $(seq 1 10); do
            if ! aws eks update-kubeconfig --name "${var.name}" --region "${var.region}" --kubeconfig "${local.kube_config_path}"; then
                sleep 20
            else
                break
            fi
        done

        test $i -le 10
EOT
  }

  depends_on = [aws_eks_cluster.eks_cluster]
}

resource "null_resource" "create_kubecnofig" {
  triggers = {
    always = timestamp()
  }
  provisioner "local-exec" {
    command = <<EOT
    aws eks update-kubeconfig --name "${var.name}" --region "${var.region}" --kubeconfig "${local.kube_config_path}"
    EOT
  }
  depends_on = [null_resource.wait_eks_complete]
}

resource "null_resource" "set_env" {
  provisioner "local-exec" {
    command = <<EOT
        set -eu
        kubectl set env ds aws-node --kubeconfig "${local.kube_config_path}" --context "arn:aws:eks:${var.region}:${local.aws_account_id}:cluster/${var.name}" -n kube-system WARM_PREFIX_TARGET=0
        kubectl set env ds aws-node --kubeconfig "${local.kube_config_path}" --context "arn:aws:eks:${var.region}:${local.aws_account_id}:cluster/${var.name}" -n kube-system WARM_IP_TARGET=2
        kubectl set env ds aws-node --kubeconfig "${local.kube_config_path}" --context "arn:aws:eks:${var.region}:${local.aws_account_id}:cluster/${var.name}" -n kube-system MINIMUM_IP_TARGET=0
EOT
  }
  depends_on = [null_resource.create_kubecnofig]
}

data "aws_eks_cluster" "eks_cluster_data" {
  name = var.name
  depends_on = [
    null_resource.set_env
  ]
}

# Creating IAM role for EKS nodes to work with other AWS Services.
resource "aws_iam_role" "eks_nodes" {
  name = local.eks_nodegroup_role_name

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
}

resource "aws_iam_role_policy_attachment" "AmazonEKSWorkerNodePolicy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.eks_nodes.name
}

resource "aws_iam_role_policy_attachment" "AmazonEKS_CNI_Policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.eks_nodes.name
}

resource "aws_iam_role_policy_attachment" "AmazonEC2ContainerRegistryReadOnly" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.eks_nodes.name
}

resource "aws_iam_role_policy_attachment" "AmazonEBSCSIDriverPolicy" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"
  role       = aws_iam_role.eks_nodes.name
}

resource "aws_iam_role_policy_attachment" "AmazonEFSCSIDriverPolicy" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonEFSCSIDriverPolicy"
  role       = aws_iam_role.eks_nodes.name
}

resource "aws_iam_role_policy_attachment" "AmazonSSMManagedInstanceCore" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
  role       = aws_iam_role.eks_nodes.name
}

resource "aws_launch_template" "eks_launch_template" {
  name = "eks-nodegroup-${var.name}-1"

  vpc_security_group_ids = [
    aws_security_group.eks_cluster.id,
    aws_eks_cluster.eks_cluster.vpc_config[0].cluster_security_group_id
  ]

  block_device_mappings {
    device_name = "/dev/xvda"

    ebs {
      volume_size = var.volume_size
      volume_type = var.volume_type
    }
  }

  image_id      = data.aws_ami.eks_node_ami.id
  instance_type = var.node_instance_type

  user_data = base64encode(templatefile("${path.module}/eks_cloud_init.tpl", {
    user_script_pre_cloud_init = var.user_script_pre_cloud_init
    user_script_post_cloud_init = var.user_script_post_cloud_init
    http_proxy = var.http_proxy
    https_proxy = var.https_proxy
    no_proxy = var.no_proxy
    region = var.region
    enable_cache_registry = var.enable_cache_registry
    cache_registry = var.cache_registry
    eks_endpoint = data.aws_eks_cluster.eks_cluster_data.endpoint
    eks_cluster_ca = data.aws_eks_cluster.eks_cluster_data.certificate_authority[0].data
    name = var.name
    eks_node_ami_id = data.aws_ami.eks_node_ami.id
    max_pods = var.max_pods
  }))
  metadata_options {
    http_tokens = "required"
  }

  tag_specifications {
    resource_type = "instance"
    tags = {
      "environment" : var.name
      "customer"    : var.customer_tag
      "Name"        : "eks-nodegroup-${var.name}-1"
    }
  }
}

resource "aws_launch_template" "additional_node_group_launch_template" {
  for_each = var.additional_node_groups
  name     = "eks-nodegroup-${var.name}-${each.key}"

  vpc_security_group_ids = [
    aws_security_group.eks_cluster.id,
    aws_eks_cluster.eks_cluster.vpc_config[0].cluster_security_group_id
  ]

  block_device_mappings {
    device_name = "/dev/xvda"

    ebs {
      volume_size = each.value.volume_size
      volume_type = each.value.volume_type
    }
  }

  image_id      = data.aws_ami.eks_node_ami.id
  instance_type = each.value.instance_type

  user_data = base64encode(templatefile("${path.module}/eks_cloud_init.tpl", {
    user_script_pre_cloud_init = var.user_script_pre_cloud_init
    user_script_post_cloud_init = var.user_script_post_cloud_init
    http_proxy = var.http_proxy
    https_proxy = var.https_proxy
    no_proxy = var.no_proxy
    region = var.region
    enable_cache_registry = var.enable_cache_registry
    cache_registry = var.cache_registry
    eks_endpoint = data.aws_eks_cluster.eks_cluster_data.endpoint
    eks_cluster_ca = data.aws_eks_cluster.eks_cluster_data.certificate_authority[0].data
    name = var.name
    eks_node_ami_id = data.aws_ami.eks_node_ami.id
    max_pods = var.max_pods
  }))
  metadata_options {
    http_tokens = "required"
  }

  tag_specifications {
    resource_type = "instance"
    tags = {
      "environment" : var.name
      "customer"    : var.customer_tag
      "Name"        : "eks-nodegroup-${var.name}-${each.key}"
    }
  }
}

resource "aws_eks_node_group" "nodegroup" {
  cluster_name = aws_eks_cluster.eks_cluster.name
  node_group_name      = "nodegroup-${var.name}"
  node_role_arn        = aws_iam_role.eks_nodes.arn
  subnet_ids           = var.subnet_ids
  force_update_version = true
  timeouts {
    update = "90m"
  }

  scaling_config {
    desired_size = var.desired_size
    min_size     = var.min_size
    max_size     = var.max_size
  }

  launch_template {
    name    = aws_launch_template.eks_launch_template.name
    version = aws_launch_template.eks_launch_template.latest_version
  }

  depends_on = [
    aws_iam_role_policy_attachment.AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.AmazonEC2ContainerRegistryReadOnly,
    aws_iam_role_policy_attachment.AmazonEBSCSIDriverPolicy,
    aws_launch_template.eks_launch_template
  ]
}

resource "aws_eks_node_group" "additional_node_group" {
  for_each             = var.additional_node_groups
  cluster_name         = aws_eks_cluster.eks_cluster.name
  node_group_name      = each.key
  node_role_arn        = aws_iam_role.eks_nodes.arn
  subnet_ids           = var.subnet_ids
  force_update_version = true
  timeouts {
    update = "90m"
  }

  scaling_config {
    desired_size = each.value.desired_size
    min_size     = each.value.min_size
    max_size     = each.value.max_size
  }

  launch_template {
    name    = aws_launch_template.additional_node_group_launch_template[each.key].name
    version = aws_launch_template.additional_node_group_launch_template[each.key].latest_version
  }

  labels = each.value.labels
  dynamic "taint" {
    for_each = each.value.taints
    content {
      key    = taint.key
      value  = taint.value.value
      effect = taint.value.effect
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.AmazonEC2ContainerRegistryReadOnly,
    aws_iam_role_policy_attachment.AmazonEBSCSIDriverPolicy,
    aws_launch_template.eks_launch_template
  ]
}

resource "aws_eks_addon" "addons" {
  for_each                    = { for addon in var.addons : addon.name => addon }
  cluster_name                = var.name
  addon_name                  = each.value.name
  addon_version               = each.value.version
  resolve_conflicts_on_create = "OVERWRITE"
  resolve_conflicts_on_update = "OVERWRITE"
  depends_on                  = [aws_eks_node_group.nodegroup]
  configuration_values        = each.value.configuration_values
}

resource "aws_iam_policy" "aws_load_balancer" {
  name   = "${var.name}-aws-load-balancer"
  policy = file("${path.module}/lb_policy.json")
}

resource "aws_iam_role_policy_attachment" "ELB_Controller" {
  policy_arn = aws_iam_policy.aws_load_balancer.arn
  role       = aws_iam_role.eks_nodes.name
}

data "tls_certificate" "cluster" {
  url = aws_eks_cluster.eks_cluster.identity[0].oidc[0].issuer
}

# OIDC identitfy provider
resource "aws_iam_openid_connect_provider" "cluster" {
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.cluster.certificates.0.sha1_fingerprint]
  url             = aws_eks_cluster.eks_cluster.identity[0].oidc[0].issuer
}

data "aws_instances" "eks_nodegroup_instances" {
  depends_on = [aws_eks_node_group.nodegroup]

  instance_tags = {
    "aws:eks:cluster-name" = var.name
    "customer"             = var.customer_tag
  }
}

# Policy for Kubernetes autoscaler
locals {
  oidc_provider_id = replace(aws_eks_cluster.eks_cluster.identity[0].oidc[0].issuer, "https://", "")
}

resource "aws_iam_policy" "cas_controller" {
  name   = "CASControllerPolicy-${var.name}"
  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "autoscaling:DescribeAutoScalingGroups",
        "autoscaling:DescribeAutoScalingInstances",
        "autoscaling:DescribeLaunchConfigurations",
        "autoscaling:DescribeScalingActivities",
        "autoscaling:DescribeTags",
        "ec2:DescribeImages",
        "ec2:DescribeInstanceTypes",
        "ec2:DescribeLaunchTemplateVersions",
        "ec2:GetInstanceTypesFromInstanceRequirements",
        "eks:DescribeNodegroup"
      ],
      "Resource": ["*"]
    },
    {
      "Effect": "Allow",
      "Action": [
        "autoscaling:SetDesiredCapacity",
        "autoscaling:TerminateInstanceInAutoScalingGroup"
      ],
      "Resource": ["*"]
    }
  ]
}
  EOF
}

resource "aws_iam_role" "cas_controller" {
  name               = "CASControllerRole-${var.name}"
  assume_role_policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Federated": "arn:aws:iam::${data.aws_caller_identity.current.account_id}:oidc-provider/${local.oidc_provider_id}"
            },
            "Action": "sts:AssumeRoleWithWebIdentity",
            "Condition": {
                "StringEquals": {
                    "${local.oidc_provider_id}:aud": "sts.amazonaws.com",
                    "${local.oidc_provider_id}:sub": "system:serviceaccount:${local.cas_namespace}:${local.cas_service_account}"
                }
            }
        }
    ]
}
  EOF
}

resource "aws_iam_role_policy_attachment" "cas_controller" {
  role       = aws_iam_role.cas_controller.name
  policy_arn = aws_iam_policy.cas_controller.arn
}

# Creating IAM role for cert-manager.
resource "aws_iam_role" "certmgr" {
  name = "certmgr-${var.name}"

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
    },
    {
        "Effect": "Allow",
        "Principal": {
            "AWS": "arn:aws:iam::${local.aws_account_id}:role/eks-node-${var.name}"
        },
        "Action": "sts:AssumeRole"
    }
  ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "certmgr_AmazonSSMManagedInstanceCore" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
  role       = aws_iam_role.certmgr.name
}

resource "aws_iam_policy" "certmgr_acm_sync" {
  name   = "certmgr_acm_sync-${var.name}"
  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "acm:DescribeCertificate",
                "acm:RemoveTagsFromCertificate",
                "acm:GetCertificate",
                "acm:UpdateCertificateOptions",
                "acm:ListCertificates",
                "acm:AddTagsToCertificate",
                "acm:ImportCertificate",
                "acm:RenewCertificate",
                "acm:ListTagsForCertificate"
            ],
            "Resource": "*"
        }
    ]
}
  EOF
}

resource "aws_iam_role_policy_attachment" "certmgr_acm_sync_certmgr" {
  policy_arn = aws_iam_policy.certmgr_acm_sync.arn
  role       = aws_iam_role.certmgr.name
}

resource "aws_iam_role_policy_attachment" "certmgr_acm_sync_eks_node" {
  policy_arn = aws_iam_policy.certmgr_acm_sync.arn
  role       = aws_iam_role.eks_nodes.name
}

resource "aws_iam_policy" "certmgr_write_route53" {
  name   = "certmgr_write_route53-${var.name}"
  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "route53:GetChange",
            "Resource": "arn:aws:route53:::change/*"
        },
        {
            "Effect": "Allow",
            "Action": [
                "route53:ChangeResourceRecordSets",
                "route53:ListResourceRecordSets"
            ],
            "Resource": "arn:aws:route53:::hostedzone/*"
        },
        {
            "Effect": "Allow",
            "Action": [
                "route53:ListHostedZonesByName",
                "route53:ListHostedZones",
                "route53:GetHostedZone",
                "route53:ListTagsForResource",
                "route53:ListTagsForResources"
            ],
            "Resource": "*"
        }
    ]
}
  EOF
}

resource "aws_iam_role_policy_attachment" "certmgr_write_route53" {
  policy_arn = aws_iam_policy.certmgr_write_route53.arn
  role       = aws_iam_role.certmgr.name
}
