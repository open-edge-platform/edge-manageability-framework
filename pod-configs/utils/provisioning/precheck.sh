#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# This script checks the following:
# - OS check, support Linux or OSX only
# - Tools are installed(Terraform, AWS, shuttle, jq)
# - AWS is authorized
CURRENT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "${CURRENT_DIR}/../.." && pwd)
RET_CODE=0

MIN_TF_VERSION="1.5.4"
MIN_AWS_CLI_VERSION="2.9.8"
MIN_PSQL_VERSION="15.4"

log_error() {
  echo -en "\033[0;31m" # Red
  echo -n "$1"
  echo -e "\033[0m" # No Color
}

log_info() {
  echo -en "\033[0;32m" # Green
  echo -n "$1"
  echo -e "\033[0m" # No Color
}

vercomp () {
    if [[ $1 == "$2" ]]
    then
        return 0
    fi
    local IFS=.
    local i ver1=($1) ver2=($2)
    # fill empty fields in ver1 with zeros
    for ((i=${#ver1[@]}; i<${#ver2[@]}; i++))
    do
        ver1[i]=0
    done
    for ((i=0; i<${#ver1[@]}; i++))
    do
        if [[ -z ${ver2[i]} ]]
        then
            # fill empty fields in ver2 with zeros
            ver2[i]=0
        fi
        if ((10#${ver1[i]} > 10#${ver2[i]}))
        then
            return 0
        fi
        if ((10#${ver1[i]} < 10#${ver2[i]}))
        then
            return 1
        fi
    done
    return 0
}

# OS Check
if [[ ! "$OSTYPE" == "darwin"* ]] && [[ ! "$OSTYPE" == "linux"* ]]; then
  log_error "Only support OSX or Linux"
  exit 1
fi

log_info "Checking required tools..."
if ! which terraform &> /dev/null; then
  log_error "Unable to find terraform tool, minimum version: $MIN_TF_VERSION"
  log_error "Follow this guide to install it: https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli"
  RET_CODE=1
else
  TF_VERSION=$(terraform version | sed -nr 's/Terraform v([0-9]\.[0-9]\.[0-9])/\1/p')
  if ! vercomp "$TF_VERSION" "$MIN_TF_VERSION"; then
    log_error "Terraform version is too old"
    log_error "Current: ${TF_VERSION}, minimum: ${MIN_TF_VERSION}"
    RET_CODE=1
  fi
fi

if ! which aws &> /dev/null; then
  log_error "Unable to find AWS CLI"
  log_error "Follow the guide to install it: https://aws.amazon.com/cli/"
  RET_CODE=1
else
  AWS_CLI_VERSION=$(aws --version | sed -nr 's#^aws-cli/([0-9]+\.[0-9]+\.[0-9]+).*$#\1#p')
  if ! vercomp "$AWS_CLI_VERSION" "$MIN_AWS_CLI_VERSION"; then
    log_error "AWS CLI version is too old"
    log_error "Current: ${AWS_CLI_VERSION}, minimum: ${MIN_AWS_CLI_VERSION}"
    RET_CODE=1
  fi
fi

if ! which sshuttle &> /dev/null; then
  log_error "Unable to find sshuttle"
  log_error "Follow this guide to install it: https://github.com/sshuttle/sshuttle#readme"
  RET_CODE=1
fi

if ! which jq &> /dev/null; then
  log_error "Unable to find jq"
  log_error "Follow this guide to install it: https://jqlang.github.io/jq/download/"
  RET_CODE=1
fi

if ! which psql &> /dev/null; then
  log_error "Unable to find psql"
  log_error "Install the postgresql-client package"
  RET_CODE=1
else
  PSQL_VERSION=$(psql --version | sed -nr 's#^psql \(PostgreSQL\) ([0-9]+\.[0-9]+).*$#\1#p')
  if ! vercomp "$PSQL_VERSION" "$MIN_PSQL_VERSION"; then
    log_error "psql version is too old"
    log_error "Current: ${PSQL_VERSION}, minimum: ${MIN_PSQL_VERSION}"
    RET_CODE=1
  fi
fi

# Check if we have sufficient permission to run Terraform on AWS
log_info "Checking AWS credential settings..."
if ! identity=$(aws sts get-caller-identity --no-cli-pager --output json); then
  log_error "failed"
  RET_CODE=1
else
  log_info "ok"
  user_id=$(echo "$identity" | jq .UserId)
  account=$(echo "$identity" | jq .Account)
  arn=$(echo "$identity" | jq .Arn)
  log_info "User: $user_id"
  log_info "Account: $account"
  log_info "Role: $arn"
fi

log_info ""
if [ $RET_CODE == 0 ]; then
  log_info "Ready to provision the infrastructure"
else
  log_error "Please fix error(s) before provision the infrastructure"
  exit 1
fi
