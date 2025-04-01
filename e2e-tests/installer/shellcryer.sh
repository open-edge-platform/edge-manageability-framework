#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

sleep_time=${1:-60}

# Function to print the timestamp
print_timestamp() {
    echo -e "\n\n**** $(date) ****\n\n"
}

# Run the function in the background every 60 seconds
while true; do
    print_timestamp
    sleep $sleep_time
done
