#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# This script is used to upgrade the version of an EKS cluster. Refer to the 'usage_upgrade()' function for more details.
set -ue
set -o pipefail

. utils/lib/common.sh

# Global variables to be updated
ENV_NAME="${ENV_NAME:-}"
AWS_REGION="${AWS_REGION:-}"
AWS_ACCOUNT="${AWS_ACCOUNT:-}"
CUSTOMER_STATE_PREFIX="${CUSTOMER_STATE_PREFIX:-}"
BUCKET_NAME="${BUCKET_NAME:-}"
BUCKET_REGION="${BUCKET_REGION:-}"
PARENT_DOMAIN="${PARENT_DOMAIN:-}"
PARENT_ZONE="${PARENT_ZONE:-}"
HOST_NAME="${HOST_NAME:-}"
CUSTOMER_TAG="${CUSTOMER_TAG:-}"
ENABLE_CACHE_REGISTRY="${ENABLE_CACHE_REGISTRY:-false}"
TLS_CERT="${TLS_CERT:-}"
TLS_KEY="${TLS_KEY:-}"
TLS_CA_CERT="${TLS_CA_CERT:-}"
SOCKS_PROXY="${SOCKS_PROXY:-}"
FULLCHAIN="${FULLCHAIN:-}"
CHAIN="${CHAIN:-}"
PRIVKEY="${PRIVKEY:-}"
AUTO_APPROVE="${AUTO_APPROVE:-}"
VALUES="${VALUES:-}"
SAVE_DIR_S3="${SAVE_DIR_S3:-}"
PROVISION_CONTEXT="${PROVISION_CONTEXT:-true}"
LICENSE_CUSTOMERID="${LICENSE_CUSTOMERID:-}"
LICENSE_PRODUCTKEY="${LICENSE_PRODUCTKEY:-}"
JUMPHOST_SSHKEY="${JUMPHOST_SSHKEY:-}"
VPC_CIDR="${VPC_CIDR:-}"
RS_TOKEN="${RS_TOKEN:-"release-service-token-prod"}"
SECRET_NAME="licensing-secret"
SECRET_NAMESPACE="gateway-system"
VALUES_TO_VALIDATE=("$LICENSE_CUSTOMERID:customerid" "$LICENSE_PRODUCTKEY:productkey")
AZUREAD_REFRESH_TOKEN=${AZUREAD_REFRESH_TOKEN:-}
VPC_ID=$(get_vpc_id)
CREATE_ROOT_DOMAIN="false"
PARENT_DOMAIN="${PARENT_DOMAIN:-}"

cluster_backend() {
    cat <<EOF
bucket = "$BUCKET_NAME"
key    = "$AWS_REGION/cluster/$ENV_NAME"
region = "$BUCKET_REGION" # region of the S3 bucket to store the TF state
EOF
}

check_eks_exist() {
    if aws eks describe-cluster --region ${AWS_REGION} --name ${ENV_NAME} &> /dev/null; then
        return 0
    else
        return 1
    fi
}

get_azs() {
    azs=$(aws ec2 describe-availability-zones --region $AWS_REGION --output json | jq -r '.AvailabilityZones[].ZoneName' | head -3 | sort)

    n=$(echo "$azs" | wc -l)
    if [[ $n != 3 ]]; then
        echo "Error: Cannot get three AWS available zone in the region $AWS_REGION. "
        exit 1
    fi

    echo "$azs"
}

get_eks_node_ami() {
    ami=""
    if check_eks_exist; then
        ami=$(aws eks describe-nodegroup --region ${AWS_REGION} --cluster-name ${ENV_NAME} --nodegroup-name nodegroup-${ENV_NAME} | jq -r '.nodegroup.releaseVersion')
    fi

    if [[ -z "$ami" ]] || [[ "$ami" == "null" ]]; then
        ami="$(get_eksnode_ami $EKS_VERSION)"
    fi

    echo $ami
}


get_aurora_ins_azs() {
    aurora_azs=($1)

    aurora_ins_azs="["
    n=$(( $NUM_RDS_INSTANCES - 1 ))
    i_az=0
    for i in $(seq 0 $n); do
        if [[ $i -gt 0 ]]; then
            aurora_ins_azs="${aurora_ins_azs},"
        fi
        aurora_ins_azs="${aurora_ins_azs}\"${aurora_azs[$i_az]}\""

        (( i_az ++ )) || true
        [[ $i_az -ge 3 ]] && i_az=0
    done
    aurora_ins_azs="${aurora_ins_azs}]"

    echo $aurora_ins_azs
}


cluster_variable() {
    s=$(get_azs)
    azs=($s)
    aurora_ins_azs="$(get_aurora_ins_azs "$s")"
    VPC_TERRAFORM_BACKEND_KEY="${AWS_REGION}/vpc/${ENV_NAME}"

    # We will not create VPC if the --skip-apply-vpc is set
    # and the VPC_ID is not empty.
    if [[ -n "$VPC_ID" ]] && $SKIP_APPLY_VPC; then
        VPC_TERRAFORM_BACKEND_KEY="${AWS_REGION}/vpc/${VPC_ID}"
    fi
    LT_ID=$(aws eks describe-nodegroup \
      --cluster-name $ENV_NAME \
      --nodegroup-name nodegroup-$ENV_NAME \
      --region $AWS_REGION \
      --query 'nodegroup.launchTemplate.id' \
      --output text)
    
    LT_VERSION=$(aws eks describe-nodegroup \
      --cluster-name $ENV_NAME \
      --nodegroup-name nodegroup-$ENV_NAME \
      --region $AWS_REGION \
      --query 'nodegroup.launchTemplate.version' \
      --output text)
    
    OUT=$(aws ec2 describe-launch-template-versions \
      --launch-template-id $LT_ID \
      --versions $LT_VERSION \
      --region $AWS_REGION \
      --query 'LaunchTemplateVersions[0].LaunchTemplateData.BlockDeviceMappings[0].Ebs.[VolumeSize,VolumeType]' \
      --output text)
    
    VOL_SIZE=$(echo $OUT | awk '{print $1}')
    VOL_TYPE=$(echo $OUT | awk '{print $2}')

    LT_ID_OBS=$(aws eks describe-nodegroup \
      --cluster-name $ENV_NAME \
      --nodegroup-name observability \
      --region $AWS_REGION \
      --query 'nodegroup.launchTemplate.id' \
      --output text)
    
    LT_VERSION_OBS=$(aws eks describe-nodegroup \
      --cluster-name $ENV_NAME \
      --nodegroup-name observability \
      --region $AWS_REGION \
      --query 'nodegroup.launchTemplate.version' \
      --output text)
    
    OUT_OBS=$(aws ec2 describe-launch-template-versions \
      --launch-template-id $LT_ID_OBS \
      --versions $LT_VERSION_OBS \
      --region $AWS_REGION \
      --query 'LaunchTemplateVersions[0].LaunchTemplateData.[InstanceType,BlockDeviceMappings[0].Ebs.VolumeSize,BlockDeviceMappings[0].Ebs.VolumeType]' \
      --output text)
    
    INSTANCE_TYPE_OBS=$(echo $OUT_OBS | awk '{print $1}')
    VOL_SIZE_OBS=$(echo $OUT_OBS | awk '{print $2}')
    VOL_TYPE_OBS=$(echo $OUT_OBS | awk '{print $3}')

    FULLCHAIN="fullchain-${AWS_ACCOUNT}-${ENV_NAME}.pem"
    CHAIN="chain-${AWS_ACCOUNT}-${ENV_NAME}.pem"
    PRIVKEY="privkey-${AWS_ACCOUNT}-${ENV_NAME}.pem"
    tls_cert_content=$(cat ${SAVE_DIR}/${FULLCHAIN})
    ca_cert_content=$(cat ${SAVE_DIR}/${CHAIN})
    tls_key_content=$(cat ${SAVE_DIR}/${PRIVKEY})
    cat <<EOF
vpc_terraform_backend_bucket       = "$BUCKET_NAME"
vpc_terraform_backend_key          = "${VPC_TERRAFORM_BACKEND_KEY}"
vpc_terraform_backend_region       = "${BUCKET_REGION}" # region of the S3 bucket to store the TF state
eks_cluster_name                   = "$ENV_NAME"
aws_account_number                 = "$AWS_ACCOUNT"
eks_volume_size                    = $VOL_SIZE
eks_desired_size                   = 3
eks_min_size                       = 1
eks_max_size                       = 5
eks_node_ami_id                    = "$(get_eks_node_ami)"
eks_volume_type                    = "$VOL_TYPE"
aws_region                         = "${AWS_REGION}"
aurora_availability_zones          = ["${azs[0]}", "${azs[1]}", "${azs[2]}"]
aurora_instance_availability_zones = ${aurora_ins_azs}
aurora_dev_mode                    = false
public_cloud                       = true
efs_throughput_mode                = "elastic"
cluster_fqdn                       = "${ROOT_DOMAIN}"
enable_cache_registry              = ${ENABLE_CACHE_REGISTRY}
enable_pull_through_cache_proxy    = ${ENABLE_CACHE_REGISTRY}
cache_registry                     = "https://docker-cache.${ROOT_DOMAIN}"
cas_namespace                      = "kube-system"
cas_service_account                = "cluster-autoscaler"

tls_cert = <<EOF_TLS
${tls_cert_content}
EOF_TLS

ca_cert = <<EOF_CA
${ca_cert_content}
EOF_CA

tls_key = <<EOF_KEY
${tls_key_content}
EOF_KEY

# specific to IAC shippable version
enable_eks_auth                    = true
aws_roles                          = [${AWS_ADMIN_ROLES}]
release_service_refresh_token      =  "$AZUREAD_REFRESH_TOKEN"
eks_additional_iam_policies        = []
auto_cert                          = "${AUTO_CERT}"
eks_user_script_post_cloud_init        = <<CIEOF
$EKS_USER_SCRIPT_POST_CLOUD_INIT
CIEOF

eks_user_script_pre_cloud_init         = <<CIEOF
$EKS_USER_SCRIPT_PRE_CLOUD_INIT
CIEOF

eks_http_proxy                         = "$EKS_HTTP_PROXY"
eks_https_proxy                        = "$EKS_HTTPS_PROXY"
eks_no_proxy                           = "$EKS_NO_PROXY"
eks_cluster_dns_ip                 = "$EKS_CLUSTER_DNS_IP"
eks_additional_node_groups         ={
    "observability": {
        desired_size = $OBS_DES
        min_size = 0
        max_size = 1
        labels = {
            "node.kubernetes.io/custom-rule": "observability"
        }
        taints = {
        "node.kubernetes.io/custom-rule": {
            value = "observability"
            effect = "NO_SCHEDULE"
        }
        }
        instance_type = "$INSTANCE_TYPE_OBS"
        volume_size = "$VOL_SIZE_OBS"
        volume_type = "$VOL_TYPE_OBS"
    }
}

EOF

    if [[ -n "$CUSTOMER_TAG" ]]; then
        echo "customer_tag = \"${CUSTOMER_TAG}\""
    fi
}

# TEMPORARY: Extract S3_PREFIX cluster_state.json
# This function can be removed after 2025.2.0 as variable s3_prefix_used will be available
get_s3_prefix() {
    local bucket_name=$(cat cluster_state.json | jq -r '.resources[] | select(.type == "aws_s3_bucket" and .module == "module.s3") | .instances[0].attributes.bucket' | head -1)
    echo "$bucket_name" | sed "s/^${ENV_NAME}-//" | cut -d'-' -f1
}

action_cluster() {
    echo "Creating directory for environemnts"
    dir="${ROOT_DIR}/${ORCH_DIR}/cluster/environments/${ENV_NAME}"
    [[ ! -d ${dir} ]] && mkdir -p "${dir}"

    if $AUTO_CERT; then
        export TF_VAR_tls_key="$(cat ${SAVE_DIR}/${PRIVKEY})"
        export TF_VAR_tls_cert="$(cat ${SAVE_DIR}/${FULLCHAIN})"
        export TF_VAR_ca_cert="$(cat ${SAVE_DIR}/${CHAIN})"
    fi
    export TF_VAR_auto_cert=${AUTO_CERT}
    export TF_VAR_webhook_github_netrc=""
    echo "Creating backend & variable files"
    backend="$(cluster_backend)" && echo "$backend" > $dir/backend.tf
    variable="$(cluster_variable)" && echo "$variable" > $dir/variable.tfvar
    BUCKET=$(grep -E '^bucket' $dir/backend.tf | awk -F'=' '{print $2}' | tr -d ' "')
    CLUSTER_PATH="s3://${BUCKET}/${AWS_REGION}/cluster/${ENV_NAME}"
    LB_PATH="s3://${BUCKET}/${AWS_REGION}/orch-load-balancer/${ENV_NAME}"
    aws s3 cp $CLUSTER_PATH cluster_state.json
    aws s3 cp $LB_PATH lb_state.json
    
    echo "export FILE_SYSTEM_ID=$(cat cluster_state.json | jq -r '.outputs.efs_file_system_id.value')" > ~/pod-configs/.env
    echo "export ARGOCD_TG_ARN=$(cat lb_state.json | jq -r '.outputs.argocd_target_groups.value.argocd.arn // empty')" >> ~/pod-configs/.env
    echo "export TRAEFIK_TG_ARN=$(cat lb_state.json | jq -r '.outputs.traefik_target_groups.value.default.arn // empty')" >> ~/pod-configs/.env
    echo "export S3_PREFIX=$(get_s3_prefix)" >> ~/pod-configs/.env
    sed -i '/^#!\/bin\/bash$/a source ~/pod-configs/.env' /root/configure-cluster.sh
}

setup_cas() {

CLUSTER_NAME=$ENV_NAME

for NG in $(aws eks list-nodegroups \
              --cluster-name $CLUSTER_NAME \
              --query "nodegroups[]" \
              --output text)
do
  ARN=$(aws eks describe-nodegroup \
        --cluster-name $CLUSTER_NAME \
        --nodegroup-name $NG \
        --query "nodegroup.nodegroupArn" \
        --output text)

echo "Updating tags for ${NG}"

  aws eks tag-resource \
    --resource-arn "$ARN" \
    --tags "k8s.io/cluster-autoscaler/enabled=true,k8s.io/cluster-autoscaler/${CLUSTER_NAME}=owned"

done

echo "Updating nodegroup-${ENV_NAME}: min=1, max=5, desired=3"

aws eks update-nodegroup-config \
  --cluster-name $ENV_NAME \
  --nodegroup-name nodegroup-${ENV_NAME} \
  --region $AWS_REGION \
  --scaling-config minSize=1,maxSize=5,desiredSize=3

echo "Fetching current  desired size for observability..."

OBS_DES=$(aws eks describe-nodegroup \
  --cluster-name $ENV_NAME \
  --nodegroup-name observability \
  --region $AWS_REGION \
  --query 'nodegroup.scalingConfig.desiredSize' \
  --output text)

echo "Updating observability: min=0, max=1, desired=$OBS_DES"

aws eks update-nodegroup-config \
  --cluster-name $ENV_NAME \
  --nodegroup-name observability \
  --region $AWS_REGION \
  --scaling-config minSize=0,maxSize=1,desiredSize=$OBS_DES

POLICY_NAME="CASControllerPolicy-${CLUSTER_NAME}"

echo "Finding policy ARN..."

POLICY_ARN=$(aws iam list-policies \
  --scope Local \
  --query "Policies[?PolicyName=='${POLICY_NAME}'].Arn" \
  --output text)

if [ -z "$POLICY_ARN" ]; then
  echo "Policy not found!"
  return
fi

echo "Policy ARN: $POLICY_ARN"

echo "Fetching default policy version..."

VERSION_ID=$(aws iam get-policy \
  --policy-arn "$POLICY_ARN" \
  --query "Policy.DefaultVersionId" \
  --output text)

echo "Downloading current policy..."

aws iam get-policy-version \
  --policy-arn "$POLICY_ARN" \
  --version-id "$VERSION_ID" \
  --query "PolicyVersion.Document" \
  --output json > policy.json

echo "Checking if eks:DescribeCluster already exists..."

if grep -q "eks:DescribeCluster" policy.json; then
  echo "Permission already exists. Exiting."
  return
fi

echo "Updating policy..."

jq '.Statement += [{
  "Effect": "Allow",
  "Action": "eks:DescribeCluster",
  "Resource": "*"
}]' policy.json > updated-policy.json

echo "Checking policy version limit..."

VERSIONS=$(aws iam list-policy-versions \
  --policy-arn "$POLICY_ARN" \
  --query "Versions[?IsDefaultVersion==\`false\`].VersionId" \
  --output text)

COUNT=$(echo "$VERSIONS" | wc -w)

if [ "$COUNT" -ge 4 ]; then
  OLDEST=$(aws iam list-policy-versions \
    --policy-arn "$POLICY_ARN" \
    --query "Versions[?IsDefaultVersion==\`false\`]|sort_by(@,&CreateDate)[0].VersionId" \
    --output text)

  echo "Deleting old version: $OLDEST"

  aws iam delete-policy-version \
    --policy-arn "$POLICY_ARN" \
    --version-id "$OLDEST"
fi

echo "Creating new policy version..."

aws iam create-policy-version \
  --policy-arn "$POLICY_ARN" \
  --policy-document file://updated-policy.json \
  --set-as-default

echo "✅ Policy successfully updated!"
}


apply_cas() {

dir="${ROOT_DIR}/${ORCH_DIR}/cluster"
echo "Changing directory to $dir..."
cd "$dir"

echo "Initializing Terraform for environment: $ENV_NAME..."
if terraform init -reconfigure -backend-config="environments/${ENV_NAME}/backend.tf"; then
    echo "✅ Terraform initialization succeeded."
else
    echo "❌ Terraform initialization failed!"
    exit 1
fi

echo "Applying changes for EKS CAS module..."
if terraform apply -target=module.eks-cas -var-file="environments/${ENV_NAME}/variable.tfvar" -auto-approve; then
    echo "✅ Terraform apply for EKS CAS module succeeded."
else
    echo "❌ Terraform apply for EKS CAS module failed!"
    exit 1
fi

}

orch_route53_backend() {
    cat <<EOF
bucket = "$BUCKET_NAME"
key    = "${AWS_REGION}/orch-route53/${ENV_NAME}"
region = "$BUCKET_REGION"
EOF
}

orch_route53_variable() {
    local create_root_domain=${1:-false}
    # Use PARENT_ZONE if available, otherwise fall back to PARENT_DOMAIN
    local parent_zone="${PARENT_ZONE:-${PARENT_DOMAIN:-}}"
    local host_name="${HOST_NAME:-}"
    
    cat <<EOF
parent_zone                     = "${parent_zone}"
create_root_domain              = ${create_root_domain}
orch_name                       = "${ENV_NAME}"
host_name                       = "${host_name}"
vpc_id                          = "${VPC_ID}"
vpc_region                      = "${AWS_REGION}"
lb_created                      = true
enable_pull_through_cache_proxy = ${ENABLE_CACHE_REGISTRY:-false}
EOF

    if [[ -n "${CUSTOMER_TAG:-}" ]]; then
        echo "customer_tag = \"${CUSTOMER_TAG}\""
    fi
}

orch_loadbalancer_backend() {
    cat <<EOF
bucket = "${BUCKET_NAME}"
key    = "${AWS_REGION}/orch-load-balancer/${ENV_NAME}"
region = "${BUCKET_REGION}"
EOF
}

orch_loadbalancer_variable() {
    # Use the same logic as cluster_variable to determine VPC backend key
    local VPC_TERRAFORM_BACKEND_KEY="${AWS_REGION}/vpc/${ENV_NAME}"
    
    # We will not create VPC if the --skip-apply-vpc is set
    # and the VPC_ID is not empty.
    if [[ -n "$VPC_ID" ]] && ${SKIP_APPLY_VPC:-false}; then
        VPC_TERRAFORM_BACKEND_KEY="${AWS_REGION}/vpc/${VPC_ID}"
    fi
    
    cat <<EOF
vpc_terraform_backend_bucket = "${BUCKET_NAME}"
vpc_terraform_backend_key    = "${VPC_TERRAFORM_BACKEND_KEY}"
vpc_terraform_backend_region = "${BUCKET_REGION}"

cluster_terraform_backend_bucket = "${BUCKET_NAME}"
cluster_terraform_backend_key    = "${AWS_REGION}/cluster/${ENV_NAME}"
cluster_terraform_backend_region = "${BUCKET_REGION}"

cluster_name = "${ENV_NAME}"
ip_allow_list = ["0.0.0.0/0"]
create_target_group_attachment = true
root_domain = "${ROOT_DOMAIN:-}"

internal = false
EOF

    if [[ -n "${CUSTOMER_TAG:-}" ]]; then
        echo "customer_tag = \"${CUSTOMER_TAG}\""
    fi
}

orch_loadbalancer_variable_internal_default() {
    # Use the same logic as cluster_variable to determine VPC backend key
    local VPC_TERRAFORM_BACKEND_KEY="${AWS_REGION}/vpc/${ENV_NAME}"
    
    # We will not create VPC if the --skip-apply-vpc is set
    # and the VPC_ID is not empty.
    if [[ -n "$VPC_ID" ]] && ${SKIP_APPLY_VPC:-false}; then
        VPC_TERRAFORM_BACKEND_KEY="${AWS_REGION}/vpc/${VPC_ID}"
    fi
    
    cat <<EOF
vpc_terraform_backend_bucket = "${BUCKET_NAME}"
vpc_terraform_backend_key    = "${VPC_TERRAFORM_BACKEND_KEY}"
vpc_terraform_backend_region = "${BUCKET_REGION}"

cluster_terraform_backend_bucket = "${BUCKET_NAME}"
cluster_terraform_backend_key    = "${AWS_REGION}/cluster/${ENV_NAME}"
cluster_terraform_backend_region = "${BUCKET_REGION}"

cluster_name = "${ENV_NAME}"
ip_allow_list = ["0.0.0.0/0"]
create_target_group_attachment = true
root_domain = "${ROOT_DOMAIN:-}"

internal = true
EOF

    if [[ -n "$CUSTOMER_TAG" ]]; then
        echo "customer_tag = \"${CUSTOMER_TAG}\""
    fi
}

action_orch_loadbalancer() {

    dir="${ROOT_DIR}/${ORCH_DIR}/orch-load-balancer/environments/${ENV_NAME}"
    [[ ! -d ${dir} ]] && mkdir -p "${dir}"

    # vvvvvvvv Keep this part for backward compatibility vvvvvvvv
    # Will deprecate in 25.02 release
    # The application load balancer module has its own certificate variables.
    if $AUTO_CERT; then
        export TF_VAR_tls_key="$(cat ${SAVE_DIR}/${PRIVKEY})"
        export TF_VAR_tls_cert_chain="$(cat ${SAVE_DIR}/${CHAIN})"
        export TF_VAR_tls_cert_body="$(get_end_cert "$(cat ${SAVE_DIR}/${FULLCHAIN})")"
        export TF_VAR_auto_cert=${AUTO_CERT}
    else
        load_values
        export TF_VAR_tls_cert_chain="$TF_VAR_tls_cert"
        export TF_VAR_tls_cert_body=$(get_end_cert "$TF_VAR_tls_cert")
    fi

    local variable_override=$(mktemp)

    # The application load balancer module has its own certificate variables.
    if $AUTO_CERT; then
        echo "tls_key = <<-EOF" > $variable_override
        cat "${SAVE_DIR}/${PRIVKEY}" >> $variable_override
        echo "EOF" >> $variable_override

        echo "tls_cert_chain = <<-EOF" >> $variable_override
        cat "${SAVE_DIR}/${CHAIN}" >> $variable_override
        echo "EOF" >> $variable_override

        tls_cert_body="$(get_end_cert "$(cat ${SAVE_DIR}/${FULLCHAIN})")"
        echo "tls_cert_body = <<-EOF" >> $variable_override
        echo "$tls_cert_body" >> $variable_override
        echo "EOF" >> $variable_override

        echo "auto_cert = ${AUTO_CERT}"
    else
        # TODO: remove "load_values" once we remove values.sh in 25.02 release
        # We should use a single tfvar file to store all the variables, including certificate variables
        load_values

        echo "tls_key = <<-EOF" > $variable_override
        cat "${TF_VAR_tls_key}" >> $variable_override
        echo "EOF" >> $variable_override

        echo "tls_cert_chain = <<-EOF" >> $variable_override
        cat "${TF_VAR_tls_cert}" >> $variable_override
        echo "EOF" >> $variable_override

        tls_cert_body="$(get_end_cert "${TF_VAR_tls_cert}")"
        echo "tls_cert_body = <<-EOF" >> $variable_override
        echo "$tls_cert_body" >> $variable_override
        echo "EOF" >> $variable_override
    fi

    module=""
    backend="$(orch_loadbalancer_backend)" && echo "$backend" > $dir/backend.tf
    module="${ROOT_DIR}/${ORCH_DIR}/orch-load-balancer"
    if ! $INTERNAL; then
        variable="$(orch_loadbalancer_variable)" && echo "$variable" > $dir/variable.tfvar
    else
        variable="$(orch_loadbalancer_variable_internal_default)" && echo "$variable" > $dir/variable.tfvar
    fi

    cd $module
    echo "Initializing Terraform for environment: $ENV_NAME..."
    if terraform init -reconfigure -backend-config="environments/${ENV_NAME}/backend.tf"; then
      echo "✅ Terraform initialization succeeded."
    else
      echo "❌ Terraform initialization failed!"
      exit 1
    fi

    # Check for existing target group bindings to avoid conflicts
    echo "Checking for existing target group bindings..."
    kubectl get targetgroupbinding -A 2>/dev/null || true
    
    # Enhanced cleanup for ingress-nginx services across all namespaces
    echo "Enhanced cleanup for ingress-nginx services across all namespaces..."
    
    # Find all ingress-nginx related services in any namespace
    ingress_nginx_services=$(kubectl get services -A -o name 2>/dev/null | grep -E "ingress-nginx" || echo "")
    
    if [[ -n "$ingress_nginx_services" ]]; then
        echo "Found ingress-nginx services to clean up:"
        echo "$ingress_nginx_services"
        
        echo "$ingress_nginx_services" | while IFS= read -r service_ref; do
            if [[ -n "$service_ref" ]]; then
                # Extract namespace and service name
                namespace=$(kubectl get "$service_ref" -o jsonpath='{.metadata.namespace}' 2>/dev/null || echo "")
                service_name=$(echo "$service_ref" | sed 's|.*/||')
                
                if [[ -n "$namespace" && -n "$service_name" ]]; then
                    echo "Cleaning up service $service_name in namespace $namespace..."
                    
                    # Remove finalizers to ensure clean deletion
                    kubectl patch service "$service_name" -n "$namespace" --type=json \
                        -p='[{"op":"remove","path":"/metadata/finalizers"}]' 2>/dev/null || true
                    
                    # Remove AWS load balancer controller annotations
                    kubectl annotate service "$service_name" -n "$namespace" \
                        service.beta.kubernetes.io/aws-load-balancer-type- \
                        service.beta.kubernetes.io/aws-load-balancer-backend-protocol- \
                        service.beta.kubernetes.io/aws-load-balancer-ssl-ports- \
                        service.beta.kubernetes.io/aws-load-balancer-ssl-cert- \
                        --ignore-not-found=true 2>/dev/null || true
                    
                    # Force delete the service
                    kubectl delete service "$service_name" -n "$namespace" --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
                    
                    echo "✅ Cleaned up $service_name in namespace $namespace"
                fi
            fi
        done
        
        # Wait for cleanup to propagate
        echo "⏳ Waiting for enhanced ingress-nginx service cleanup to complete..."
        sleep 15
    else
        echo "ℹ️ No ingress-nginx services found to clean up"
    fi
    
    # Clean up Tinkerbell nginx ingress that blocks haproxy transition
    echo "Checking for Tinkerbell nginx ingress in orch-infra namespace..."
    if kubectl get ingress tinkerbell-nginx-ingress -n orch-infra >/dev/null 2>&1; then
        echo "🧹 Found tinkerbell-nginx-ingress - removing to allow haproxy transition..."
        
        # Remove finalizers and delete the nginx ingress
        kubectl patch ingress tinkerbell-nginx-ingress -n orch-infra --type=json \
            -p='[{"op":"remove","path":"/metadata/finalizers"}]' 2>/dev/null || true
        kubectl delete ingress tinkerbell-nginx-ingress -n orch-infra --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
        
        # Also clean up related nginx service in orch-infra if it exists
        if kubectl get service -n orch-infra | grep -q nginx; then
            echo "Cleaning up nginx services in orch-infra namespace..."
            kubectl delete service -l "app.kubernetes.io/name=nginx" -n orch-infra --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
            kubectl delete service -l "app=nginx" -n orch-infra --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
        fi
        
        echo "✅ Cleaned up tinkerbell-nginx-ingress to prepare for haproxy transition"
    else
        echo "ℹ️ tinkerbell-nginx-ingress not found in orch-infra namespace"
    fi
    
    # Plan first to see what changes will be made
    echo "Planning load balancer target group changes..."
    if ! terraform plan -target=module.traefik_lb_target_group_binding -var-file="environments/${ENV_NAME}/variable.tfvar" -out=lb_plan; then
        echo "❌ Terraform plan failed!"
        exit 1
    fi
    
    # Clean up any existing ingress-nginx-controller target group bindings that might conflict
    echo "Cleaning up any existing ingress-nginx-controller target group bindings..."
    
    # Delete only ingress-nginx-controller bindings across all namespaces
    echo "Searching for ingress-nginx-controller bindings..."
    bindings_output=$(kubectl get targetgroupbinding -A -o json 2>/dev/null || echo '{"items":[]}')
    
    if [[ "$bindings_output" != '{"items":[]}' ]]; then
        echo "Processing existing target group bindings..."
        echo "$bindings_output" | jq -r '.items[]? | select(.metadata.name == "ingress-nginx-controller") | "\(.metadata.namespace) \(.metadata.name)"' 2>/dev/null | while read -r namespace name; do
            if [[ -n "$namespace" && -n "$name" ]]; then
                echo "Removing existing ingress-nginx-controller binding: $name in namespace $namespace"
                kubectl delete targetgroupbinding "$name" -n "$namespace" 2>/dev/null || true
            fi
        done || true
    else
        echo "No target group bindings found to process."
    fi
    
    # Wait for ingress-nginx-controller cleanup to complete
    echo "Waiting for ingress-nginx-controller target group binding cleanup to complete..."
    sleep 10
    
    # Verify ingress-nginx-controller cleanup
    echo "Verifying ingress-nginx-controller cleanup completed..."
    remaining_nginx_bindings=$(kubectl get targetgroupbinding -A 2>/dev/null | grep -c "ingress-nginx-controller" 2>/dev/null || echo "0")
    if [[ "$remaining_nginx_bindings" -gt 0 ]]; then
        echo "⚠️  Warning: $remaining_nginx_bindings ingress-nginx-controller bindings still exist. Forcing cleanup..."
        # Force delete only ingress-nginx-controller bindings
        kubectl get targetgroupbinding -A -o name 2>/dev/null | grep "ingress-nginx-controller" | xargs -r kubectl delete 2>/dev/null || true
        sleep 5
    fi

    echo "Applying changes for traefik lb target group binding..."
    if terraform apply -target=module.traefik_lb_target_group_binding -var-file="environments/${ENV_NAME}/variable.tfvar" -auto-approve; then
      echo "✅ Terraform apply for traefik lb target group binding module succeeded."
    else
      echo "❌ Terraform apply for traefik lb target group binding module failed!"
      # Final ingress-nginx-controller cleanup attempt if still failing
      echo "🔧 Attempting final ingress-nginx-controller cleanup and retry..."
      kubectl get targetgroupbinding -A -o name 2>/dev/null | grep "ingress-nginx-controller" | xargs -r kubectl delete 2>/dev/null || true
      sleep 10
      echo "Retrying Terraform apply after ingress-nginx-controller cleanup..."
      if terraform apply -target=module.traefik_lb_target_group_binding -var-file="environments/${ENV_NAME}/variable.tfvar" -auto-approve; then
        echo "✅ Terraform apply succeeded after ingress-nginx-controller cleanup."
      else
        echo "❌ Terraform apply failed even after ingress-nginx-controller cleanup!"
        echo "Listing remaining ingress-nginx-controller target group bindings:"
        kubectl get targetgroupbinding -A 2>/dev/null | grep "ingress-nginx-controller" || echo "No ingress-nginx-controller bindings found"
        exit 1
      fi
    fi

    echo "Applying changes for aws lb security group roles..."
    if terraform apply -target=module.aws_lb_security_group_roles -var-file="environments/${ENV_NAME}/variable.tfvar" -auto-approve; then
      echo "✅ Terraform apply for aws lb security group roles module succeeded."
    else
      echo "❌ Terraform apply for aws lb security group roles module failed!"
      exit 1
    fi
    rm -rf $dir
}

# Function to detect if this is an internal deployment
is_internal_deployment() {
    # Check for internal deployment indicators
    # 1. Check if INTERNAL variable is set to true
    if [[ "${INTERNAL:-false}" == "true" ]]; then
        echo "Internal deployment detected via INTERNAL variable"
        return 0
    fi
    
    # 2. Check if the load balancer configuration uses internal=true
    local lb_config="${ROOT_DIR}/${ORCH_DIR}/orch-load-balancer/environments/${ENV_NAME}/variable.tfvar"
    if [[ -f "$lb_config" ]] && grep -q "internal = true" "$lb_config"; then
        echo "Internal deployment detected via load balancer configuration"
        return 0
    fi
    
    # 3. Check if any of the internal deployment environment variables are set
    if [[ -n "${AUTOINSTALL_INTERNAL:-}" ]] || [[ -n "${VPC_ID:-}" ]] || [[ "${USE_ARGO_PROXY:-false}" == "true" ]]; then
        echo "Internal deployment detected via environment variables"
        return 0
    fi
    
    # 4. Check for Intel proxy configurations (common in internal deployments)
    if [[ "${HTTP_PROXY:-}" == *"intel.com"* ]] || [[ "${HTTPS_PROXY:-}" == *"intel.com"* ]]; then
        echo "Internal deployment detected via Intel proxy configuration"
        return 0
    fi
    
    # 5. Check if ArgoCD has proxy configuration applied (typical for internal deployments)
    if kubectl get configmap argocd-cm -n argocd -o yaml 2>/dev/null | grep -q "proxy"; then
        echo "Internal deployment detected via ArgoCD proxy configuration"
        return 0
    fi
    
    echo "External/non-internal deployment detected"
    return 1
}

action_orch_route53_wi_lb() {
    dir="${ROOT_DIR}/${ORCH_DIR}/orch-route53/environments/${ENV_NAME}"
    [[ ! -d ${dir} ]] && mkdir -p "${dir}"

    backend="$(orch_route53_backend)" && echo "$backend" > $dir/backend.tf
    variable="$(orch_route53_variable true)" && echo "$variable" > $dir/variable.tfvar

    if [[ ! -f "$SAVE_DIR/$VARIABLE_TFVAR" ]]; then
        # In case the variable file is not created, create an empty one
        touch "$SAVE_DIR/$VARIABLE_TFVAR"
    fi

    cd "${ROOT_DIR}/${ORCH_DIR}/orch-route53"

    echo "Initializing Terraform for environment: $ENV_NAME..."
    if terraform init -reconfigure -backend-config="environments/${ENV_NAME}/backend.tf"; then
      echo "✅ Terraform initialization succeeded."
    else
      echo "❌ Terraform initialization failed!"
      exit 1
    fi

    # Check if hosted zones exist and import them if needed
    echo "Checking for existing Route53 hosted zones..."

    # Get the zone name that should exist
    local orch_zone="${ENV_NAME}.${PARENT_DOMAIN}"
    #if [[ -n "${HOST_NAME}" ]]; then
    #    orch_zone="${HOST_NAME}.${PARENT_DOMAIN}"
    #fi

    # Check if public zone exists and import if needed
    public_zone_id=$(aws route53 list-hosted-zones --query "HostedZones[?Name=='${orch_zone}.'].Id" --output text 2>/dev/null | sed 's|/hostedzone/||')
    if [[ -n "$public_zone_id" && "$public_zone_id" != "None" ]]; then
        echo "Found existing public hosted zone: $public_zone_id for $orch_zone"
        # Try to import if not already in state
        terraform import 'data.aws_route53_zone.orch_public[0]' "$public_zone_id" 2>/dev/null || true
    fi

    # Check if private zone exists and import if needed
    private_zone_id=$(aws route53 list-hosted-zones --query "HostedZones[?Name=='${orch_zone}.' && Config.PrivateZone].Id" --output text 2>/dev/null | sed 's|/hostedzone/||')
    if [[ -n "$private_zone_id" && "$private_zone_id" != "None" ]]; then
        echo "Found existing private hosted zone: $private_zone_id for $orch_zone"
        # Try to import if not already in state
        terraform import 'data.aws_route53_zone.orch_private[0]' "$private_zone_id" 2>/dev/null || true
    fi

    echo "Planning Route53 changes..."
    if ! terraform plan -var-file="environments/${ENV_NAME}/variable.tfvar" -out=tfplan; then
        echo "❌ Terraform plan failed!"
        exit 1
    fi

    # Check if plan tries to destroy any hosted zones
    if terraform show -json tfplan | jq -r '.resource_changes[]? | select(.change.actions[]? == "delete") | select(.type == "aws_route53_zone") | .address' | grep -q .; then
        echo "❌ ERROR: Terraform plan wants to delete hosted zones! This would cause HostedZoneNotEmpty error."
        echo "Hosted zones to be deleted:"
        terraform show -json tfplan | jq -r '.resource_changes[]? | select(.change.actions[]? == "delete") | select(.type == "aws_route53_zone") | .address'
        echo "Aborting upgrade to prevent zone deletion..."
        exit 1
    fi

    echo "Applying Route53 changes for load balancer target group binding..."
    if terraform apply -var-file="environments/${ENV_NAME}/variable.tfvar" -auto-approve; then
      echo "✅ Terraform apply for Route53 lb target group binding module succeeded."
    else
      echo "❌ Terraform apply for Route53 lb target group binding module failed!"
      exit 1
    fi
    rm -rf $dir

}    

# Main

if [[ ${COMMAND:-""} != upgrade ]]; then
    # Not called by the provision script, need to parse command line parameters.
    # shellcheck disable=SC2068
    ##parse_params $@
    exit 1
fi

# Terminate existing sshuttle
refresh_sshuttle
connect_cluster

echo "Starting action cluster"
setup_cas
action_cluster
apply_cas
action_orch_loadbalancer
action_orch_route53_wi_lb

# Terminate existing sshuttle
terminate_sshuttle

echo "Info: Upgrade completed successfully."
