# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

smSecret: {{ .Values.argo.aws.smSecret | default "release-service-token" }}
