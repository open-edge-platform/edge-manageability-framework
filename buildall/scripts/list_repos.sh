#!/usr/bin/env bash

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

# list_repos.sh
# lists all repos within an organizaiton

set -eu -o pipefail

if [[ $# != 1 ]]; then
  echo "Usage: $0 <name of github org>"
fi

# bring in environmental variables
source env.sh

GITHUB_ORG="$1"

GITHUB_USER="${GITHUB_USER:-user}"
GITHUB_TOKEN="${GITHUB_TOKEN:-ghp_invalid}"

# FIXME: The following CURL only works if there are fewer than 100 repos in the org
PAGE=1

curl -u "$GITHUB_USER:$GITHUB_TOKEN" \
  "https://api.github.com/orgs/${GITHUB_ORG}/repos?page=${PAGE}&per_page=100" |\
  jq -r '.[] | .name' | sort
