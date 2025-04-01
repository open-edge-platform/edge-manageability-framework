#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Usage: ./test-presets.sh <directory>

# Check if the directory is provided as an argument
if [ -z "$1" ]; then
  echo "Usage: $0 <directory>"
  exit 1
fi

# Get the specified directory
DIRECTORY=$1

# Ensure the directory exists
if [ ! -d "$DIRECTORY" ]; then
  echo "Error: Directory '$DIRECTORY' does not exist."
  exit 1
fi

# Iterate through all .yaml files in the directory
for FILE in "$DIRECTORY"/*.yaml; do
  # Check if the glob didn't match any files
  [ -e "$FILE" ] || continue

  # Get the relative path of the file
  RELATIVE_PATH=$(realpath --relative-to="$(pwd)" "$FILE")
  
  # Call the mage command with the relative path
  echo "Running: mage -v config:usePreset $RELATIVE_PATH"
  mage -v config:usePreset "$RELATIVE_PATH"
done