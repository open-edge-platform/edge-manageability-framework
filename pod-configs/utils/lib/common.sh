#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd) # The repository root directory
SAVE_DIR=$(realpath "${SAVE_DIR:-${ROOT_DIR}/SAVEME}")
ORCH_DIR="orchestrator"

S3_REMOVE_TIMEOUT="10m"

# Commonly used functions

all_data_files() {
    echo ${JUMPHOST_SSHKEY} ${JUMPHOST_SSHKEY}.pub ${OUTPUT} ${VALUES} ${PROFILE_TFVAR} ${VARIABLE_TFVAR} ${S3_PENDING}
}

all_data_files_v2() {
    echo ${JUMPHOST_SSHKEY} ${JUMPHOST_SSHKEY}.pub ${OUTPUT} ${VALUES} ${VPCSTATE} ${PROFILE_TFVAR} ${VARIABLE_TFVAR} ${S3_PENDING}
}

all_cert_files() {
    echo ${PRIVKEY} ${FULLCHAIN} ${CHAIN}
}

# Check for an active session and fail out if not present.
check_aws_auth() {
    aws sts get-caller-identity 2>&1 >/dev/null
    if [[ $? -ne 0 ]]; then
        echo "Error: AWS credentials missing or expired. Please refresh AWS credentials to proceed."
        return 1
    fi
    return 0
}

# Set AWS_ACCOUNT in the environment if a logged in session is active
get_session_aws_account() {
    aws sts get-caller-identity 2>&1 >/dev/null
    if [[ $? -eq 0 ]]; then
        aws sts get-caller-identity --query 'Account' --output text
    fi
}

check_s3_savedir_file() {
    fn=$1

    if aws s3 ls --region $BUCKET_REGION s3://${BUCKET_NAME}/${AWS_REGION}/${SAVE_DIR_S3}/${fn} &>/dev/null; then
        return 0
    fi

    return 1
}

check_s3_savedir_empty() {
    files=$(all_data_files)

    for file in $files; do
        check_s3_savedir_file ${file} && return 1
    done

    if $AUTO_CERT; then
        files=$(all_cert_files)

        for file in $files; do
            check_s3_savedir_file ${file} && return 1
        done
    fi

    true
}

check_local_savedir_file() {
    fn=$1

    test -f ${SAVE_DIR}/${fn}
}

check_local_savedir_empty() {
    files=$(all_data_files)

    for file in $files; do
        check_local_savedir_file ${file} && return 1
    done

    if $AUTO_CERT; then
        files=$(all_cert_files)

        for file in $files; do
            check_local_savedir_file ${file} && return 1
        done
    fi

    true
}

check_savedir() {
    if [[ ! -d ${SAVE_DIR} ]]; then
        mkdir -p ${SAVE_DIR}
    fi
    if [[ ! -z "$VARIABLE_TFVAR" ]] && [[ ! -f "${SAVE_DIR}/$VARIABLE_TFVAR" ]]; then
        touch "${SAVE_DIR}/$VARIABLE_TFVAR"
    fi
}

upload_savedir_file() {
    fn=$1

    file="${SAVE_DIR}/$fn"
    if ! [[ -f $file ]]; then
        echo "Warning: $file doesn't exist."
        return
    fi

    aws s3 cp --region $BUCKET_REGION ${file} s3://${BUCKET_NAME}/${AWS_REGION}/${SAVE_DIR_S3}/${fn}
}

delete_s3_savedir_file () {
    fn=$1
    aws s3 rm --region $BUCKET_REGION s3://${BUCKET_NAME}/${AWS_REGION}/${SAVE_DIR_S3}/${fn}
}

upload_savedir() {
    files=$(all_data_files_v2)
    for file in $files; do
        if [[ -f ${SAVE_DIR}/${file} ]]; then
            upload_savedir_file ${file}
        fi
    done

    if $AUTO_CERT; then
        files=$(all_cert_files)
        for file in $files; do
            check_local_savedir_file ${file} && upload_savedir_file ${file}
        done
    fi
}

remove_s3_data() {
    files=$(all_data_files_v2)
    for file in $files; do
        delete_s3_savedir_file ${file}
    done
}

connect_cluster() {
    aws eks update-kubeconfig --name "${ENV_NAME}" --region "${AWS_REGION}" --kubeconfig "${KUBECONFIG}"
}

check_proxy_settings() {
    local https_proxy_host=""
    local socks_proxy_host=""

    # Fix up different cased proxy variables to ensure all scripts are functional. Warn on mismatched settings.
    if [[ -z "${HTTPS_PROXY-}" && -n "${https_proxy-}" ]]; then
        export HTTPS_PROXY=$https_proxy
    fi
    if [[ -z "${https_proxy-}" && -n "${HTTPS_PROXY-}" ]]; then
        export https_proxy=$HTTPS_PROXY
    fi
    if [[ "${HTTPS_PROXY-}" != "${https_proxy-}" ]]; then
        echo "Warning: mismatched HTTPS_PROXY and https_proxy. This may cause deployment failures."
    fi

    if [[ -z "${HTTP_PROXY-}" && -n "${http_proxy-}" ]]; then
        export HTTP_PROXY=$http_proxy
    fi
    if [[ -z "${http_proxy-}" && -n "${HTTP_PROXY-}" ]]; then
        export http_proxy=$HTTP_PROXY
    fi
    if [[ "${HTTP_PROXY-}" != "${http_proxy-}" ]]; then
        echo "Warning: mismatched HTTP_PROXY and http_proxy. This may cause deployment failures."
    fi

    if [[ -z "${NO_PROXY-}" && -n "${no_proxy-}" ]]; then
        export NO_PROXY=$no_proxy
    fi
    if [[ -z "${no_proxy-}" && -n "${NO_PROXY-}" ]]; then
        export no_proxy=$NO_PROXY
    fi
    if [[ "${NO_PROXY-}" != "${no_proxy-}" ]]; then
        echo "Warning: mismatched NO_PROXY and no_proxy. This may cause deployment failures."
    fi

    if ! ${INTERNAL-false}; then
        if [[ -z "${SOCKS_PROXY-}" && -n "${socks_proxy-}" ]]; then
            export SOCKS_PROXY=$socks_proxy
        fi
        if [[ -z "${socks_proxy-}" && -n "${SOCKS_PROXY-}" ]]; then
            export socks_proxy=$SOCKS_PROXY
        fi
        if [[ "${SOCKS_PROXY-}" != "${socks_proxy-}" ]]; then
            echo "Warning: mismatched SOCKS_PROXY and socks_proxy. This may cause deployment failures."
        fi
    else
        export SOCKS_PROXY=""
        export socks_proxy=""
    fi

    if [[ -n "${NO_PROXY-}" ]]; then
        if echo "${NO_PROXY}" | grep -q 'eks.amazonaws.com'; then
            # Return success, HTTPS_PROXY and SOCKS_PROXY should not match in this case
            return 0
        fi
    fi

    if [[ -n "${HTTPS_PROXY-}" ]]; then
        https_proxy_host=$(echo "$HTTPS_PROXY" | sed -E 's|.*//([^:]+):.*|\1|')
    fi
    #echo "https_proxy_host: $https_proxy_host"

    if [[ -n "${SOCKS_PROXY-}" ]]; then
        socks_proxy_host=$(echo "$SOCKS_PROXY" | sed -E 's|([^:]+):.*|\1|')
    fi
    #echo "socks_proxy_host: $socks_proxy_host"

    if [[ "$socks_proxy_host" != "$https_proxy_host" ]]; then
        if [[ -z "$socks_proxy_host" || -z "$https_proxy_host" ]]; then
            if [[ -z "$socks_proxy_host" ]]; then
                empty_value="SOCKS_PROXY"
            fi
            if [[ -z "$https_proxy_host" ]]; then
                empty_value="HTTPS_PROXY"
            fi
            echo "Error: proxy settings mismatch: $empty_value is unset. If HTTPS_PROXY is set, SOCKS_PROXY must be set as well. If HTTPS_PROXY is unset, SOCKS_PROXY must be unset as well."
            return 1
        else
            echo "Warning: mismatched HTTPS_PROXY and SOCKS_PROXY. This may cause tunnel connection failures."
        fi
    fi
    return 0
}

download_savedir_file() {
    fn=$1
    if ! aws s3 cp --region $BUCKET_REGION s3://${BUCKET_NAME}/${AWS_REGION}/${SAVE_DIR_S3}/${fn} ${SAVE_DIR}/${fn} >/dev/null; then
        echo "Info: File s3://${BUCKET_NAME}/${AWS_REGION}/${SAVE_DIR_S3}/${fn} does not exist."
    else
        echo "Info: Download ${fn}."
    fi
}

download_savedir() {
    files=$(all_data_files)
    for file in $files; do
        if check_s3_savedir_file ${file}; then
            download_savedir_file ${file}
            chmod 600 ${SAVE_DIR}/${file}
        fi
    done

    if $AUTO_CERT; then
        files=$(all_cert_files)
        for file in $files; do
            if check_s3_savedir_file ${file}; then
                download_savedir_file ${file}
                chmod 600 ${SAVE_DIR}/${file}
            fi
        done
    fi
}

load_values() {
    if [[ -f "${SAVE_DIR}/${VALUES}" ]]; then
        source ${SAVE_DIR}/${VALUES}
    else
        echo "Warning: No value file is found."
    fi
}


get_end_cert() {
    echo "$1" | awk -v n=1 'split_after==1 {n++;split_after=0} /-----END CERTIFICATE-----/ {split_after=1} {if(n==1) print}' | sed -e 's/-----END CERTIFICATE-----/-----END CERTIFICATE-----\n/g'
}

jumphost_ip() {
    if [[ -n "$JUMPHOST_IP" ]]; then
        echo $JUMPHOST_IP
        return
    fi

    outfile="${SAVE_DIR}/${BUCKET_NAME}-${AWS_REGION}-vpc-${ENV_NAME}.json"
    aws s3api get-object --region "${BUCKET_REGION}" --bucket ${BUCKET_NAME} --key ${AWS_REGION}/vpc/${ENV_NAME} $outfile > /dev/null
    s="$(jq -r '.resources[] | select((.module == "module.jumphost") and (.type == "aws_eip")) | .instances[0].attributes.public_ip' $outfile)"
    echo -n $s
}

check_running_sshuttle() {
    if ps -e -o pid -o command= | grep -v grep | grep "$VPC_CIDR" | grep -q sshuttle; then
        echo "Warning: There has been already a sshuttle running process for $VPC_CIDR. It has to be terminated before the provision can continue."
        echo "         Note that terminating the running sshuttle process could impact other applications."
        echo -n "         Enter 'yes' to terminate the running shuttle process and continue the provision. Enter others to stop the current provision process: "; read s
        s=$(echo $s | tr '[:upper:]' '[:lower:]')
        if [[ "$s" == "yes" ]]; then
            terminate_sshuttle
        else
            exit
        fi
    fi
}

refresh_sshuttle() {
    terminate_sshuttle
    start_sshuttle
}

start_sshuttle() {
    jip=$(jumphost_ip)
    if [[ -z "$jip" ]]; then
        echo "Error: Cannot get jumphost IP address."
        exit 1
    fi

    echo "Info: Starting SSH tunnel..."
    ret="true"
    if [[ -z "${SOCKS_PROXY}" ]]; then
      sshuttle -D -r ubuntu@"$jip" $VPC_CIDR --ssh-cmd "ssh -o StrictHostKeyChecking=no -o ServerAliveInterval=120 -i ${SAVE_DIR}/${JUMPHOST_SSHKEY}" || ret="false"
    else
      sshuttle -D -r ubuntu@"$jip" $VPC_CIDR --ssh-cmd "ssh -o StrictHostKeyChecking=no -o ServerAliveInterval=120 -i ${SAVE_DIR}/${JUMPHOST_SSHKEY} -o ProxyCommand='nc -X 5 -x ${SOCKS_PROXY} %h %p'" || ret="false"
    fi

    if $ret; then
        echo "Info: SSH tunnel created."
    else
        echo "Error: Not able to create SSH tunnel. Please check the network connectivity."
        exit 1
    fi
}

terminate_sshuttle() {
    ps -e -o pid -o command= | grep -v grep | grep "$VPC_CIDR" | grep sshuttle |awk '{s=sprintf("kill -9 %s",$1); system(s);}' || true
}

bucket_backend() {
    cat << EOF
path="environments/${ENV_NAME}/${BUCKET_NAME}.tfstate"
EOF
}

bucket_variable() {
    cat <<EOF
bucket="$BUCKET_NAME"
region="$BUCKET_REGION"
EOF
}

create_bucket() {
    echo "Info: Creating Bucket ${BUCKET_NAME} ..."
    dir="${ROOT_DIR}/buckets/environments/${ENV_NAME}"
    [[ ! -d ${dir} ]] && mkdir -p "${dir}"

    backend="$(bucket_backend)" && echo "$backend" > $dir/backend.tf
    variable="$(bucket_variable)" && echo "$variable" > $dir/variable.tfvar

    apply_terraform "${ROOT_DIR}/buckets" "apply" "$dir/backend.tf" "$dir/variable.tfvar" "$SAVE_DIR/$VARIABLE_TFVAR"
}

check_bucket() {
    if aws s3 ls "s3://${BUCKET_NAME}" --region "${BUCKET_REGION}" &>/dev/null ; then
        echo "Info: Bucket ${BUCKET_NAME} has already existed. Skip it."
        return
    fi

    create_bucket

    cp ${ROOT_DIR}/buckets/environments/${ENV_NAME}/${BUCKET_NAME}.tfstate $SAVE_DIR
    dir="${ROOT_DIR}/buckets/environments/${ENV_NAME}"
    rm -rf $dir
}

aws_admin_role() {
    case $1 in
        # The roles generated by the organization
        "054305100460") echo '"AWSReservedSSO_AWSAdministratorAccess_933fc287558617cc", "AWSReservedSSO_Developer_EKS_054305100460_52b02cdf70e84917"';;
        "316277718646") echo '"AWSReservedSSO_AWSAdministratorAccess_f9b77156746d4389", "AWSReservedSSO_Developer_EKS_316277718646_529a2164a5fb2d53"';;
        "955337957877") echo '"AWSReservedSSO_AWSAdministratorAccess_a383d0bd17085739"';;
        *) echo '"AWSReservedSSO_AWSAdministratorAccess"';;
    esac
}

# 24.08.0
get_eksnode_ami() {
    version=$1
    aws ssm get-parameter --name /aws/service/eks/optimized-ami/$version/amazon-linux-2/recommended/image_id --region $AWS_REGION --query "Parameter.Value" --output text
}

get_eks_version() {
    aws eks --region $AWS_REGION describe-cluster --name $ENV_NAME --query "cluster.version" | tr -d '"'
}

get_secret() {
    # $1: namespace
    # $2: secret name
    # $3: key
    kubectl --kubeconfig "${KUBECONFIG}" get secret --namespace $1 $2 -o jsonpath={.data.$3} | base64 --decode
}

get_cert_expire() {
    cert_content="$1"
    if ! expiry_date=$(date -d $(openssl x509 -noout -enddate -in <(echo "$cert_content") 2>/dev/null | cut -d= -f2|awk '{print $1"-"$2"-"$4}') +'%s' 2>/dev/null); then
        return 1
    fi
    today=$(date +'%s')
    diff=$(echo $(( ( expiry_date - today )/(60*60*24) )) )
    echo $diff
}

get_deployed_version() {
    rm -f "${KUBECONFIG}"
    aws eks --region ${AWS_REGION} update-kubeconfig --name ${ENV_NAME} --kubeconfig "${KUBECONFIG}" &> /dev/null
    if v=$(helm ls -A -o yaml --kubeconfig "${KUBECONFIG}"| yq -r '.[] | select(.name=="root-app") | .app_version' | sed -ne 's|\([^\-]\+\).*|\1|p') && [[ -n ${v:-""} ]]; then
        # There is no letter v leading in the above command output
        echo $v
    elif v=$(kubectl get applications -n ${ENV_NAME} root-app -o yaml --kubeconfig "${KUBECONFIG}" | yq '.spec.sources[] | select(has("helm")) | .helm.valuesObject.argo.orchestratorVersion' | sed -ne 's|v\([^\-]\+\).*|\1|p') && [[ -n ${v:-""} ]]; then
        # There is a letter v leading in the above command output
        echo $v
    else
        echo "Unable to find the deployed version"
        exit 1
    fi
}

apply_terraform() {
    if [ "$#" -le 3 ]; then
        echo "Usage apply_terraform [module dir] [apply|destroy|plan] [backend file] [variable files...]"
        exit 1
    fi

    AUTO_APPROVE=${AUTO_APPROVE:-}
    MODULE_DIR=$1
    ACTION=$2
    BACKEND_CONFIG=$3
    shift 3;
    VARIABLE_FILES=( "$@" )

    if [ "$ACTION" != "apply" ] && [ "$ACTION" != "destroy" ] && [ "$ACTION" != "plan" ]; then
        echo "Usage apply_terraform [module dir] [apply|destroy|plan] [backend file] [variable files...]"
        exit 1
    fi

    pushd "$MODULE_DIR" || exit 1

    rm -rf .terraform* || true
    terraform init -reconfigure -backend-config "$BACKEND_CONFIG"

    if [ "$ACTION" == "destroy" ] && [[ "$MODULE_DIR" == *"${ORCH_DIR}/cluster" ]]; then
        # Remove all S3 buckets which are left over from previous setup if there is any.
        remove_s3_pending || true
        # Empty all S3 buckets which belong to the current cluster.
        empty_current_s3buckets || true

        terraform state rm module.aurora_database 2>/dev/null || true
        terraform state rm module.aurora_import 2>/dev/null || true
        terraform state rm module.orch_init 2>/dev/null || true
        terraform state rm module.kms.kubernetes_secret.vault_kms_unseal 2>/dev/null || true
        terraform state rm module.gitea 2>/dev/null || true

        # Remove the SSM document since Terraform doesn't update the state file
        # See https://github.com/hashicorp/terraform-provider-aws/issues/42127
        terraform state rm module.ec2log[0].aws_ssm_document.push_log 2>/dev/null || true
        aws ssm delete-document --region ${AWS_REGION} --name orch-ec2log-${ENV_NAME}-term 2>/dev/null || true
    fi

    if [[ "$MODULE_DIR" == *"${ORCH_DIR}/cluster" ]]; then
        s3_prefix=$(get_variable_value "s3_prefix" ${VARIABLE_FILES[*]})
    fi

    if [ "$ACTION" == "apply" ] && [[ "$MODULE_DIR" == *"${ORCH_DIR}/cluster" ]]; then
        import_pending=false
        if determine_import_s3_bucket $s3_prefix; then
            import_pending=true
        fi
    fi

    if [ "$ACTION" == "destroy" ] && [[ "$MODULE_DIR" == *"${ORCH_DIR}/release-service-token-renewal" ]]; then
        terraform state rm module.release_service_token_renewal.module.token.aws_secretsmanager_secret_version.secret_value
    fi

    if [ -n "$AUTO_APPROVE" ]; then
        cmd="terraform $ACTION -auto-approve -compact-warnings -no-color"
    else
        cmd="terraform $ACTION -compact-warnings -no-color"
    fi

    for var_file in "${VARIABLE_FILES[@]}"; do
        cmd="$cmd -var-file $var_file"
    done

    if [ "$ACTION" == "apply" ] && [[ "$MODULE_DIR" == *"${ORCH_DIR}/cluster" ]]; then
        if $import_pending; then
            # Create EKS only first before the import of S3 buckets can occur
            cmd_eks="$cmd -target=module.eks"
            eval "$cmd_eks"
            import_s3_pending "$s3_prefix"
        fi
    fi

    eval "$cmd" || return 1

    if [ "$ACTION" == "destroy" ] && [[ "$MODULE_DIR" == *"${ORCH_DIR}/cluster" ]]; then
        expire_all_s3buckets
    fi

    popd || exit 1 # pushd "$MODULE_DIR"
}

profile_variables() {
    # Usage profile_variables [profile_name]
    if [[ $# != 1 ]]; then
        echo "Usage: profile_variables [profile_name]" >&2
        exit 1
    fi

    local profile_name="$1"
    local profile_config="$ROOT_DIR/utils/provisioning/profiles.yaml"
    local profile_yaml=$(yq -r ".[] | select(.name == \"$profile_name\")" $profile_config)
    if [[ -z $profile_yaml ]]; then
        echo "Error: Profile $profile_name not found." >&2
        exit 1
    fi

    local tmp_file=$(mktemp)
    echo "$profile_yaml" > "$tmp_file"

    local item_output=$(yq -r ".variables.[].variable" $tmp_file)
    local IFS=$'\n' var_vars=($item_output)

    for i in $(seq 0 $((${#var_vars[@]} - 1))); do
        local n=$(yq -r ".variables.[$i].variable" $tmp_file)
        local v=$(yq -r ".variables.[$i].value" $tmp_file)
        local t=$(yq -r ".variables.[$i].type" $tmp_file)
        if [[ $t == string ]]; then
            v="\"$v\""
        fi
        echo "$n=$v"
    done

    rm -f "$tmp_file"
}

wait_for_gitea() {
    echo "Info: Waiting for Gitea to be up..."
    for i in {1..20}; do
        if ! curl -s -w "%{http_code}\n" https://gitea.$ROOT_DOMAIN -o /dev/null | grep -q 200; then
            echo "Waiting for Gitea to be up... $i retries"
            sleep 15
            continue
        fi
        break
    done

    if [[ $i == 20 ]]; then
        echo "Unable to reach Gitea from installer"
        exit 1
    fi
    echo "Done"
}

get_running_eks_node_ami() {
    aws eks describe-nodegroup --cluster-name ${ENV_NAME} --nodegroup-name nodegroup-${ENV_NAME} --region ${AWS_REGION} --output json | jq -r '.nodegroup.releaseVersion'
}

empty_current_s3buckets() {
    # Important: The backend must have been initialized before this function is called.
    # The curent directory must be the cluster dirctory.
    buckets=$(get_s3buckets)

    s3_empty=true
    for bucket in $buckets; do
        if ! empty_s3bucket $bucket; then
            echo "Info: Some S3 bucket cannot be empty due to large amount of content."
            s3_empty=false
            break
        fi
    done

    if $s3_empty; then
        return
    fi

    for bucket in $buckets; do
        resource=$(s3bucket_resource $bucket)
        terraform state rm "$resource" || true
        echo "$bucket" >> "${SAVE_DIR}/${S3_PENDING}"
    done

    upload_savedir_file $S3_PENDING
}

empty_s3bucket() {
    bucket="$1"  # S3 bucket to be emptied
    ret=0
    echo "Info: Emptying S3 bucket ${bucket}. It could take a couple of minutes ..."
    timeout $S3_REMOVE_TIMEOUT aws s3 rm s3://"${bucket}" --recursive --region "${AWS_REGION}" &>/dev/null || ret=$?
    return $ret
}

get_s3buckets() {
    # Important: The backend must have been initialized before this function is called.
    # The curent directory must be the cluster dirctory.

    for resource in $(terraform state list | grep -oP '^module.s3.aws_s3_bucket.bucket\[".+"\]$'); do
        terraform state show "$resource" | grep -P "^\s* bucket\s*=" | cut -d'=' -f2 | xargs echo
    done
}

expire_all_s3buckets() {
    if [[ ! -f "${SAVE_DIR}/${S3_PENDING}" ]]; then
        return
    fi

    expire_rulefile="s3-expire-lifecycle.json"
    cat <<EOT > $expire_rulefile
{
    "Rules": [{
            "Expiration": {
                "Days": 1
            },
            "ID": "FullDelete",
            "Filter": {
                "Prefix": ""
            },
            "Status": "Enabled",
            "NoncurrentVersionExpiration": {
                "NoncurrentDays": 1
            },
            "AbortIncompleteMultipartUpload": {
                "DaysAfterInitiation": 1
            }
        },
        {
            "Expiration": {
                "ExpiredObjectDeleteMarker": true
            },
            "ID": "DeleteMarkers",
            "Filter": {
                "Prefix": ""
            },
            "Status": "Enabled"
        }
    ]
}
EOT

    for bucket in $(cat "${SAVE_DIR}/${S3_PENDING}"); do
        if check_s3bucket_exist $bucket; then
            expire_s3bucket $bucket $expire_rulefile
        fi
    done

    rm $expire_rulefile
}

expire_s3bucket() {
    bucket="$1"
    rule_file="$2"
    aws s3api put-bucket-lifecycle-configuration --bucket "$bucket" --region "${AWS_REGION}" --lifecycle-configuration "file://$rule_file"
}

s3bucket_resource() {
    # Get S3 resource name with bucket name
    bucket="$1"
    key=$(echo $bucket | sed -ne 's|.\+-\([^-]\+-[^-]\+-[^-]\+\)$|\1|p')
    echo "module.s3.aws_s3_bucket.bucket[\"${key}\"]"
}

has_s3_pending() {
    if [[ ! -f ${SAVE_DIR}/${S3_PENDING} ]]; then
        return 1
    fi

    verify_s3_pending

    if [[ -n "$(cat ${SAVE_DIR}/${S3_PENDING})" ]]; then
        return 0
    else
        rm ${SAVE_DIR}/${S3_PENDING}
        return 1
    fi
}

remove_s3_pending() {
    if [[ ! -f ${SAVE_DIR}/${S3_PENDING} ]]; then
        return
    fi

    buckets=$(cat ${SAVE_DIR}/${S3_PENDING}) || return 0

    for bucket in $buckets; do
        if aws s3 ls --region "${AWS_REGION}" ${bucket} &>/dev/null; then
            echo -n "Info: Removing S3 bucket ${bucket}. It could take a couple of minutes ... "
            if timeout $S3_REMOVE_TIMEOUT aws s3 rm s3://"${bucket}" --recursive --region "${AWS_REGION}" &>/dev/null && \
                timeout $S3_REMOVE_TIMEOUT aws s3 rb s3://"${bucket}" --region "${AWS_REGION}" &>/dev/null; then
                sed -ie "/^${bucket}\$/d" ${SAVE_DIR}/${S3_PENDING}
                echo "The bucket contains large amount of data and cannot be removed completely for now."
            else
                echo "OK."
            fi
        fi
    done
}

verify_s3_pending() {
    for b in $(cat ${SAVE_DIR}/${S3_PENDING}); do
        if ! check_s3bucket_exist $b; then
            sed -ie "/^${b}\$/d" ${SAVE_DIR}/${S3_PENDING}
        fi
    done
}

get_variable_value() {
    # Get value of the variable from the variable files and the environment
    variable="$1"
    shift 1
    variable_files=( "$@" )

    i=$(( ${#variable_files[@]} - 1 ))
    while [[ $i -ge 0 ]]; do
        f="${variable_files[$i]}"
        (( i-- )) || true
        if [[ ! -f $f ]]; then
            continue
        fi
        prefix=$(cat $f | grep -P "^\\s*${variable}\\s*=" | sed -ne "s|^\\s*${variable}\\s*=\\s*\\([^\\s]\\+\\)|\\1|p") || true
        if [[ -n "$prefix" ]]; then
            eval echo $prefix
            return
        fi
    done

    # If the variable is not found in any of the variable file:
    tf_var="TF_VAR_${variable}"
    echo ${!tf_var:-""}
}

determine_import_s3_bucket() {
    prefix="$1"

    if [[ ! -f ${SAVE_DIR}/${S3_PENDING} ]]; then
        return 1
    fi

    grep -q "^${ENV_NAME}-${prefix}" ${SAVE_DIR}/${S3_PENDING}
}

import_s3_pending() {
    prefix="$1"

    if [[ ! -f ${SAVE_DIR}/${S3_PENDING} ]]; then
        return
    fi
    touch ${ROOT_DIR}/module/ec2log/ssm-term.py
    buckets=$(grep "^${ENV_NAME}-${prefix}" ${SAVE_DIR}/${S3_PENDING}) || return 0
    export TF_VAR_import_s3_buckets=true
    for bucket in $buckets; do
        echo "Info: Import $bucket if the bucket exists."
        resource=$(s3bucket_resource $bucket)
        terraform import -var-file environments/${ENV_NAME}/variable.tfvar "$resource" "$bucket" || true
        sed -ie "/^${bucket}\$/d" ${SAVE_DIR}/${S3_PENDING}
    done
    delete_s3_savedir_file $S3_PENDING
    upload_savedir_file $S3_PENDING
    rm ${ROOT_DIR}/module/ec2log/ssm-term.py
    export TF_VAR_import_s3_buckets=false
}

check_s3bucket_exist() {
    bucket=$1
    aws s3 ls "s3://${bucket}" --region "${AWS_REGION}" &>/dev/null
}
