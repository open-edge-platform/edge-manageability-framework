#!/usr/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Check if sshpass is installed
if ! command -v sshpass &> /dev/null
then
    echo "sshpass could not be found, please install it."
    exit 1
fi

# Define variables
password=$SSH_PASSWORD
command="$@"

# Use sshpass to handle the password prompt and execute the command
sshpass -p "$password" $command
