# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

sourceSecretName: platform-keycloak-{{.Values.argo.database.type}}-postgresql
targetSecretName: platform-keycloak-{{.Values.argo.database.type}}-postgresql
