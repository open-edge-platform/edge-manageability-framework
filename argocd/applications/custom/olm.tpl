# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

## Custom template for OLM application
## This file provides environment-specific configuration overrides
## for the OLM installation

# OLM configuration - complete structure required for values resolution
olm:
  enabled: true
  namespace: olm
  version: v0.28.0
  
  # Installation method - using official OLM releases
  installMethod: job
  
  # Resource configuration for installer job
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 200m
      memory: 256Mi