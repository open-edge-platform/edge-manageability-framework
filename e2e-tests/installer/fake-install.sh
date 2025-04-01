#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Read the YAML file and iterate over the actions
actions=$(yq e '.actions' auto-install.yaml)

index=0
total=$(echo "$actions" | yq e 'length' -)

while [ $index -lt $total ]; do
    output=$(echo "$actions" | yq e ".[$index].output" -)
    prompt=$(echo "$actions" | yq e ".[$index].prompt" -)

    echo -e "Output: $output"

    # Wait for user input before continuing
    read -p "$prompt "
    echo "You entered: $REPLY"

    index=$((index + 1))
done
