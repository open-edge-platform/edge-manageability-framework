# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Frontend configuration (Grafana UI)
grafana:
  imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
  {{- toYaml . | nindent 4 }}
  {{- end }}
  grafana_proxy:
    registry: {{ .Values.argo.containerRegistryURL }}
  {{- if and .Values.argo.o11y .Values.argo.o11y.orchestrator .Values.argo.o11y.orchestrator.grafana }}
  {{- if .Values.argo.o11y.orchestrator.grafana.resources }}
  resources:
    {{- toYaml .Values.argo.o11y.orchestrator.grafana.resources | nindent 4 }}
  {{- end }}
  {{- if and .Values.argo.o11y.orchestrator.grafana.sidecar .Values.argo.o11y.orchestrator.grafana.sidecar.resources }}
  sidecar:
    resources:
      {{- toYaml .Values.argo.o11y.orchestrator.grafana.sidecar.resources | nindent 6 }}
  {{- end }}
  {{- end }}
  grafana.ini:
    server:
      root_url: https://observability-admin.{{ .Values.argo.clusterDomain }}
    auth:
      signout_redirect_url: https://keycloak.{{ .Values.argo.clusterDomain }}/realms/master/protocol/openid-connect/logout?redirect_uri=https%3A%2F%2Fweb-ui.{{ .Values.argo.clusterDomain }}
    auth.generic_oauth:
      auth_url: https://keycloak.{{ .Values.argo.clusterDomain }}/realms/master/protocol/openid-connect/auth

# Logging configuration (Grafana Loki)
loki:
  {{ if ((.Values.argo.aws).efs) }}
  {{- if .Values.argo.aws.account }}
  serviceAccount:
    name: aws-s3-sa-loki
    annotations:
      eks.amazonaws.com/role-arn: "arn:aws:iam::{{ .Values.argo.aws.account }}:role/{{ .Values.argo.clusterName }}-s3-role"
  {{- end }}
  {{- end }}
  {{- if and .Values.argo.o11y .Values.argo.o11y.orchestrator .Values.argo.o11y.orchestrator.loki .Values.argo.o11y.orchestrator.loki.backend .Values.argo.o11y.orchestrator.loki.backend.resources }}
  backend:
    resources:
      {{- toYaml .Values.argo.o11y.orchestrator.loki.backend.resources | nindent 6 }}
  {{- end }}
  {{- if .Values.argo.o11y }}
  {{- with .Values.argo.o11y.orchestrator }}
  {{- if or (and .loki .loki.write) (and .commonConfig .commonConfig.storageClass) }}
  write:
    {{- if or (and .loki .loki.write .loki.write.volumeSize) (and .commonConfig .commonConfig.storageClass) }}
    persistence:
      {{- if and .loki .loki.write .loki.write.volumeSize }}
      size: {{ .loki.write.volumeSize }}
      {{- end }}
      {{- if and .commonConfig .commonConfig.storageClass }}
      storageClass: {{ .commonConfig.storageClass }}
      {{- end }}
    {{- end }}
    {{- if and .loki .loki.write .loki.write.resources }}
    resources:
      {{- toYaml .loki.write.resources | nindent 6 }}
    {{- end }}
    {{- if and .loki .loki.write .loki.write.replicas }}
    replicas: {{ .loki.write.replicas }}
    {{- end }}
  {{- end }}
  {{- if or (and .loki .loki.read) (and .commonConfig .commonConfig.storageClass) }}
  read:
    {{- if or (and .loki .loki.read .loki.read.volumeSize) (and .commonConfig .commonConfig.storageClass) }}
    persistence:
      {{- if and .loki .loki.read .loki.read.volumeSize }}
      size: {{ .loki.read.volumeSize }}
      {{- end }}
      {{- if and .commonConfig .commonConfig.storageClass }}
      storageClass: {{ .commonConfig.storageClass }}
      {{- end }}
    {{- end }}
    {{- if and .loki .loki.read .loki.read.resources }}
    resources:
      {{- toYaml .loki.read.resources | nindent 6 }}
    {{- end }}
    {{- if and .loki .loki.read .loki.read.replicas }}
    replicas: {{ .loki.read.replicas }}
    {{- end }}
  {{- end }}
  {{- if and .loki .loki.gateway .loki.gateway.resources }}
  gateway:
    resources:
      {{- toYaml .loki.gateway.resources | nindent 6 }}
  {{- end }}
  {{- if and .loki .loki.memcachedExporter .loki.memcachedExporter.resources }}
  memcachedExporter:
    resources:
      {{- toYaml .loki.memcachedExporter.resources | nindent 6 }}
  {{- end }}
  {{- if and .loki .loki.chunksCache .loki.chunksCache.resources }}
  chunksCache:
    resources:
      {{- toYaml .loki.chunksCache.resources | nindent 6 }}
  {{- end }}
  {{- if and .loki .loki.resultsCache .loki.resultsCache.resources }}
  resultsCache:
    resources:
      {{- toYaml .loki.resultsCache.resources | nindent 6 }}
  {{- end }}
  {{- end }}
  {{- /* end of with .Values.argo.o11y.orchestrator */}}
  {{- end }}
  loki:
    {{- if and .Values.argo.o11y .Values.argo.o11y.orchestrator .Values.argo.o11y.orchestrator.loki .Values.argo.o11y.orchestrator.loki.replicationFactor }}
    commonConfig:
      replication_factor: {{ .Values.argo.o11y.orchestrator.loki.replicationFactor }}
    {{- end }}
    compactor:
      retention_enabled: true
    {{- if and .Values.argo.o11y .Values.argo.o11y.orchestrator .Values.argo.o11y.orchestrator.loki .Values.argo.o11y.orchestrator.loki.logRetentionPeriod }}
    limits_config:
      retention_period: {{ .Values.argo.o11y.orchestrator.loki.logRetentionPeriod }}
    {{- end }}
  {{ if ((.Values.argo.aws).efs) }}
    {{- if .Values.argo.aws.account }}
    storage:
      bucketNames:
        chunks: {{ .Values.argo.aws.bucketPrefix | default .Values.argo.clusterName }}-orch-loki-chunks
        ruler: {{ .Values.argo.aws.bucketPrefix | default .Values.argo.clusterName }}-orch-loki-ruler
        admin: {{ .Values.argo.aws.bucketPrefix | default .Values.argo.clusterName }}-orch-loki-admin
      type: s3
      s3:
        s3: s3://{{ .Values.argo.aws.region }}
        region: {{ .Values.argo.aws.region }}
        s3ForcePathStyle: false
        insecure: false
        http_config: {}
        # Need to unset the following values when using IRSA: https://github.com/grafana/loki/issues/8152
        # Use empty string instead of null since we are facing this issue: https://github.com/rancher/fleet/issues/264
        endpoint: ""
        secretAccessKey: ""
        accessKeyId: ""
    {{- end }}
  {{- end }}
  {{- with .Values.argo.o11y.orchestrator.loki.sidecar.resources }}
  sidecar:
    resources:
      {{- toYaml . | nindent 6 }}
  {{- end }}

# Metrics configuration (Grafana Mimir)
mimir-distributed:
  {{ if ((.Values.argo.aws).efs) }}
  {{- if .Values.argo.aws.account }}
  serviceAccount:
    name: aws-s3-sa-mimir
    annotations:
      eks.amazonaws.com/role-arn: "arn:aws:iam::{{ .Values.argo.aws.account }}:role/{{ .Values.argo.clusterName }}-s3-role"
  minio:
    enabled: false
  {{- else }}

  minio:
    {{- if and .Values.argo.o11y.orchestrator .Values.argo.o11y.orchestrator.mimir .Values.argo.o11y.orchestrator.mimir.compactor .Values.argo.o11y.orchestrator.mimir.compactor.volumeSize }}
    persistence:
      size: {{ .Values.argo.o11y.orchestrator.mimir.compactor.volumeSize }}
    {{- end }}
    {{- if .Values.argo.o11y.orchestrator.mimir.minio.resources }}
    resources:
      {{- toYaml .Values.argo.o11y.orchestrator.mimir.minio.resources | nindent 6 }}
    {{- else }}
    resources: null
    {{- end }}
  {{- end }}
  {{- else }}
  minio:
    {{- if and .Values.argo.o11y .Values.argo.o11y.orchestrator .Values.argo.o11y.orchestrator.mimir .Values.argo.o11y.orchestrator.mimir.compactor .Values.argo.o11y.orchestrator.mimir.compactor.volumeSize }}
    persistence:
      size: {{ .Values.argo.o11y.orchestrator.mimir.compactor.volumeSize }}
    {{- end }}
    {{- if .Values.argo.o11y.orchestrator.mimir.minio.resources }}
    resources:
      {{- toYaml .Values.argo.o11y.orchestrator.mimir.minio.resources | nindent 6 }}
    {{- else }}
    resources: null
    {{- end }}
  {{- end }}
  mimir:
    # Need to set a few values to null when using IRSA
    structuredConfig:
    {{- if and .Values.argo.o11y .Values.argo.o11y.orchestrator .Values.argo.o11y.orchestrator.mimir .Values.argo.o11y.orchestrator.mimir.structuredConfig }}
      {{- if .Values.argo.o11y.orchestrator.mimir.structuredConfig.querySchedulerMaxOutstandingRequestsPerTenant }}
      query_scheduler:
        max_outstanding_requests_per_tenant: {{ .Values.argo.o11y.orchestrator.mimir.structuredConfig.querySchedulerMaxOutstandingRequestsPerTenant }}
      {{- end }}
      {{- if .Values.argo.o11y.orchestrator.mimir.structuredConfig.frontendMaxOutstandingRequestsPerTenant }}
      frontend:
        max_outstanding_per_tenant: {{ .Values.argo.o11y.orchestrator.mimir.structuredConfig.frontendMaxOutstandingRequestsPerTenant }}
      {{- end }}
      {{- if .Values.argo.o11y.orchestrator.mimir.structuredConfig.querierTime }}
      querier:
        query_store_after: {{ .Values.argo.o11y.orchestrator.mimir.structuredConfig.querierTime }}
      {{- end }}
      limits:
      {{- if .Values.argo.o11y.orchestrator.mimir.structuredConfig.metricsRetentionPeriod }}
        compactor_blocks_retention_period: {{ .Values.argo.o11y.orchestrator.mimir.structuredConfig.metricsRetentionPeriod }}
      {{- end }}
      {{- if .Values.argo.o11y.orchestrator.mimir.structuredConfig.ingestionRate }}
        ingestion_rate: {{ .Values.argo.o11y.orchestrator.mimir.structuredConfig.ingestionRate }}
      {{- end }}
      {{- if .Values.argo.o11y.orchestrator.mimir.structuredConfig.ingestionBurstSize }}
        ingestion_burst_size: {{ .Values.argo.o11y.orchestrator.mimir.structuredConfig.ingestionBurstSize }}
      {{- end }}
      {{- if .Values.argo.o11y.orchestrator.mimir.structuredConfig.maxGlobalSeriesPerUser }}
        max_global_series_per_user: {{ .Values.argo.o11y.orchestrator.mimir.structuredConfig.maxGlobalSeriesPerUser }}
      {{- end }}
      {{- if .Values.argo.o11y.orchestrator.mimir.structuredConfig.maxLabelNamesPerSeries }}
        max_label_names_per_series: {{ .Values.argo.o11y.orchestrator.mimir.structuredConfig.maxLabelNamesPerSeries }}
      {{- end }}
    {{- end }}
      blocks_storage:
        tsdb:
          wal_compression_enabled: true
        backend: s3
      {{ if ((.Values.argo.aws).efs) }}
        {{- if .Values.argo.aws.account }}
        s3:
          bucket_name: {{ .Values.argo.aws.bucketPrefix | default .Values.argo.clusterName }}-orch-mimir-tsdb
          endpoint: s3.{{ .Values.argo.aws.region }}.amazonaws.com
          region: {{ .Values.argo.aws.region }}
          insecure: false
          secret_access_key: ""
          access_key_id: ""
        {{- end }}
      {{- end }}
      ruler_storage:
        backend: s3
      {{ if ((.Values.argo.aws).efs) }}
        {{- if .Values.argo.aws.account }}
        s3:
          bucket_name: {{ .Values.argo.aws.bucketPrefix | default .Values.argo.clusterName }}-orch-mimir-ruler
          endpoint: s3.{{ .Values.argo.aws.region }}.amazonaws.com
          region: {{ .Values.argo.aws.region }}
          insecure: false
          secret_access_key: ""
          access_key_id: ""
        {{- end }}
      {{- end }}
  gateway:
    ingress:
      enabled: false
    {{- if and .Values.argo.o11y .Values.argo.o11y.orchestrator .Values.argo.o11y.orchestrator.mimir }}
    {{- with .Values.argo.o11y.orchestrator.mimir }}
    {{- if .replicas }}
    replicas: {{ .replicas }}
    {{- end }}
    {{- if .resources }}
    resources:
      {{- toYaml .resources | nindent 6 }}
    {{- end }}
    {{- end }}
    {{- end }}
  {{- if .Values.argo.o11y }}
  {{- with .Values.argo.o11y.orchestrator }}
  {{- if or (and .mimir (or .mimir.storeGateway .mimir.replicationFactor)) (and .commonConfig .commonConfig.storageClass) }}
  store_gateway: # Note: naming conversion to snake_case is intentional
    {{- if or (and .mimir .mimir.storeGateway .mimir.storeGateway.volumeSize) (and .commonConfig .commonConfig.storageClass) }}
    persistentVolume:
      {{- if and .mimir .mimir.storeGateway .mimir.storeGateway.volumeSize }}
      size: {{ .mimir.storeGateway.volumeSize }}
      {{- end }}
      {{- if and .commonConfig .commonConfig.storageClass }}
      storageClass: {{ .commonConfig.storageClass }}
      {{- end }}
    {{- end }}
    {{- with .mimir }}
    {{- if .replicationFactor }}
    sharding_ring:
      replication_factor: {{ .replicationFactor }}
    {{- end }}
    {{- with .storeGateway }}
    {{- if .replicas }}
    replicas: {{ .replicas }}
    {{- end }}
    {{- if .resources }}
    resources:
      {{- toYaml .resources | nindent 6 }}
    {{- end }}
    {{- end }}
    {{- end }}
  {{- end }}
  {{- if or (and .mimir (or .mimir.ingester .mimir.replicationFactor)) (and .commonConfig .commonConfig.storageClass) }}
  ingester:
    {{- if or (and .mimir .mimir.ingester .mimir.ingester.volumeSize) (and .commonConfig .commonConfig.storageClass) }}
    persistentVolume:
      {{- if and .mimir .mimir.ingester .mimir.ingester.volumeSize }}
      size: {{ .mimir.ingester.volumeSize }}
      {{- end }}
      {{- if and .commonConfig .commonConfig.storageClass }}
      storageClass: {{ .commonConfig.storageClass }}
      {{- end }}
    {{- end }}
    {{- with .mimir }}
    {{- if .replicationFactor }}
    ring:
      replication_factor: {{ .replicationFactor }}
    {{- end }}
    {{- with .ingester }}
    {{- if .replicas }}
    replicas: {{ .replicas }}
    {{- end }}
    {{- if .resources }}
    resources:
      {{- toYaml .resources | nindent 6 }}
    {{- end }}
    {{- end }}
    {{- end }}
  {{- end }}
  {{- if and .mimir .mimir.distributor }}
  distributor:
    {{- if .mimir.distributor.replicas }}
    replicas: {{ .mimir.distributor.replicas }}
    {{- end }}
    {{- if .mimir.distributor.resources }}
    resources:
      {{- toYaml .mimir.distributor.resources | nindent 6 }}
    {{- end }}
  {{- end }}
  {{- if or (and .mimir .mimir.compactor) (and .commonConfig .commonConfig.storageClass) }}
  compactor:
    {{- if or (and .mimir .mimir.compactor .mimir.compactor.volumeSize) (and .commonConfig .commonConfig.storageClass) }}
    persistentVolume:
      {{- if and .mimir .mimir.compactor .mimir.compactor.volumeSize }}
      size: {{ .mimir.compactor.volumeSize }}
      {{- end }}
      {{- if and .commonConfig .commonConfig.storageClass }}
      storageClass: {{ .commonConfig.storageClass }}
      {{- end }}
    {{- end }}
    {{- if and .mimir .mimir.compactor .mimir.compactor.resources }}
    resources:
      {{- toYaml .mimir.compactor.resources | nindent 6 }}
    {{- end }}
  {{- end }}
  {{- end }}
  {{- /* end of with .Values.argo.o11y.orchestrator */}}
  {{- end }}

  {{- with .Values.argo.o11y.orchestrator.mimir.querier.resources }}
  querier:
    resources:
      {{- toYaml . | nindent 6 }}
  {{- end }}

  {{- with .Values.argo.o11y.orchestrator.mimir.query_frontend.resources }}
  query_frontend:
    resources:
      {{- toYaml . | nindent 6 }}
  {{- end }}
  {{- with .Values.argo.o11y.orchestrator.mimir.ruler.resources }}
  ruler:
    resources:
      {{- toYaml . | nindent 6 }}
  {{- end }}

# OpenTelemetry additional pipelines config

opentelemetry-collector-daemonset:
  {{- if and .Values.argo.o11y .Values.argo.o11y.dedicatedEdgenodeEnabled }}
  tolerations:
  - key: "node.kubernetes.io/custom-rule"
    operator: "Equal"
    value: "observability"
    effect: "NoSchedule"
  {{- end }}
  {{- if .Values.argo.o11y.orchestrator.opentelemetryCollectorDaemonset.resources }}
  resources:
    {{- toYaml .Values.argo.o11y.orchestrator.opentelemetryCollectorDaemonset.resources | nindent 4 }}
  {{- else }}
  resources: null
  {{- end }}

{{- with .Values.argo.o11y.orchestrator.opentelemetryCollector.resources }}
opentelemetry-collector:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}

# Tracing configuration (Grafana Tempo)
{{- if and .Values.argo.o11y .Values.argo.o11y.orchestrator .Values.argo.o11y.orchestrator.tempo }}
{{- if eq (index .Values.argo.o11y.orchestrator.tempo "enabled") true }}
import:
  tempo:
    enabled: true

tempo:
  storage:
    traces:
      backend: s3
      {{ if and ((.Values.argo.aws).efs) .Values.argo.aws.account }}
      s3:
        bucket: {{ .Values.argo.aws.bucketPrefix | default .Values.argo.clusterName }}-tempo-traces
        endpoint: s3.{{ .Values.argo.aws.region }}.amazonaws.com
        region: {{ .Values.argo.aws.region }}
        insecure: false
        secret_access_key: ""
        access_key_id: ""
      {{- end }}
{{- end }}
{{- end }}
