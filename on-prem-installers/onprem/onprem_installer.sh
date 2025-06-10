#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Script Name: onprem_installer.sh
# Description: This script:
#               Reads AZURE AD refresh_token credential from user input,
#               Downloads installer and repo artifacts,
#               Set's up OS level dependencies,
#               Installs RKE2 and basic cluster components,
#               Installs ArgoCD
#               Installs Gitea
#               Creates secrets (with user inputs where required)
#               Creates namespaces
#               Installs Edge Orchestrator SW:
#                   Untars and populates Gitea repos with Edge Orchestrator deployment code
#                   Kickstarts deployment via ArgoCD

# Usage: ./onprem_installer
#    -s:             Enables TLS for SRE Exporter. Private TLS CA cert may be provided for SRE destination as an additional argument - provide path to cert (optional)
#    -d:             Disable TLS verification for SMTP endpoint
#    -h:             help (optional)
#    -o:             Override production values with dev values
#    -u:             Set the Release Service URL

set -e
set -o pipefail

# Import shared functions
# shellcheck disable=SC1091
wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
gopath_bin="$(go env GOPATH)/bin"
export PATH="${PATH}:${gopath_bin}:/usr/local/go/bin:/home/ubuntu/.asdf/installs/mage/1.15.0/bin"

cd ../
go install github.com/asdf-vm/asdf/cmd/asdf@v0.17.0
asdf plugin add mage
asdf install mage latest

cd -

mage onPrem:Deploy
