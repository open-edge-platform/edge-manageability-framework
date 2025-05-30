# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Feature flags to enable or disable certain modules
variable "enable_orch_init" {
  default = true
}
variable "enable_eks_auth" {
  default = false
}

# Required variables
variable "vpc_terraform_backend_bucket" {
  description = "The Terraform S3 bucket to import VPC state"
}
variable "vpc_terraform_backend_key" {
  description = "The Terraform S3 key to import VPC state"
}
variable "vpc_terraform_backend_region" {
  description = "The Terraform S3 region to import VPC state"
}
variable "eks_cluster_name" {}
variable "aws_account_number" {}
variable "aws_region" {}
variable "tls_cert" {
  description = "The content of TLS certificate for Orchestrator services"
}
variable "tls_key" {
  description = "The content of TLS key for Orchestrator services"
}
variable "ca_cert" {
  description = "The content of CA certificate for Orchestrator services"
}
variable "aurora_availability_zones" {
  type        = set(string)
  description = "Availability zones to associate to the RDS cluster."
  validation {
    condition     = length(var.aurora_availability_zones) >= 3
    error_message = "Aurora requires a minimum of 3 AZs."
  }
}
variable "aurora_instance_availability_zones" {
  type        = set(string)
  description = "Availability zones to associate to the RDS instance."
  validation {
    condition     = length(var.aurora_instance_availability_zones) >= 1
    error_message = "At least 1 AZ for RDS instance."
  }
}
variable "cluster_fqdn" {
  type        = string
  description = "The FQDN of the cluster"
}

# Optional variables
variable "eks_node_ami_id" {
  default = "ami-09ea311630482acd7"
}
variable "eks_volume_size" {
  default = 20
}
variable "eks_volume_type" {
  default = "gp3"
}
variable "eks_node_instance_type" {
  default = "t3.2xlarge"
}
variable "eks_desired_size" {
  default = 1
}
variable "eks_min_size" {
  default = 1
}
variable "eks_max_size" {
  default = 1
}
variable "eks_addons" {
  type = list(object({
    name                 = string
    version              = string
    configuration_values = optional(string, "")
  }))
  default = [
    {
      name    = "aws-ebs-csi-driver",
      version = "v1.39.0-eksbuild.1"
    },
    {
      name    = "vpc-cni",
      version = "v1.19.2-eksbuild.1"
      configuration_values = "{\"enableNetworkPolicy\": \"true\", \"nodeAgent\": {\"healthProbeBindAddr\": \"8163\", \"metricsBindAddr\": \"8162\"}}"
    },
    {
      name    = "aws-efs-csi-driver"
      version = "v2.1.4-eksbuild.1"
    }
  ]
}
variable "eks_version" {
  default = "1.32"
}
variable "eks_max_pods" {
  default = 58
}
variable "public_cloud" {
  type    = bool
  default = true
}
variable "enable_cache_registry" {
  type    = string
  default = "false"
}
variable "cache_registry" {
  type    = string
  default = ""
}
variable "needed_namespaces" {
  type    = list(string)
  default = ["orch-sre", "cattle-system", "orch-boots", "fleet-default", "argocd", "orch-secret"]
}
variable "istio_namespaces" {
  type    = list(string)
  default = ["orch-infra", "orch-app", "orch-cluster", "orch-ui", "orch-platform", "orch-gateway"]
}
variable "webhook_github_netrc" {
  description = "Content of netrc file which contains access token to connect to GitHub."
  default     = ""
}
variable "s3_prefix" {
  type        = string
  default     = ""
  description = "The prefix which attach to S3 buckets"
}
variable "s3_create_tracing" {
  type    = bool
  default = false
}
variable "efs_policy_name" {
  default = "EFS_CSI_Driver_Policy"
}
variable "efs_role_name" {
  default = "EFS_CSI_DriverRole"
}
variable "efs_sg_name" {
  default = "efs-nfs"
}
variable "efs_policy_source" {
  default = "https://raw.githubusercontent.com/kubernetes-sigs/aws-efs-csi-driver/v1.5.4/docs/iam-policy-example.json"
}
variable "efs_transition_to_ia" {
  default = "AFTER_7_DAYS"
}
variable "efs_transition_to_primary_storage_class" {
  default = "AFTER_1_ACCESS"
}
variable "aurora_postgres_ver_major" {
  type    = string
  default = "14"
}
variable "aurora_postgres_ver_minor" {
  type    = string
  default = "9"
}
variable "aurora_min_acus" {
  # 1 ACU ~= 2GB memory
  type        = number
  default     = "0.5"
  description = "Minimum of ACUs for Aurora instances, 1 ACU ~= 2GB memory."
}
variable "aurora_max_acus" {
  type        = number
  default     = 2
  description = "Maximum of ACUs for Aurora instances, 1 ACU ~= 2GB memory."
}
variable "aurora_dev_mode" {
  type        = bool
  default     = true
  description = <<EOT
Development mode, apply the following settings when true:
- Disable deletion protection
- Skips final snapshot when delete
- Make backup retention period to 7 days(30 days for production)
- Applies changes immediately instead of update the cluster during the maintenance window.
  EOT
}

variable "orch_databases" {
  description = "Databases to be created"
  type = map(object({
    namespace = string
    user      = string
  }))
  default = {
    "app-orch-catalog" : {
      namespace : "orch-app"
      user : "app-orch-catalog_user"
    },
    "inventory" : {
      namespace : "orch-infra"
      user : "orch-infra-system-inventory_user"
    },
    "platform-keycloak" : {
      namespace : "orch-platform"
      user : "orch-platform-system-platform-keycloak_user"
    },
    "vault" : {
      namespace : "orch-platform"
      user : "orch-platform-system-vault_user"
    },
    "alerting" : {
      namespace : "orch-infra"
      user : "orch-infra-system-alerting_user"
    },
    "mps" : {
      namespace : "orch-infra"
      user: "orch-infra-system-mps_user"
    },
    "rps" : {
      namespace : "orch-infra"
      user: "orch-infra-system-rps_user"
    },
    # Uncomment following when using RDS for Gitea
    # "gitea": {
    #   namespace : "gitea"
    #   user: "gitea-gitea_user"
    # },
  }
}

variable "aws_roles" {
  type    = list(string)
  default = ["AWSReservedSSO_AWSAdministratorAccess_933fc287558617cc", "AWSReservedSSO_Developer_EKS_054305100460_52b02cdf70e84917"]
}

variable "efs_throughput_mode" {
  type    = string
  default = "bursting"
}

variable "argocd_repos" {
  type        = list(string)
  description = "List of argo cd repos to be created for a cluster"
  default     = ["edge-manageability-framework"]
}

variable "eks_additional_iam_policies" {
  type = set(string)
  default = []
}

variable "eks_additional_node_groups" {
  type = map(object({
    desired_size : number
    min_size : number
    max_size : number
    taints : map(object({
      value : string
      effect : string
    }))
    labels : map(string)
    instance_type : string
    volume_size : number
    volume_type : string
  }))
  default = {
    "observability" : {
      desired_size = 1
      min_size     = 1
      max_size     = 1
      labels = {
        "node.kubernetes.io/custom-rule" : "observability"
      }
      taints = {
        "node.kubernetes.io/custom-rule" : {
          value  = "observability"
          effect = "NO_SCHEDULE"
        }
      }
      instance_type = "t3.2xlarge"
      volume_size   = 20
      volume_type   = "gp3"
    }
  }
}

variable "sre_basic_auth_username" {
  type    = string
  default = ""
}

variable "sre_basic_auth_password" {
  type    = string
  default = ""
}

variable "sre_destination_secret_url" {
  type    = string
  default = ""
}

variable "sre_destination_ca_secret" {
  type    = string
  default = ""
}

variable "smtp_user" {
  description = "SMTP server username"
  type        = string
  default     = "29761a7bdbcceb6b"
}

variable "smtp_pass" {
  description = "SMTP server password"
  type        = string
  default     = ""
}

variable "smtp_url" {
  description = "SMTP server address"
  type        = string
  default     = "r01s32-r01.igk.intel.com"
}

variable "smtp_port" {
  description = "SMTP server port"
  type        = number
  default     = 587
}

variable "smtp_from" {
  description = "SMTP from header"
  type        = string
  default     = ""
}

variable "auto_cert" {
  type    = bool
  default = false
}

variable "enable_ec2log" {
  type    = bool
  default = true
}

variable "ec2log_file_list" {
  type    = string
  default = "/var/log/messages* /var/log/aws-routed-eni/* /var/log/dmesg /tmp/kubelet.log /tmp/free.log /tmp/df.log /tmp/top.log"
}

variable "ec2log_script" {
  type    = string
  default = "sudo journalctl -xeu kubelet >/tmp/kubelet.log; free >/tmp/free.log; df -h >/tmp/df.log; top -b -n 3 >/tmp/top.log"
}

variable "ec2log_s3_expire" {
  description = "Expiration period in days for the uploaded logs"
  type        = number
  default     = 30
}

variable "ec2log_cw_expire" {
  description = "Retention period in days for the CloudWatch log group for the Lambda function"
  type        = number
  default     = 7
}

variable "release_service_refresh_token" {
  type        = string
  description = "Refresh token to renew release service token"
}

variable "customer_tag" {
  description = "For customers to specify a tag for AWS resources"
  type = string
  default = ""
}

variable "import_s3_buckets" {
  type    = bool
  default = false
}

variable "eks_user_script_pre_cloud_init" {
  type = string
  default = ""
  description = "Script to run before cloud-init"
}

variable "eks_user_script_post_cloud_init" {
  type = string
  default = ""
  description = "Script to run after cloud-init"
}

variable "eks_http_proxy" {
  type = string
  default = ""
  description = "HTTP proxy to use for EKS nodes"
}

variable "eks_https_proxy" {
  type = string
  default = ""
  description = "HTTPS proxy to use for EKS nodes"
}

variable "eks_no_proxy" {
  type = string
  default = ""
  description = "No proxy to use for EKS nodes"
}

variable "eks_cluster_dns_ip" {
  type = string
  default = ""
  description = "IP address of the DNS server for the cluster, leave empty to use the default DNS server"
}

