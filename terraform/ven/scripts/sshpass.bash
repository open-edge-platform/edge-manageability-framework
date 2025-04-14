#!/usr/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Define variables
password=$SSH_PASSWORD
command="$@"

# Use sshpass to handle the password prompt and execute the command
sshpass -p "$password" $command
