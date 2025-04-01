#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Get the directory of the calling script
SCRIPT_DIR=$(dirname "$(realpath "$0")")

if [ -f "${SCRIPT_DIR}/pod-configs/utils/lib/common.sh" ]; then
    . ${SCRIPT_DIR}/pod-configs/utils/lib/common.sh
elif [ -f "${SCRIPT_DIR}/common.sh" ]; then
    . ${SCRIPT_DIR}/common.sh
else
    echo "Error: Unable to load common.sh"
    exit 1
fi

load_provision_env() {
    # Does $HOME/.env exist? If so, load it.
    if [ -f ${HOME}/.env ]; then
        export $(cat ${HOME}/.env | xargs)
    fi
}

save_cluster_env() {
    if [[ -z $AWS_ACCOUNT ]]; then
        echo AWS_ACCOUNT=${AWS_ACCOUNT} > ${HOME}/.env
    fi

    if [[ -z $AWS_REGION ]]; then
        echo AWS_REGION=${AWS_REGION} >> ${HOME}/.env
    fi

    if [[ -z $BUCKET_NAME ]]; then
        echo BUCKET_NAME=${BUCKET_NAME} >> ${HOME}/.env
    fi

    if [[ -z $CLUSTER_FQDN ]]; then
        echo CLUSTER_FQDN=${CLUSTER_FQDN} >> ${HOME}/.env
    fi

    if [[ -z $ADMIN_EMAIL ]]; then
        echo ADMIN_EMAIL=${ADMIN_EMAIL} >> ${HOME}/.env
    fi
}

update_kube_config() {
    echo "  Applying Kubernetes context for ${CLUSTER_NAME} in ${AWS_REGION}"
    aws eks --region ${AWS_REGION} update-kubeconfig --name ${CLUSTER_NAME}
}

load_provision_values() {
    PROVISION_CONFIG_VALUES=$(echo ${SAVE_DIR}/${AWS_ACCOUNT}-${CLUSTER_NAME}-values.tfvar | tr -d '"')
    if [ ! -f ${PROVISION_CONFIG_VALUES} ]; then
        download_savedir
        if [ ! -f ${PROVISION_CONFIG_VALUES} ]; then
            echo "Error: Unable to load provision configuration values."
            return 1
        fi
    fi
    
    # Extract and export specific variables from the tfvar file
    while IFS='=' read -r key value; do
        # Skip lines that are comments or empty
        if [[ $key =~ ^#.*$ || -z $key ]]; then
            continue
        fi

        # Remove spaces and quotes from the key
        key=$(echo "$key" | tr -d ' ')

        # Trim leading/trailing spaces and quotes from the value
        value=$(echo "$value" | sed 's/^ *//;s/ *$//' | tr -d '"')

       # Export only the required variables
        case $key in
            sre_secret_string)
                export SRE_SECRET_STRING="$value"
                ;;
            auto_cert)
                export AUTO_CERT="$value"
                ;;
            smtp_url)
                export SMTP_URL="$value"
                ;;
        esac
    done < ${PROVISION_CONFIG_VALUES}
}

download_tunnel_config() {
    TUNNEL_CONFIG_VALUES="${SAVE_DIR}/${AWS_ACCOUNT}-${CUSTOMER_STATE_PREFIX}-${AWS_REGION}-vpc-${CLUSTER_NAME}.json"
    if [ ! -f ${TUNNEL_CONFIG_VALUES} ]; then
        download_savedir

        # TBD: Is it worth being selective?
        # download_savedir_file "${AWS_ACCOUNT}-${CUSTOMER_STATE_PREFIX}-${AWS_REGION}-vpc-${CLUSTER_NAME}.json"
        # download_savedir_file "jumphost_sshkey_${CLUSTER_NAME}"
        # download_savedir_file "jumphost_sshkey_${CLUSTER_NAME}.pub"

        if [ ! -f ${TUNNEL_CONFIG_VALUES} ]; then
            echo "Error: Unable to load tunnel configuration values."
            return 1
        fi
    fi
}

load_scm_auth() {
    if [[ ! -f ${SAVE_DIR}/output-${AWS_ACCOUNT}-${CLUSTER_NAME}.json ]]; then
        # Pull the file from S3
        # aws s3 cp "s3://${BUCKET_NAME}/${AWS_REGION}/${CLUSTER_NAME}-SAVEME/output-${AWS_ACCOUNT}-${CLUSTER_NAME}.json" "${SAVE_DIR}/output-${AWS_ACCOUNT}-${CLUSTER_NAME}.json"

        # Pull the full cluster context from S3
        download_savedir

        if [ ! -f ${PROVISION_CONFIG_VALUES} ]; then
            echo "Error: Unable to load provision configuration values."
            return 1
        fi
    fi

    echo Looking up Git credentials...
    export GIT_USER=$(jq '.git_user' ${SAVE_DIR}/output-${AWS_ACCOUNT}-${CLUSTER_NAME}.json)
    export GIT_TOKEN=$(jq '.git_token' ${SAVE_DIR}/output-${AWS_ACCOUNT}-${CLUSTER_NAME}.json)
    export GITEA_ARGO_USER=$(jq '.gitea_argo_user' ${SAVE_DIR}/output-${AWS_ACCOUNT}-${CLUSTER_NAME}.json)
    export GITEA_ARGO_TOKEN=$(jq '.gitea_argo_token' ${SAVE_DIR}/output-${AWS_ACCOUNT}-${CLUSTER_NAME}.json)
    export GITEA_CO_USER=$(jq '.gitea_co_user' ${SAVE_DIR}/output-${AWS_ACCOUNT}-${CLUSTER_NAME}.json)
    export GITEA_CO_TOKEN=$(jq '.gitea_co_token' ${SAVE_DIR}/output-${AWS_ACCOUNT}-${CLUSTER_NAME}.json)

    if [[ (-z "${GIT_USER}" || -z "${GIT_TOKEN}") && (-z "${GITEA_ARGO_USER}" || -z "${GITEA_ARGO_TOKEN}") ]]; then
        echo "Error: Git credential lookup failed. Please define git credentials and re-execute this script"
        echo "GIT_USER and GIT_TOKEN are required for CodeCommit"
        echo "GITEA_ARGO_USER and GITEA_ARGO_TOKEN are required for Gitea"
        return 1
    fi

    return 0
}

save_scm_auth() {
    if [[ -n "${GIT_USER}" && -n "${GIT_TOKEN}" ]]; then
        echo "machine git-codecommit.${AWS_REGION}.amazonaws.com login ${GIT_USER} password ${GIT_TOKEN}" | sed "s/'//g; s/\"//g" > ~/.netrc
    fi
    echo GIT_USER=${GIT_USER} >> ${HOME}/.env
    echo GIT_TOKEN=${GIT_TOKEN} >> ${HOME}/.env

    if [[ -n "${GITEA_ARGO_USER}" && -n "${GITEA_ARGO_TOKEN}" ]]; then
        echo "machine gitea.${CLUSTER_FQDN} login ${GITEA_ARGO_USER} password ${GITEA_ARGO_TOKEN}" | sed "s/'//g; s/\"//g" >> ~/.netrc
    fi
    echo GITEA_ARGO_USER=${GITEA_ARGO_USER} >> ${HOME}/.env
    echo GITEA_ARGO_TOKEN=${GITEA_ARGO_TOKEN} >> ${HOME}/.env

    if [[ -n "${GITEA_CO_USER}" && -n "${GITEA_CO_TOKEN}" ]]; then
        echo "machine gitea.${CLUSTER_FQDN} login ${GITEA_CO_USER} password ${GITEA_CO_TOKEN}" | sed "s/'//g; s/\"//g" >> ~/.netrc
    fi
    echo GITEA_ARGO_USER=${GITEA_CO_USER} >> ${HOME}/.env
    echo GITEA_ARGO_TOKEN=${GITEA_CO_TOKEN} >> ${HOME}/.env
}

check_provision_env() {
    verbose=0
    prompt=0
    exit_code=0
    if [[ $1 == "-v" ]]; then
        verbose=1
    fi
    if [[ $1 == "-p" ]]; then
        prompt=1
    fi

    # Verified in AWS actions. Not needed here.
    # Check for the required environment variables
    # if [[ -z $AWS_ACCOUNT ]]; then
    #     if [[ $verbose -eq 1 ]]; then
    #         echo "AWS_ACCOUNT is not defined."
    #     fi
    #     exit_code=1
    # fi

    # Set on Container Startup. Not needed here.
    # if [[ -z $AWS_REGION ]]; then
    #     if [[ $verbose -eq 1 ]]; then
    #         echo "AWS_REGION is not defined."
    #     fi
    #     exit_code=1
    # fi

    if [[ -z $CLUSTER_FQDN ]]; then
        if [[ $prompt -eq 1 ]]; then
            while [[ $prompt -eq 1 ]]; do
                read -p "Enter the full domain name for the cluster: " CLUSTER_FQDN
                if [[ -n ${CLUSTER_FQDN} ]]; then
                    break
                else
                    echo "Error: A valid cluster domain name is required."
                fi
            done
            export CLUSTER_FQDN=${CLUSTER_FQDN}
        else
            # If not prompting, exit with an error
            if [[ $verbose -eq 1 ]]; then
                echo "CLUSTER_FQDN is not defined."
            fi
            exit_code=1
        fi
    fi

    if [[ -z $ADMIN_EMAIL ]]; then
        if [[ $prompt -eq 1 ]]; then
            while true; do
                read -p "Please provide the administrator email address associated with the cluster's provisioning: " ADMIN_EMAIL
                if [[ -n ${ADMIN_EMAIL} ]]; then
                    break
                else
                    echo "Error: A valid administrator email is required."
                fi
            done
            export ADMIN_EMAIL=${ADMIN_EMAIL}
        else
            # If not prompting, exit with an error
            if [[ $verbose -eq 1 ]]; then
                echo "ADMIN_EMAIL is not defined."
            fi
            exit_code=1
        fi
    fi

    return $exit_code
}

load_cluster_state_env() {
    # Get the cluster values from the local environment and state files in SAVE_DIR or S3

    # AWS_ACCOUNT - based on the current session
    SESSION_ACCOUNT=$(get_session_aws_account)
    if [[ -z "${SESSION_ACCOUNT}" ]]; then
        echo "Error: AWS credentials missing or expired. Please refresh AWS credentials to proceed."
        return 1
    fi
    if [[ -z "${AWS_ACCOUNT}" ]]; then
        export AWS_ACCOUNT=${SESSION_ACCOUNT}
    elif [[ "${AWS_ACCOUNT}" != "${SESSION_ACCOUNT}" ]]; then
        echo "Error: Mismatched AWS session credentials. Current login session account doesn't match deployment account."
        return 1
    fi

    # CUSTOMER_STATE_PREFIX - based on the installer environment or user input
    if [[ -z ${CUSTOMER_STATE_PREFIX} ]]; then
        echo "Error: A valid customer state prefix is required."
    fi
    export BUCKET_NAME="${AWS_ACCOUNT}-${CUSTOMER_STATE_PREFIX}"

    # Build environment variables for all of the state files based on the cluster name
    export ENV_NAME="${CLUSTER_NAME}"
    FULLCHAIN="fullchain-${AWS_ACCOUNT}-${ENV_NAME}.pem"
    CHAIN="chain-${AWS_ACCOUNT}-${ENV_NAME}.pem"
    PRIVKEY="privkey-${AWS_ACCOUNT}-${ENV_NAME}.pem"
    OUTPUT="output-${AWS_ACCOUNT}-${ENV_NAME}.json"
    VPCSTATE="${BUCKET_NAME}-${AWS_REGION}-vpc-${ENV_NAME}.json"
    KUBECONFIG="${SAVE_DIR}/kube-config-${AWS_ACCOUNT}-${ENV_NAME}"
    CODECOMMIT_GITOPS_SSHKEY="codecommit_gitops_sshkey_${ENV_NAME}"
    CODECOMMIT_ADM_SSHKEY="codecommit_adm_sshkey_${ENV_NAME}"
    JUMPHOST_SSHKEY="jumphost_sshkey_${ENV_NAME}"
    VALUES="${AWS_ACCOUNT}-${ENV_NAME}-values.sh"
    VALUES_CHANGED=".${AWS_ACCOUNT}-${ENV_NAME}-valueschanged"
    SAVE_DIR_S3="${ENV_NAME}-SAVEME"

    # CLUSTER_FQDN - based on S3 state or user input
    if [[ -z "$CLUSTER_FQDN" ]]; then
        # Look up FQDN from S3 if not defined.
        export CLUSTER_FQDN=$(aws s3 cp s3://$BUCKET_NAME/$AWS_REGION/orch-route53/$CLUSTER_NAME - | jq '.resources[] | select(.name == "traetik_public") | .instances[0].attributes.fqdn' | tr -d '"' | sed 's/^traefik\.//')
    fi

    # fullchain.pem doesn't have the admin email on it and it is stored nowhere else that I can see.
    # Upgrade doesn't need it and provision will post it to the provision ".env" file so leaving it out for now.

    # # ADMIN_EMAIL - based on certificate contents from S3 or SAVE_DIR or user input
    # if [[ -z "$ADMIN_EMAIL" ]]; then
    #     if [[ ! -f ${SAVE_DIR}/fullchain-${AWS_ACCOUNT}-${CLUSTER_NAME}.pem ]]; then
    #         # Pull the file from S3
    #         aws s3 cp "s3://${BUCKET_NAME}/${AWS_REGION}/${CLUSTER_NAME}-SAVEME/fullchain-${AWS_ACCOUNT}-${CLUSTER_NAME}.pem" "${SAVE_DIR}/fullchain-${AWS_ACCOUNT}-${CLUSTER_NAME}.pem"
    #         if [[ ! -f ${SAVE_DIR}/fullchain-${AWS_ACCOUNT}-${CLUSTER_NAME}.pem ]]; then
    #             echo "Error: Unable to load cluster certificate."
    #             return 1
    #         fi
    #     fi
    #     export ADMIN_EMAIL=$(openssl x509 -in ${SAVE_DIR}/fullchain-${AWS_ACCOUNT}-${CLUSTER_NAME}.pem -noout -email)
    #  fi
}

clone_repo() {
    local repo_url=$1
    local repo_name=$2

    pushd ${HOME}/src

    # Update remote URL if repo exists
    # In upgrade scenario, prepare-upgrade.sh will clone the repos
    if [ -d $repo_name ]; then
        pushd $repo_name
        git remote set-url origin $repo_url
        popd
    # Init git repo for the fresh install scenario
    else
        echo cloning $repo_name from $repo_url...
        git clone "$repo_url" "$repo_name"

        # Create repos if they do not exist on Gitea
        if [ ! -d $repo_name ]; then
            mkdir $repo_name
            pushd $repo_name
            git init
            git remote add origin $repo_url
            popd
        fi

        cd ${repo_name}
        git config user.name "orchestrator_installer"
        git config user.email "orchestrator_installer@local"
        if git show-ref --verify --quiet "refs/remotes/origin/main"; then
            git checkout main
            git pull
        else
            git checkout -b main
            touch README.md
            git add .
            git commit -m "Initialize main branch"
            git push --set-upstream origin main
        fi
    fi

    popd
}

commit_repo() {
    local repo_name=$1
    local comment=${2:-"Update main with latest release"}

    echo updating $repo_name with release contents...
    pushd ${HOME}/src/${repo_name}
    git config user.name "orchestrator_installer"
    git config user.email "orchestrator_installer@local"
    git checkout main
    git add .
    git commit -m "${comment}"
    git push --set-upstream origin main
    popd
}

init_terraform() {
    local podconfigs=$1
    local module=$2
    local state_bucket=$3
    local key=$4

    local dir=${podconfigs}/${module}
    mkdir -p ${dir}/environments/${CLUSTER_NAME} || true
    pushd $dir &>/dev/null

    local backend=environments/${CLUSTER_NAME}/backend.tf
    cat <<EOF > $backend
bucket = "$state_bucket"
key    = "${key}"
region = "${BUCKET_REGION:-"us-west-2"}" 
EOF

    rm -rf .terraform || true
    terraform init -backend-config $backend >/dev/null
}

cleanup_init_terraform() {
    # Must be called after init_terraform and on the same directory.
    rm -rf .terraform || true
    popd &>/dev/null
}

get_s3_prefix() {
    init_terraform "pod-configs" "orchestrator/cluster" "$BUCKET_NAME" "${AWS_REGION}/cluster/${CLUSTER_NAME}"

    local resource
    
    if resource=$(terraform state list | grep -oP '^module.s3.aws_s3_bucket.bucket\[".+"\]$' | head -1) && \
        [[ -n "$resource" ]]; then
        local bucket=$(terraform state show "$resource" | grep -P "^\s* bucket\s*=" | cut -d'=' -f2 | xargs echo)
        echo $bucket | sed -ne "s|^${CLUSTER_NAME}-\([^-]\+\)-.*$|\1|p"
    else
        echo "Error: Not able to get the info of the S3 bucket for $buckeet_type." >&2
        exit 1
    fi

    cleanup_init_terraform
}