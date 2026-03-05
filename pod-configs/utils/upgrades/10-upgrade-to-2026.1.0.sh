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

# Function to ensure Terraform target group bindings can be destroyed
cleanup_terraform_target_group_bindings() {
    echo "🔧 Cleaning up Terraform target group bindings for ingress controllers..."
    
    # Find any remaining services in orch-boots that might have target group bindings
    local remaining_services=$(kubectl get services -n orch-boots -o name 2>/dev/null | grep -E "(ingress-nginx|nginx-ingress)" || echo "")
    
    if [[ -n "$remaining_services" ]]; then
        echo "Found remaining ingress services, removing them:"
        echo "$remaining_services"
        
        # Remove finalizers from services to ensure they can be deleted
        echo "$remaining_services" | while IFS= read -r service; do
            if [[ -n "$service" ]]; then
                service_name=$(echo "$service" | sed 's|service/||')
                echo "Removing finalizers from $service_name..."
                kubectl patch service "$service_name" -n orch-boots --type=json \
                    -p='[{"op":"remove","path":"/metadata/finalizers"}]' 2>/dev/null || true
                    
                # Force delete the service
                kubectl delete service "$service_name" -n orch-boots --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
            fi
        done
    fi
    
    # Wait a moment for Kubernetes to process the deletions
    echo "⏳ Waiting for service deletions to complete..."
    sleep 10
    
    # Double-check that ingress-nginx-controller service is completely gone
    if kubectl get service ingress-nginx-controller -n orch-boots >/dev/null 2>&1; then
        echo "🚨 ingress-nginx-controller service still exists, forcing removal..."
        kubectl patch service ingress-nginx-controller -n orch-boots --type=json \
            -p='[{"op":"remove","path":"/metadata/finalizers"}]' 2>/dev/null || true
        kubectl delete service ingress-nginx-controller -n orch-boots --force --grace-period=0 --ignore-not-found=true 2>/dev/null
        
        # Wait and verify
        for i in {1..15}; do
            if ! kubectl get service ingress-nginx-controller -n orch-boots >/dev/null 2>&1; then
                echo "✅ ingress-nginx-controller service finally removed"
                break
            fi
            echo "Still waiting for ingress-nginx-controller service removal... (attempt $i/15)"
            sleep 3
        done
    else
        echo "✅ ingress-nginx-controller service already removed"
    fi
    
    echo "✅ Target group binding cleanup completed"
}

# Function to remove ingress applications from ArgoCD root-app configuration
remove_ingress_from_argocd_config() {
    echo "📝 Removing ingress applications from ArgoCD root-app configuration..."
    
    # Find and modify the root-app to exclude ingress applications
    if kubectl get application root-app -n argocd >/dev/null 2>&1; then
        echo "Modifying root-app to exclude ingress applications..."
        
        # Create a temporary patch to exclude ingress applications
        local patch='{
            "spec": {
                "source": {
                    "helm": {
                        "parameters": [
                            {"name": "ingress-nginx.enabled", "value": "false"},
                            {"name": "nginx-ingress-pxe-boots.enabled", "value": "false"}
                        ]
                    }
                }
            }
        }'
        
        kubectl patch application root-app -n argocd --type=merge -p="$patch" 2>/dev/null || true
    fi
    
    # Also check for any helm values that might re-enable these applications
    if kubectl get configmap argocd-values -n argocd >/dev/null 2>&1; then
        echo "Updating ArgoCD values to disable ingress applications..."
        kubectl patch configmap argocd-values -n argocd --type=merge \
            -p='{"data":{"values.yaml":"ingress-nginx:\n  enabled: false\nnginx-ingress-pxe-boots:\n  enabled: false\n"}}' 2>/dev/null || true
    fi
    
    echo "✅ Removed ingress applications from ArgoCD configuration"
}

# Function to aggressively disable ArgoCD sync for ingress applications
disable_argocd_ingress_sync() {
    echo "🛑 Aggressively disabling ArgoCD sync for ingress applications..."
    
    local ingress_apps=("ingress-nginx" "nginx-ingress-pxe-boots")
    
    # First, suspend all ingress applications immediately
    for app in "${ingress_apps[@]}"; do
        if kubectl get application "$app" -n argocd >/dev/null 2>&1; then
            echo "🔒 Suspending auto-sync and setting manual sync for $app..."
            kubectl patch application "$app" -n argocd --type=merge \
                -p='{"spec":{"syncPolicy":{"automated":null,"syncOptions":["CreateNamespace=false"]}}}' 2>/dev/null || true
                
            # Mark the application as out-of-sync to prevent automatic sync
            kubectl annotate application "$app" -n argocd \
                argocd.argoproj.io/sync="manual" \
                argocd.argoproj.io/compare-options="IgnoreExtraneous" --overwrite 2>/dev/null || true
        fi
    done
    
    # Temporarily disable ArgoCD application controller auto-sync globally for ingress apps
    echo "🔧 Temporarily modifying ArgoCD controller settings..."
    kubectl patch configmap argocd-cmd-params-cm -n argocd --type=merge \
        -p='{"data":{"application.instanceLabelKey":"disabled","controller.self.heal.timeout.seconds":"0"}}' 2>/dev/null || true
        
    # Force ArgoCD to reload configuration
    kubectl rollout restart deployment argocd-application-controller -n argocd 2>/dev/null || true
    
    echo "✅ ArgoCD sync disabled for ingress applications"
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

# Function to clean up ingress applications for internal deployments
cleanup_ingress_applications_internal() {
    echo "🔧 Detected internal deployment - cleaning up ingress applications that ArgoCD may not properly manage..."
    
    # List of ingress applications that need cleanup in internal deployments
    local ingress_apps=("ingress-nginx" "nginx-ingress-pxe-boots")
    
    for app in "${ingress_apps[@]}"; do
        echo "Checking for ArgoCD application: $app"
        
        # Check if the application exists in ArgoCD
        if kubectl get application "$app" -n argocd >/dev/null 2>&1; then
            echo "Found ArgoCD application: $app - proceeding with cleanup"
            
            # Get application status before deletion
            local app_status=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.health.status}' 2>/dev/null || echo "Unknown")
            echo "Application $app status: $app_status"
            
            # Remove finalizers to allow deletion
            echo "Removing finalizers from ArgoCD application: $app"
            kubectl patch application "$app" -n argocd --type=json \
                -p='[{"op":"remove","path":"/metadata/finalizers"}]' 2>/dev/null || true
            
            # Delete the ArgoCD application
            echo "Deleting ArgoCD application: $app"
            kubectl delete application "$app" -n argocd --force --grace-period=0 \
                --ignore-not-found=true 2>/dev/null || true
            
            # Wait a moment for ArgoCD to process the deletion
            sleep 5
        else
            echo "ArgoCD application $app not found - may already be cleaned up"
        fi
        
        # Clean up any remaining Kubernetes resources directly
        echo "Cleaning up Kubernetes resources for: $app"
        
        # Delete the Helm release if it exists
        if helm list -n orch-boots 2>/dev/null | grep -q "$app"; then
            echo "Deleting Helm release: $app"
            helm uninstall "$app" -n orch-boots --timeout=120s 2>/dev/null || true
            sleep 3
        fi
        
        # Clean up any remaining resources in the orch-boots namespace
        echo "Cleaning up labeled resources for: $app"
        kubectl delete deployment,service,configmap,secret,ingress,daemonset,replicaset \
            -l "app.kubernetes.io/name=$app" -n orch-boots \
            --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
            
        kubectl delete deployment,service,configmap,secret,ingress,daemonset,replicaset \
            -l "app=$app" -n orch-boots \
            --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
            
        # Clean up specific ingress controller resources
        if [[ "$app" == "ingress-nginx" ]]; then
            echo "Cleaning up ingress-nginx specific resources including controller service"
            
            # Force delete the ingress-nginx-controller service that Terraform is tracking
            kubectl delete service ingress-nginx-controller -n orch-boots --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
            kubectl delete service ingress-nginx-controller-admission -n orch-boots --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
            kubectl delete service ingress-nginx-controller-metrics -n orch-boots --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
            
            # Remove specific ingress-nginx resources
            kubectl delete deployment ingress-nginx-controller -n orch-boots --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
            kubectl delete validatingwebhookconfiguration ingress-nginx-admission --ignore-not-found=true 2>/dev/null || true
            kubectl delete ingressclass nginx --ignore-not-found=true 2>/dev/null || true
            
            # Clean up any remaining ingress-nginx resources by name pattern
            kubectl delete all -l "app.kubernetes.io/name=ingress-nginx" -n orch-boots --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
            kubectl delete all -l "app.kubernetes.io/component=controller" -l "app.kubernetes.io/name=ingress-nginx" -n orch-boots --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
            
            # Verify the controller service is completely removed
            echo "Verifying ingress-nginx-controller service removal..."
            for i in {1..10}; do
                if ! kubectl get service ingress-nginx-controller -n orch-boots >/dev/null 2>&1; then
                    echo "✅ ingress-nginx-controller service successfully removed"
                    break
                fi
                echo "Waiting for ingress-nginx-controller service to be removed... (attempt $i/10)"
                sleep 2
            done
        fi
    done
    
    # Clean up any conflicting NodePort services
    echo "Checking for NodePort conflicts on ports that ingress applications typically use..."
    for port in 31443 30080 30443; do
        conflicting_services=$(kubectl get services -A -o json 2>/dev/null | \
            jq -r ".items[] | select(.spec.ports[]?.nodePort == $port) | \"\(.metadata.namespace) \(.metadata.name)\"" 2>/dev/null || echo "")
        
        if [[ -n "$conflicting_services" ]]; then
            echo "Found services using NodePort $port:"
            echo "$conflicting_services"
            
            echo "$conflicting_services" | while IFS= read -r service_info; do
                if [[ -n "$service_info" ]]; then
                    namespace=$(echo "$service_info" | awk '{print $1}')
                    service=$(echo "$service_info" | awk '{print $2}')
                    if [[ -n "$namespace" && -n "$service" ]]; then
                        echo "Removing conflicting service: $service in namespace $namespace"
                        kubectl delete service "$service" -n "$namespace" --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
                    fi
                fi
            done
        fi
    done
    
    # Verify cleanup completion
    echo "Verifying ingress application cleanup..."
    for app in "${ingress_apps[@]}"; do
        if kubectl get application "$app" -n argocd >/dev/null 2>&1; then
            echo "⚠️  Warning: ArgoCD application $app still exists after cleanup attempt"
        else
            echo "✅ ArgoCD application $app successfully removed"
        fi
    done
    
    # Wait for cleanup to complete
    echo "Waiting for ingress application cleanup to complete..."
    sleep 15
    
    echo "✅ Internal deployment ingress application cleanup completed"
}

# Function to disable ingress applications in ArgoCD configuration for internal deployments
disable_ingress_applications_in_argocd() {
    echo "🔧 Disabling ingress applications in ArgoCD configuration for internal deployment..."
    
    # List of ingress applications to disable
    local ingress_apps=("ingress-nginx" "nginx-ingress-pxe-boots")
    
    # Check if root-app exists and get its current configuration
    if ! kubectl get application root-app -n argocd >/dev/null 2>&1; then
        echo "⚠️  Warning: root-app not found in ArgoCD, cannot disable ingress applications"
        return 0
    fi
    
    # Get the current root-app configuration
    echo "Retrieving current root-app configuration..."
    kubectl get application root-app -n argocd -o yaml > /tmp/root-app-backup.yaml
    
    # Create a patch to disable ingress applications
    cat > /tmp/ingress-apps-disable-patch.yaml << 'EOF'
spec:
  source:
    helm:
      values: |
        argo:
          enabled:
            ingress-nginx: false
            nginx-ingress-pxe-boots: false
            # Keep ingress-haproxy enabled as it's the replacement
            ingress-haproxy: true
EOF
    
    # Apply the patch to disable ingress applications
    echo "Applying patch to disable ingress applications in root-app..."
    if kubectl patch application root-app -n argocd --patch-file=/tmp/ingress-apps-disable-patch.yaml --type=merge; then
        echo "✅ Successfully disabled ingress applications in ArgoCD configuration"
    else
        echo "⚠️  Warning: Failed to patch root-app configuration, trying alternative method..."
        
        # Alternative method: Update the ArgoCD values configmap if it exists
        if kubectl get configmap argocd-values -n argocd >/dev/null 2>&1; then
            echo "Updating argocd-values configmap to disable ingress applications..."
            kubectl get configmap argocd-values -n argocd -o yaml > /tmp/argocd-values-backup.yaml
            
            # Create updated values with disabled applications
            kubectl get configmap argocd-values -n argocd -o jsonpath='{.data.values\.yaml}' > /tmp/current-values.yaml
            
            # Add or update the enabled section to disable ingress applications
            cat >> /tmp/current-values.yaml << 'EOF'

# Disable problematic ingress applications for internal deployment
argo:
  enabled:
    ingress-nginx: false
    nginx-ingress-pxe-boots: false
    ingress-haproxy: true
EOF
            
            # Update the configmap
            kubectl create configmap argocd-values --from-file=values.yaml=/tmp/current-values.yaml -n argocd --dry-run=client -o yaml | \
                kubectl apply -f -
            
            echo "✅ Updated argocd-values configmap to disable ingress applications"
        fi
    fi
    
    # Force ArgoCD to refresh the root-app
    echo "Refreshing root-app in ArgoCD..."
    kubectl annotate application root-app -n argocd argocd.argoproj.io/refresh=normal --overwrite 2>/dev/null || true
    
    # Wait for ArgoCD to process the configuration change
    echo "Waiting for ArgoCD to process configuration changes..."
    sleep 10
    
    # Force ArgoCD to stop any ongoing syncs for ingress applications
    echo "🛑 Stopping any ongoing ArgoCD syncs for ingress applications..."
    kubectl patch argocd argocd -n argocd --type=merge \
        -p='{"spec":{"controller":{"env":[{"name":"ARGOCD_APPLICATION_NAMESPACES","value":"argocd"}]}}}' 2>/dev/null || true
    
    # Restart ArgoCD application controller to apply changes immediately
    kubectl rollout restart deployment argocd-application-controller -n argocd 2>/dev/null || true
    kubectl rollout restart deployment argocd-server -n argocd 2>/dev/null || true
    
    echo "⏳ Waiting for ArgoCD components to restart..."
    kubectl rollout status deployment argocd-application-controller -n argocd --timeout=120s 2>/dev/null || true
    kubectl rollout status deployment argocd-server -n argocd --timeout=120s 2>/dev/null || true
    
    echo "✅ Disabled ingress applications in ArgoCD configuration and restarted ArgoCD"
}

# Function to ensure ingress applications stay disabled throughout the upgrade
monitor_and_prevent_ingress_redeploy() {
    echo "🔧 Setting up monitoring to prevent ingress application redeployment..."
    
    local ingress_apps=("ingress-nginx" "nginx-ingress-pxe-boots")
    
    # Create a background job to monitor and immediately delete any ingress applications that get recreated
    {
        echo "Starting aggressive ingress application monitoring in background..."
        for i in {1..120}; do  # Monitor for 20 minutes (120 * 10 seconds)
            for app in "${ingress_apps[@]}"; do
                if kubectl get application "$app" -n argocd >/dev/null 2>&1; then
                    echo "🚫 Detected redeployment of $app - removing immediately and blocking sync..."
                    
                    # Immediately suspend the application
                    kubectl patch application "$app" -n argocd --type=merge \
                        -p='{"spec":{"syncPolicy":{"automated":null},"project":"suspended"}}' 2>/dev/null || true
                    
                    # Remove finalizers and delete
                    kubectl patch application "$app" -n argocd --type=json \
                        -p='[{"op":"remove","path":"/metadata/finalizers"}]' 2>/dev/null || true
                    kubectl delete application "$app" -n argocd --force --grace-period=0 --ignore-not-found=true 2>/dev/null || true
                    
                    # Also remove any helm releases
                    if helm list -n orch-boots 2>/dev/null | grep -q "$app"; then
                        helm uninstall "$app" -n orch-boots --timeout=60s 2>/dev/null || true
                    fi
                fi
            done
            sleep 10
        done
        echo "Ingress application monitoring completed"
    } &
    
    # Store the background job PID for potential cleanup
    INGRESS_MONITOR_PID=$!
    echo "Started ingress monitoring with PID: $INGRESS_MONITOR_PID"
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

# Check if this is an internal deployment and manage ingress applications
if is_internal_deployment; then
    echo "Info: Internal deployment detected. Managing ingress applications."
    disable_argocd_ingress_sync
    remove_ingress_from_argocd_config
    cleanup_ingress_applications_internal
    cleanup_terraform_target_group_bindings
    disable_ingress_applications_in_argocd
    monitor_and_prevent_ingress_redeploy &
    echo "Info: Ingress application management completed for internal deployment."
fi


# Terminate existing sshuttle
terminate_sshuttle

echo "Info: Upgrade completed successfully."
