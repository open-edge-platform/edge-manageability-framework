#!/usr/bin/env bash

# This script is used to upgrade the version of an EKS cluster. Refer to the 'usage_upgrade()' function for more details.
set -ue
set -o pipefail

. utils/lib/common.sh



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

# if [[ ! -f ${SAVE_DIR}/${VALUES} ]]; then
#     echo -n "Error: There is no value file found."
#     exit 1
# fi


# Upgrade

# load_values

# Terminate existing sshuttle
refresh_sshuttle
connect_cluster


 #terraform state rm module.kms.kubernetes_secret.vault_kms_unseal 2>/dev/null || true

##action_route53_orch_wi_lb apply
# update_eks

# Terminate existing sshuttle
terminate_sshuttle

echo "Info: Upgrade $VERSION completed successfully."