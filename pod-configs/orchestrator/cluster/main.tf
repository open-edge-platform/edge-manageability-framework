# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Import VPC and subnets from VPC state

data "terraform_remote_state" "vpc" {
  backend = "s3"
  config = {
    bucket = var.vpc_terraform_backend_bucket
    key    = var.vpc_terraform_backend_key
    region = var.vpc_terraform_backend_region
  }
}

locals {
  private_subnets = values(data.terraform_remote_state.vpc.outputs.private_subnets)[*].id
  public_subnets  = values(data.terraform_remote_state.vpc.outputs.public_subnets)[*].id
  vpc_id          = data.terraform_remote_state.vpc.outputs.vpc_id
  cidr_blocks     = data.terraform_remote_state.vpc.outputs.cidr_blocks
  vpc_name        = data.terraform_remote_state.vpc.outputs.vpc_name
  smtp_from       = var.smtp_from == "" ? "${var.eks_cluster_name}@intel.com" : var.smtp_from
}

module "eks" {
  source                      = "../../module/eks"
  cluster_name                = var.eks_cluster_name
  aws_account_number          = var.aws_account_number
  aws_region                  = var.aws_region
  vpc_id                      = local.vpc_id
  subnets                     = local.private_subnets
  ip_allow_list               = local.cidr_blocks # Allow entire VPC to access the cluster
  eks_node_ami_id             = var.eks_node_ami_id
  volume_size                 = var.eks_volume_size
  volume_type                 = var.eks_volume_type
  eks_node_instance_type      = var.eks_node_instance_type
  desired_size                = var.eks_desired_size
  min_size                    = var.eks_min_size
  max_size                    = var.eks_max_size
  addons                      = var.eks_addons
  eks_version                 = var.eks_version
  max_pods                    = var.eks_max_pods
  additional_node_groups      = var.eks_additional_node_groups
  public_cloud                = var.public_cloud
  enable_cache_registry       = var.enable_cache_registry
  cache_registry              = var.cache_registry
  customer_tag                = var.customer_tag
  user_script_pre_cloud_init  = var.eks_user_script_pre_cloud_init
  user_script_post_cloud_init = var.eks_user_script_post_cloud_init
  http_proxy                  = var.eks_http_proxy
  https_proxy                 = var.eks_https_proxy
  no_proxy                    = var.eks_no_proxy
  eks_cluster_dns_ip          = var.eks_cluster_dns_ip
}

resource "time_sleep" "wait_eks" {
  depends_on      = [module.eks]
  create_duration = "20s"
}

data "aws_eks_cluster" "eks_cluster_data" {
  depends_on = [time_sleep.wait_eks]
  name       = var.eks_cluster_name
}

module "s3" {
  depends_on     = [time_sleep.wait_eks]
  source         = "../../module/s3"
  aws_accountid  = var.aws_account_number
  s3_prefix      = var.s3_prefix
  cluster_name   = var.eks_cluster_name
  create_tracing = var.s3_create_tracing
  import_buckets = var.import_s3_buckets
}

module "efs" {
  depends_on                          = [time_sleep.wait_eks]
  source                              = "../../module/efs"
  policy_name                         = var.efs_policy_name
  role_name                           = var.efs_role_name
  sg_name                             = var.efs_sg_name
  subnets                             = local.private_subnets
  policy_source                       = var.efs_policy_source
  aws_accountid                       = var.aws_account_number
  transition_to_ia                    = var.efs_transition_to_ia
  transition_to_primary_storage_class = var.efs_transition_to_primary_storage_class
  efs_sg_cidr_blocks                  = local.cidr_blocks # Allow VPC to access EFS
  cluster_name                        = var.eks_cluster_name
  vpc_id                              = local.vpc_id
  throughput_mode                     = var.efs_throughput_mode
}

module "aurora" {
  source                      = "../../module/aurora"
  vpc_id                      = local.vpc_id
  cluster_name                = var.eks_cluster_name
  subnet_ids                  = local.private_subnets
  ip_allow_list               = local.cidr_blocks # Allow entire VPC to access it
  availability_zones          = var.aurora_availability_zones
  instance_availability_zones = var.aurora_instance_availability_zones
  postgres_ver_major          = var.aurora_postgres_ver_major
  postgres_ver_minor          = var.aurora_postgres_ver_minor
  min_acus                    = var.aurora_min_acus
  max_acus                    = var.aurora_max_acus
  dev_mode                    = var.aurora_dev_mode
}

locals {
  # The actual database names on RDS, uses {namespace}-{db name}
  database_names = toset([
    for db_name, db in var.orch_databases : "${db.namespace}-${db_name}"
  ])
  # Set of unique database users
  database_users = toset([
    for db_name, db in var.orch_databases : db.user
  ])
  database_user_mapping = {
    for db_name, db in var.orch_databases : "${db.namespace}-${db_name}" => db.user
  }
}

module "aurora_database" {
  depends_on = [module.aurora]
  source     = "../../module/aurora-database"
  host       = module.aurora.host
  port       = module.aurora.port
  username   = module.aurora.username
  # "password" is deprecated, we use "password_id" to read the password from secretmanager
  # keep it for now for backward compatibility.
  password      = module.aurora.password
  password_id   = module.aurora.password_id
  databases     = local.database_names
  users         = local.database_users
  database_user = local.database_user_mapping
}

module "aurora_import" {
  depends_on       = [module.aurora_database, module.orch_init]
  source           = "../../module/aurora-import"
  for_each         = var.orch_databases
  host             = module.aurora.host
  host_reader      = module.aurora.host_reader
  port             = module.aurora.port
  eks_cluster_name = var.eks_cluster_name
  username         = each.value.user
  password         = module.aurora_database.user_password[each.value.user].result
  namespace        = each.value.namespace
  database         = each.key
}

module "kms" {
  # kms module creates K8s secrets, which depends on the namespaces created in orch_init
  depends_on         = [module.orch_init]
  source             = "../../module/kms"
  cluster_name       = var.eks_cluster_name
  aws_account_number = var.aws_account_number
}

module "orch_init" {
  count                         = var.enable_orch_init ? 1 : 0
  depends_on                    = [time_sleep.wait_eks]
  source                        = "../../module/orch-init"
  needed_namespaces             = var.needed_namespaces
  istio_namespaces              = var.istio_namespaces
  tls_cert                      = var.tls_cert
  tls_key                       = var.tls_key
  ca_cert                       = var.ca_cert
  sre_basic_auth_username       = var.sre_basic_auth_username
  sre_basic_auth_password       = var.sre_basic_auth_password
  sre_destination_secret_url    = var.sre_destination_secret_url
  sre_destination_ca_secret     = var.sre_destination_ca_secret
  webhook_github_netrc          = var.webhook_github_netrc
  smtp_user                     = var.smtp_user
  smtp_pass                     = var.smtp_pass
  smtp_url                      = var.smtp_url
  smtp_port                     = var.smtp_port
  smtp_from                     = local.smtp_from
  public_cloud                  = var.public_cloud
  auto_cert                     = var.auto_cert
  release_service_refresh_token = var.release_service_refresh_token
  efs_id                        = module.efs.efs.id
  cluster_name                  = var.eks_cluster_name
}

# This block executes only when `enable_eks_auth` is set to true
module "eks_auth" {
  count              = var.enable_eks_auth ? 1 : 0
  depends_on         = [time_sleep.wait_eks, module.orch_init]
  source             = "../../module/eks-auth"
  cluster_name       = var.eks_cluster_name
  aws_account_number = var.aws_account_number
  aws_region         = var.aws_region
  vpc                = local.vpc_name
  aws_roles          = var.aws_roles
}

module "ec2log" {
  count             = var.enable_ec2log ? 1 : 0
  depends_on        = [time_sleep.wait_eks]
  source            = "../../module/ec2log"
  cluster_name      = var.eks_cluster_name
  nodegroup_role    = module.eks.eks_nodegroup_role_name
  upload_file_list  = var.ec2log_file_list
  script            = var.ec2log_script
  s3_expire         = var.ec2log_s3_expire
  cloudwatch_expire = var.ec2log_cw_expire
  s3_prefix         = var.s3_prefix
}

module "aws_lb_controller" {
  depends_on   = [module.eks]
  source       = "../../module/aws-lb-controller"
  cluster_name = var.eks_cluster_name
}

module "gitea" {
  depends_on    = [module.eks, module.orch_init, module.aws_lb_controller]
  source        = "../../module/gitea"
  name          = "gitea"
  tls_cert_body = var.tls_cert
  tls_key       = var.tls_key
  aws_region    = var.aws_region
  cluster_name  = var.eks_cluster_name
  gitea_fqdn    = "gitea.${var.cluster_fqdn}"
  # Enable following variables when switching to RDS
  # gitea_database_endpoint    = module.aurora.host
  # gitea_database_username    = "gitea-gitea_user" # See aurora_database module
  # gitea_database_password    = module.aurora_database.user_password["gitea-gitea_user"].result
  # gitea_database             = "gitea-gitea"  # See aurora_database module
}
