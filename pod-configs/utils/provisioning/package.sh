#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Tool to generate a package for SI
set -euo pipefail

CURRENT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "$CURRENT_DIR/../.." && pwd)
VERSION=$(head -n1 "$ROOT_DIR/../VERSION")
MODULES=(
  "buckets" \
  "orchestrator/cluster" \
  "orchestrator/orch-load-balancer" \
  "orchestrator/vpc" \
  "orchestrator/orch-route53" \
  "orchestrator/pull-through-cache-proxy" \
)

if [[ $VERSION == *"-dev" ]]; then
  GIT_HASH=$(git rev-parse --short HEAD)
  VERSION="$VERSION-$GIT_HASH"
fi

if ! which terraform-docs &> /dev/null; then
  echo "Unable to find terraform-doc tool"
  echo "Please follow this link to install it"
  echo "https://terraform-docs.io/user-guide/installation/"
  exit 1
fi

TMP_DIR="$(mktemp -d)"

for module in "${MODULES[@]}"; do
# Ensure the parent dir is present
mkdir -p "$TMP_DIR/pod-configs/$module" && rm -r "$TMP_DIR/pod-configs/$module"
cp -r "$ROOT_DIR/$module" "$TMP_DIR/pod-configs/$module"
done

# Remove all internal deployment configuration and docs
find "$TMP_DIR" -name environments -exec rm -rf {} +
find "$TMP_DIR" -name README.md -exec rm -rf {} +
find "$TMP_DIR" -name "$(cat "$ROOT_DIR/.gitignore")" -exec rm -rf {} +

# Misc files and dependencies
cp -r "$ROOT_DIR/module" "$TMP_DIR/pod-configs/module"
cp -r "$ROOT_DIR/utils" "$TMP_DIR/pod-configs/utils"
cp "$ROOT_DIR/Makefile" "$TMP_DIR/pod-configs/Makefile"
cp "$ROOT_DIR/README.md" "$TMP_DIR/pod-configs/README.md"

# Generate example backend config and variable file for each module
for module in "${MODULES[@]}"; do
  EXAMPLE_CFG_DIR="$TMP_DIR/pod-configs/example-config/$module"
  mkdir -p "$EXAMPLE_CFG_DIR"
  terraform-docs tfvars hcl "$TMP_DIR/pod-configs/$module" > "$EXAMPLE_CFG_DIR/variable.tfvar"
  cat <<-EOF > "$EXAMPLE_CFG_DIR/backend.tf"
region="us-west-2"
bucket="example-bucket"
key="use-west-2/$module/my-env"
EOF
  # Generate Markdown document for user
  terraform-docs markdown "$TMP_DIR/pod-configs/$module" > "$EXAMPLE_CFG_DIR/README.md"
  # Also create "environments" directory under each module
  mkdir -p "$TMP_DIR/pod-configs/$module/environments"
done

# Special case for backend config for buckets
echo 'path="the path to store tfstate"' > "$TMP_DIR/pod-configs/example-config/buckets/backend.tf"

tar -zcf "emf-pod-configs-${VERSION}.tar.gz" -C "$TMP_DIR" --exclude='.terraform' pod-configs
rm -rf "$TMP_DIR"
