#!/usr/bin/env bash

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
eks_volume_size                    = 128
eks_desired_size                   = $EKS_DESIRED_SIZE
eks_min_size                       = $EKS_MIN_SIZE
eks_max_size                       = $EKS_MAX_SIZE
eks_node_ami_id                    = "$(get_eks_node_ami)"
eks_volume_type                    = "gp3"
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
EOF

    if [[ -n "$CUSTOMER_TAG" ]]; then
        echo "customer_tag = \"${CUSTOMER_TAG}\""
    fi
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


}


apply_kms() {

dir="${ROOT_DIR}/${ORCH_DIR}/cluster"
sed -i '/module "kms"/,/^}/ s/^\([[:space:]]*\)depends_on/\1# depends_on/' $dir/main.tf
echo "Changing directory to $dir..."
cd "$dir"

echo "Initializing Terraform for environment: $ENV_NAME..."
if terraform init -reconfigure -backend-config="environments/${ENV_NAME}/backend.tf"; then
    echo "✅ Terraform initialization succeeded."
else
    echo "❌ Terraform initialization failed!"
    exit 1
fi

echo "Applying changes for KMS module..."
if terraform apply -target=module.kms -var-file="environments/${ENV_NAME}/variable.tfvar" -auto-approve; then
    echo "✅ Terraform apply for KMS module succeeded."
else
    echo "❌ Terraform apply for KMS module failed!"
    exit 1
fi

}
# Main

if [[ ${COMMAND:-""} != upgrade ]]; then
    # Not called by the provision script, need to parse command line parameters.
    # shellcheck disable=SC2068
    ##parse_params $@
    exit 1
fi

echo "Info: Checking data file..."
if ! check_s3_savedir_empty; then
    download_savedir
    echo "Info: Pulled S3 ${SAVE_DIR}."
fi
 if [[ ! -f ${SAVE_DIR}/${VALUES} ]]; then
     echo -n "Error: There is no value file found."
     exit 1
 fi
# Terminate existing sshuttle
refresh_sshuttle
connect_cluster

echo "Starting action cluster"
action_cluster
apply_kms


# Terminate existing sshuttle
terminate_sshuttle

echo "Info: Upgrade completed successfully."