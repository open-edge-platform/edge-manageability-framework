# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

cloudProvider: "aws"
awsRegion: {{ .Values.argo.aws.region | quote }}
autoDiscovery:
  clusterName: {{ .Values.argo.clusterName | quote }}
  tags: "eks:cluster-name={{ .Values.argo.clusterName }}"

extraEnv:
  HTTP_PROXY: "{{ .Values.argo.proxy.httpProxy }}"
  HTTPS_PROXY: "{{ .Values.argo.proxy.httpsProxy }}"
  NO_PROXY: "{{ .Values.argo.proxy.noProxy }}"

rbac:
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: "arn:aws:iam::{{ .Values.argo.aws.account }}:role/CASControllerRole-{{ .Values.argo.clusterName }}"
    name: "cluster-autoscaler" # Must match the one we use in pod-config
