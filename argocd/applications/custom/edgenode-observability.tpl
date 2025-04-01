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
    {{- with .Values.argo.o11y.edgeNode.grafana.grafana_proxy.resources }}
    resources:
      {{- toYaml . | nindent 6 }}
    {{- end }}
  {{- if and .Values.argo.o11y .Values.argo.o11y.dedicatedEdgenodeEnabled }}
  nodeSelector:
    node.kubernetes.io/custom-rule: observability
  tolerations:
  - key: "node.kubernetes.io/custom-rule"
    operator: "Equal"
    value: "observability"
    effect: "NoSchedule"
  {{- end }}
  {{- if and .Values.argo.o11y .Values.argo.o11y.edgeNode .Values.argo.o11y.edgeNode.grafana }}
  {{- if .Values.argo.o11y.edgeNode.grafana.resources }}
  resources:
    {{- toYaml .Values.argo.o11y.edgeNode.grafana.resources | nindent 4 }}
  {{- end }}
  {{- if and .Values.argo.o11y.edgeNode.grafana.sidecar .Values.argo.o11y.edgeNode.grafana.sidecar.resources }}
  sidecar:
    resources:
      {{- toYaml .Values.argo.o11y.edgeNode.grafana.sidecar.resources | nindent 6 }}
  {{- end }}
  {{- end }}
  grafana.ini:
    server:
      root_url: https://observability-ui.{{ .Values.argo.clusterDomain }}
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
  backend:
  {{- if and .Values.argo.o11y .Values.argo.o11y.dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
  {{- end }}
  {{- if and .Values.argo.o11y .Values.argo.o11y.edgeNode .Values.argo.o11y.edgeNode.loki .Values.argo.o11y.edgeNode.loki.backend .Values.argo.o11y.edgeNode.loki.backend.resources }}
    resources:
      {{- toYaml .Values.argo.o11y.edgeNode.loki.backend.resources | nindent 6 }}
  {{- else }}
    resources: null
  {{- end }}
  {{- with .Values.argo.o11y }}
  {{- if or .dedicatedEdgenodeEnabled (and .edgeNode .edgeNode.loki .edgeNode.loki.write) (and .edgeNode .edgeNode.commonConfig .edgeNode.commonConfig.storageClass) }}
  write:
    {{- if .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    {{- with .edgeNode }}
    {{- if or (and .loki .loki.write .loki.write.volumeSize) (and .commonConfig .commonConfig.storageClass) }}
    persistence:
      {{- if and .loki .loki.write .loki.write.volumeSize }}
      size: {{ .loki.write.volumeSize }}
      {{- end }}
      {{- if and .commonConfig .commonConfig.storageClass }}
      storageClass: {{ .commonConfig.storageClass }}
      {{- end }}
    {{- end }}
    {{- if and .loki .loki.write }}
    {{- with .loki.write.resources }}
    resources:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- if .loki.write.replicas }}
    replicas: {{ .loki.write.replicas }}
    {{- end }}
    {{- end }}
    {{- end }}
  {{- end }}
  {{- if or .dedicatedEdgenodeEnabled (and .edgeNode .edgeNode.loki .edgeNode.loki.read) (and .edgeNode .edgeNode.commonConfig .edgeNode.commonConfig.storageClass) }}
  read:
    {{- if .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    {{- with .edgeNode }}
    {{- if or (and .loki .loki.read .loki.read.volumeSize) (and .commonConfig .commonConfig.storageClass) }}
    persistence:
      {{- if and .loki .loki.read .loki.read.volumeSize }}
      size: {{ .loki.read.volumeSize }}
      {{- end }}
      {{- if and .commonConfig .commonConfig.storageClass }}
      storageClass: {{ .commonConfig.storageClass }}
      {{- end }}
    {{- end }}
    {{- if and .loki .loki.read }}
    {{- if .loki.read.resources }}
    resources:
      {{- toYaml .loki.read.resources | nindent 6 }}
    {{- end }}
    {{- if .loki.read.replicas }}
    replicas: {{ .loki.read.replicas }}
    {{- end }}
    {{- end }}
    {{- end }}
  {{- end }}
  {{- if or .dedicatedEdgenodeEnabled (and .edgeNode .edgeNode.loki .edgeNode.loki.gateway) }}
  gateway:
    {{- if .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    {{- if and .edgeNode .edgeNode.loki .edgeNode.loki.gateway }}
    {{- if .edgeNode.loki.gateway.resources }}
    resources:
      {{- toYaml .edgeNode.loki.gateway.resources | nindent 6 }}
    {{- end }}
    {{- if .edgeNode.loki.gateway.replicas }}
    replicas: {{ .edgeNode.loki.gateway.replicas }}
    {{- end }}
    {{- end }}
  {{- end }}
  {{- if or .dedicatedEdgenodeEnabled (and .edgeNode .edgeNode.loki .edgeNode.loki.chunksCache) }}
  chunksCache:
    {{- if .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    {{- if and .edgeNode .edgeNode.loki .edgeNode.loki.chunksCache }}
    {{- if .edgeNode.loki.chunksCache.timeout }}
    timeout: {{ .edgeNode.loki.chunksCache.timeout | quote }}
    {{- end }}
    {{- if .edgeNode.loki.chunksCache.writebackBuffer }}
    writebackBuffer: {{ .edgeNode.loki.chunksCache.writebackBuffer }}
    {{- end }}
    {{- if .edgeNode.loki.chunksCache.writebackParallelism }}
    writebackParallelism: {{ .edgeNode.loki.chunksCache.writebackParallelism }}
    {{- end }}
    {{- if .edgeNode.loki.chunksCache.writebackSizeLimit }}
    writebackSizeLimit: {{ .edgeNode.loki.chunksCache.writebackSizeLimit | quote }}
    {{- end }}
    {{- if and .edgeNode .edgeNode.loki .edgeNode.loki.chunksCache .edgeNode.loki.chunksCache.resources }}
    resources:
      {{- toYaml .edgeNode.loki.chunksCache.resources | nindent 6 }}
    {{- end }}
  {{- end }}
  {{- end }}
  {{- if or .dedicatedEdgenodeEnabled (and .edgeNode .edgeNode.loki .edgeNode.loki.resultsCache) }}
  resultsCache:
    {{- if .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    {{- if and .edgeNode .edgeNode.loki .edgeNode.loki.resultsCache }}
    {{- if .edgeNode.loki.resultsCache.timeout }}
    timeout: {{ .edgeNode.loki.resultsCache.timeout | quote }}
    {{- end }}
    {{- if .edgeNode.loki.resultsCache.writebackBuffer }}
    writebackBuffer: {{ .edgeNode.loki.resultsCache.writebackBuffer }}
    {{- end }}
    {{- if .edgeNode.loki.resultsCache.writebackParallelism }}
    writebackParallelism: {{ .edgeNode.loki.resultsCache.writebackParallelism }}
    {{- end }}
    {{- if .edgeNode.loki.resultsCache.writebackSizeLimit }}
    writebackSizeLimit: {{ .edgeNode.loki.resultsCache.writebackSizeLimit | quote }}
    {{- end }}
    {{- end }}
    {{- if and .edgeNode .edgeNode.loki .edgeNode.loki.resultsCache .edgeNode.loki.resultsCache.resources }}
    resources:
      {{- toYaml .edgeNode.loki.resultsCache.resources | nindent 6 }}
    {{- end }}
  {{- if and .edgeNode .edgeNode.loki .edgeNode.loki.memcachedExporter .edgeNode.loki.memcachedExporter.resources }}
  memcachedExporter:
    resources:
      {{- toYaml .edgeNode.loki.memcachedExporter.resources | nindent 6 }}
  {{- end }}
  {{- end }}
  loki:
    {{- if and .edgeNode .edgeNode.loki .edgeNode.loki.replicationFactor }}
    commonConfig:
      replication_factor: {{ .edgeNode.loki.replicationFactor }}
    {{- end }}
    compactor:
      retention_enabled: true
      {{- if and .edgeNode .edgeNode.loki .edgeNode.loki.compactor .edgeNode.loki.compactor.compactionInterval }}
      compaction_interval: {{ .edgeNode.loki.compactor.compactionInterval }}
      {{- end }}
    {{- if and .edgeNode .edgeNode.loki .edgeNode.loki.ingester }}
    ingester:
      {{- if .edgeNode.loki.ingester.chunkEncoding }}
      chunk_encoding: {{ .edgeNode.loki.ingester.chunkEncoding | quote }}
      {{- end }}
      {{- if .edgeNode.loki.ingester.chunkIdlePeriod }}
      chunk_idle_period: {{ .edgeNode.loki.ingester.chunkIdlePeriod | quote }}
      {{- end }}
      {{- if .edgeNode.loki.ingester.chunkTargetSize }}
      chunk_target_size: {{ .edgeNode.loki.ingester.chunkTargetSize }}
      {{- end }}
      {{- if .edgeNode.loki.ingester.maxChunkAge }}
      max_chunk_age: {{ .edgeNode.loki.ingester.maxChunkAge | quote }}
      {{- end }}
    {{- end }}
    {{- if and .edgeNode .edgeNode.loki .edgeNode.loki.ingesterClient }}
    ingester_client:
      grpc_client_config:
        {{- if .edgeNode.loki.ingesterClient.maxRecvMsgSize }}
        max_recv_msg_size: {{ .edgeNode.loki.ingesterClient.maxRecvMsgSize }}
        {{- end }}
        {{- if .edgeNode.loki.ingesterClient.maxSendMsgSize }}
        max_send_msg_size: {{ .edgeNode.loki.ingesterClient.maxSendMsgSize }}
        {{- end }}
    {{- end }}
    {{- if and .edgeNode .edgeNode.loki .edgeNode.loki.server }}
    server:
      {{- if .edgeNode.loki.server.maxRecvMsgSize }}
      grpc_server_max_recv_msg_size: {{ .edgeNode.loki.server.maxRecvMsgSize }}
      {{- end }}
      {{- if .edgeNode.loki.server.maxSendMsgSize }}
      grpc_server_max_send_msg_size: {{ .edgeNode.loki.server.maxSendMsgSize }}
      {{- end }}
    {{- end }}
    {{- if and .edgeNode .edgeNode.loki .edgeNode.loki.querier }}
    querier:
      {{- if .edgeNode.loki.querier.maxConcurrent }}
      max_concurrent: {{ .edgeNode.loki.querier.maxConcurrent }}
      {{- end }}
    {{- end }}
    {{- if and .edgeNode .edgeNode.loki }}
    {{- if or .edgeNode.loki.logRetentionPeriod .edgeNode.loki.limitsConfig }}
    limits_config:
      {{- with .edgeNode.loki.limitsConfig }}
      {{- if .cardinalityLimit }}
      cardinality_limit: {{ .cardinalityLimit }}
      {{- end }}
      {{- if .ingestionBurstSizeMb }}
      ingestion_burst_size_mb: {{ .ingestionBurstSizeMb }}
      {{- end }}
      {{- if .ingestionRateMb }}
      ingestion_rate_mb: {{ .ingestionRateMb }}
      {{- end }}
      {{- if .maxEntriesLimitPerQuery }}
      max_entries_limit_per_query: {{ .maxEntriesLimitPerQuery }}
      {{- end }}
      {{- if .maxGlobalStreamsPerUser }}
      max_global_streams_per_user: {{ .maxGlobalStreamsPerUser }}
      {{- end }}
      {{- if .maxLabelNameLength }}
      max_label_name_length: {{ .maxLabelNameLength }}
      {{- end }}
      {{- if .maxLabelNamesPerSeries }}
      max_label_names_per_series: {{ .maxLabelNamesPerSeries }}
      {{- end }}
      {{- if .maxLabelValueLength }}
      max_label_value_length: {{ .maxLabelValueLength }}
      {{- end }}
      {{- if .maxLineSize }}
      max_line_size: {{ .maxLineSize | quote }}
      {{- end }}
      {{- if .maxQueryParallelism }}
      max_query_parallelism: {{ .maxQueryParallelism }}
      {{- end }}
      {{- if .perStreamRateLimit }}
      per_stream_rate_limit: {{ .perStreamRateLimit | quote }}
      {{- end }}
      {{- if .perStreamRateLimitBurst }}
      per_stream_rate_limit_burst: {{ .perStreamRateLimitBurst | quote }}
      {{- end }}
      {{- if .rejectOldSamples }}
      reject_old_samples: {{ .rejectOldSamples }}
      {{- end }}
      {{- if .rejectOldSamplesMaxAge }}
      reject_old_samples_max_age: {{ .rejectOldSamplesMaxAge | quote }}
      {{- end }}
      {{- end }}
      {{- if .edgeNode.loki.logRetentionPeriod }}
      retention_period: {{ .edgeNode.loki.logRetentionPeriod | quote }}
      {{- end }}
      {{- if .edgeNode.loki.provisioningLogRetentionPeriod }}
      retention_stream:
      - selector: '{source="edgenode_provisioning"}'
        priority: 1
        period: {{ .edgeNode.loki.provisioningLogRetentionPeriod | quote }}
      {{- end }}
    {{- end }}
    {{- end }}
  {{- end }}
  {{- /* end of with .Values.argo.o11y */}}
  {{ if ((.Values.argo.aws).efs) }}
    {{- if .Values.argo.aws.account }}
    storage:
      bucketNames:
        chunks: {{ .Values.argo.aws.bucketPrefix | default .Values.argo.clusterName }}-fm-loki-chunks
        ruler: {{ .Values.argo.aws.bucketPrefix | default .Values.argo.clusterName }}-fm-loki-ruler
        admin: {{ .Values.argo.aws.bucketPrefix | default .Values.argo.clusterName }}-fm-loki-admin
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
  {{- with .Values.argo.o11y.edgeNode.loki.sidecar.resources }}
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
  {{- end }}
{{- end }}
{{ if ((.Values.argo.aws).efs) }}
  {{- if .Values.argo.aws.account }}
  minio:
    enabled: false
  {{- else }}
  {{- if and .Values.argo.o11y .Values.argo.o11y.edgeNode .Values.argo.o11y.edgeNode.mimir .Values.argo.o11y.edgeNode.mimir.compactor .Values.argo.o11y.edgeNode.mimir.compactor.volumeSize }}
  minio:
    persistence:
      size: {{ .Values.argo.o11y.edgeNode.mimir.compactor.volumeSize }}
    {{- with .Values.argo.o11y.edgeNode.mimir.minio.resources }}
    resources:
      {{- toYaml . | nindent 6 }}
    {{- end }}
  {{- end }}
  {{- end }}
{{- else }}
  {{ if and .Values.argo.o11y .Values.argo.o11y.edgeNode .Values.argo.o11y.edgeNode.mimir .Values.argo.o11y.edgeNode.mimir.compactor .Values.argo.o11y.edgeNode.mimir.compactor.volumeSize }}
  minio:
    persistence:
      size: {{ .Values.argo.o11y.edgeNode.mimir.compactor.volumeSize }}
    {{- with .Values.argo.o11y.edgeNode.mimir.minio.resources }}
    resources:
      {{- toYaml . | nindent 6 }}
    {{- end }}
  {{- else }}
  minio:
    {{- if .Values.argo.o11y.edgeNode.mimir.minio.resources }}
    resources:
      {{- toYaml .Values.argo.o11y.edgeNode.mimir.minio.resources | nindent 6 }}
    {{- else }}
    resources: null
    {{- end }}
  {{- end }}
{{- end }}
  mimir:
    {{- if and .Values.argo.o11y .Values.argo.o11y.dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    # Need to set a few values to null when using IRSA
    structuredConfig:
    {{- if and .Values.argo.o11y .Values.argo.o11y.edgeNode .Values.argo.o11y.edgeNode.mimir .Values.argo.o11y.edgeNode.mimir.structuredConfig }}
      {{- if .Values.argo.o11y.edgeNode.mimir.structuredConfig.querySchedulerMaxOutstandingRequestsPerTenant }}
      query_scheduler:
        max_outstanding_requests_per_tenant: {{ .Values.argo.o11y.edgeNode.mimir.structuredConfig.querySchedulerMaxOutstandingRequestsPerTenant }}
      {{- end }}
      {{- if .Values.argo.o11y.edgeNode.mimir.structuredConfig.frontendMaxOutstandingRequestsPerTenant }}
      frontend:
        max_outstanding_per_tenant: {{ .Values.argo.o11y.edgeNode.mimir.structuredConfig.frontendMaxOutstandingRequestsPerTenant }}
      {{- end }}
      {{- if .Values.argo.o11y.edgeNode.mimir.structuredConfig.querierTime }}
      querier:
        query_store_after: {{ .Values.argo.o11y.edgeNode.mimir.structuredConfig.querierTime }}
      {{- end }}
    {{- end }}
      blocks_storage:
        tsdb:
          wal_compression_enabled: true
        backend: s3
      {{ if ((.Values.argo.aws).efs) }}
        {{- if .Values.argo.aws.account }}
        s3:
          bucket_name: {{ .Values.argo.aws.bucketPrefix | default .Values.argo.clusterName }}-fm-mimir-tsdb
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
          bucket_name: {{ .Values.argo.aws.bucketPrefix | default .Values.argo.clusterName }}-fm-mimir-ruler
          endpoint: s3.{{ .Values.argo.aws.region }}.amazonaws.com
          region: {{ .Values.argo.aws.region }}
          insecure: false
          secret_access_key: ""
          access_key_id: ""
        {{- end }}
      {{- end }}
      limits:
        {{- if and .Values.argo.o11y .Values.argo.o11y.edgeNode .Values.argo.o11y.edgeNode.mimir .Values.argo.o11y.edgeNode.mimir.structuredConfig }}
        {{- if .Values.argo.o11y.edgeNode.mimir.structuredConfig.metricsRetentionPeriod }}
        compactor_blocks_retention_period: {{ .Values.argo.o11y.edgeNode.mimir.structuredConfig.metricsRetentionPeriod }}
        {{- end }}
        {{- if .Values.argo.o11y.edgeNode.mimir.structuredConfig.ingestionRate }}
        ingestion_rate: {{ .Values.argo.o11y.edgeNode.mimir.structuredConfig.ingestionRate }}
        {{- end }}
        {{- if .Values.argo.o11y.edgeNode.mimir.structuredConfig.maxGlobalSeriesPerUser }}
        max_global_series_per_user: {{ .Values.argo.o11y.edgeNode.mimir.structuredConfig.maxGlobalSeriesPerUser }}
        {{- end }}
        {{- end }}
  {{- with .Values.argo.o11y }}
  {{- if or .dedicatedEdgenodeEnabled (and .edgeNode .edgeNode.mimir (or .edgeNode.mimir.replicationFactor .edgeNode.mimir.storeGateway)) (and .edgeNode .edgeNode.commonConfig .edgeNode.commonConfig.storageClass) }}
  store_gateway: # Note: naming conversion to snake_case is intentional
    {{- if .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    {{- with .edgeNode }}
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
      {{- toYaml .resources | nindent 6}}
    {{- end }}
    {{- end }}
    {{- end }}
    {{- end }}
  {{- end }}
  {{- if or .dedicatedEdgenodeEnabled (and .edgeNode .edgeNode.mimir (or .edgeNode.mimir.replicationFactor .edgeNode.mimir.ingester)) (and .edgeNode .edgeNode.commonConfig .edgeNode.commonConfig.storageClass) }}
  ingester:
    {{- if .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    {{- with .edgeNode }}
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
      {{- toYaml .resources | nindent 6}}
    {{- end }}
    {{- end }}
    {{- end }}
    {{- end }}
  {{- end }}
  gateway:
    {{- if and .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    ingress:
      enabled: false
    {{- if and .edgeNode .edgeNode.mimir .edgeNode.mimir.gateway }}
    {{- if .edgeNode.mimir.gateway.replicas }}
    replicas: {{ .edgeNode.mimir.gateway.replicas }}
    {{- end }}
    {{- if .edgeNode.mimir.gateway.resources }}
    resources:
      {{- toYaml .edgeNode.mimir.gateway.resources | nindent 6 }}
    {{- end }}
    {{- end }}
  {{- if or .dedicatedEdgenodeEnabled (and .edgeNode .edgeNode.mimir .edgeNode.mimir.distributor) }}
  distributor:
    {{- if .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    {{- if and .edgeNode .edgeNode.mimir }}
    {{- with .edgeNode.mimir.distributor }}
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
  {{- if or .dedicatedEdgenodeEnabled (and .edgeNode .edgeNode.mimir .edgeNode.mimir.compactor) (and .edgeNode .edgeNode.commonConfig .edgeNode.commonConfig.storageClass) }}
  compactor:
    {{- if .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    {{- with .edgeNode }}
    {{- if or (and .mimir .mimir.compactor .mimir.compactor.volumeSize) (and .commonConfig .commonConfig.storageClass) }}
    persistentVolume:
      {{- if and .mimir .mimir.compactor .mimir.compactor.volumeSize }}
      size: {{ .mimir.compactor.volumeSize }}
      {{- end }}
      {{- if and .commonConfig .commonConfig.storageClass }}
      storageClass: {{ .commonConfig.storageClass }}
      {{- end }}
    {{- end }}
    {{- if and .mimir .mimir.compactor .mimir.compactor.replicas }}
    replicas: {{ .mimir.compactor.replicas }}
    {{- end }}
    {{- if and .mimir .mimir.compactor .mimir.compactor.resources }}
    resources:
      {{- toYaml .mimir.compactor.resources | nindent 6 }}
    {{- end }}
    {{- end }}
  {{- end }}
  query_frontend:
    {{- if .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    {{- if .edgeNode.mimir.query_frontend.resources }}
    resources:
      {{- toYaml .edgeNode.mimir.query_frontend.resources | nindent 6 }}
    {{- else }}
    resources: null
    {{- end }}

  {{- if or .dedicatedEdgenodeEnabled (and .edgeNode .edgeNode.mimir .edgeNode.mimir.querier) }}
  querier:
    {{- if .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    {{- if and .edgeNode .edgeNode.mimir .edgeNode.mimir.querier }}
    {{- if .edgeNode.mimir.querier.podAnnotations }}
    podAnnotations:
      {{- range $annotation_key, $annotation_value := .edgeNode.mimir.querier.podAnnotations }}
      {{ $annotation_key }}: "{{ $annotation_value }}"
      {{- end }}
    {{- end }}
    {{- if .edgeNode.mimir.querier.replicas }}
    replicas: {{ .edgeNode.mimir.querier.replicas }}
    {{- end }}
    {{- if .edgeNode.mimir.querier.resources }}
    resources:
      {{- toYaml .edgeNode.mimir.querier.resources | nindent 6 }}
    {{- end }}
    {{- end }}
  {{- end }}
  {{- if or .dedicatedEdgenodeEnabled (and .edgeNode .edgeNode.mimir .edgeNode.mimir.ruler) }}
  ruler:
    {{- if .dedicatedEdgenodeEnabled }}
    nodeSelector:
      node.kubernetes.io/custom-rule: observability
    tolerations:
    - key: "node.kubernetes.io/custom-rule"
      operator: "Equal"
      value: "observability"
      effect: "NoSchedule"
    {{- end }}
    {{- if and .edgeNode .edgeNode.mimir .edgeNode.mimir.ruler }}
    {{- if .edgeNode.mimir.ruler.replicas }}
    replicas: {{ .edgeNode.mimir.ruler.replicas }}
    {{- end }}
    {{- if .edgeNode.mimir.ruler.podAnnotations }}
    podAnnotations:
      {{- range $annotation_key, $annotation_value := .edgeNode.mimir.ruler.podAnnotations }}
      {{ $annotation_key }}: "{{ $annotation_value }}"
      {{- end }}
    {{- end }}
    {{- if .edgeNode.mimir.ruler.resources }}
    resources:
       {{- toYaml .edgeNode.mimir.ruler.resources | nindent 6 }}
    {{- end }}
    {{- end }}
  {{- end }}
  {{- end }}
  {{- /* end of with .Values.argo.o11y */}}

# OpenTelemetry additional pipelines config
opentelemetry-collector:
  {{- if and .Values.argo.o11y .Values.argo.o11y.dedicatedEdgenodeEnabled }}
  nodeSelector:
    node.kubernetes.io/custom-rule: observability
  tolerations:
  - key: "node.kubernetes.io/custom-rule"
    operator: "Equal"
    value: "observability"
    effect: "NoSchedule"
  {{- end }}
  {{- with .Values.argo.o11y.edgeNode.opentelemetryCollector.resources }}
  resources:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  image:
    repository: {{.Values.argo.containerRegistryURL}}/o11y/orch-otelcol
  imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 4 }}
  {{- end }}
  config:
    service:
      pipelines:
        metrics:
          receivers:
          - otlp
          {{- if index .Values.argo.enabled "infra-core" }}
          - prometheus/infra-exporter
          {{- end }}
          {{- if index .Values.argo.enabled "app-deployment-manager" }}
          - prometheus/adm-status
          {{- end }}
          {{- if and (index .Values.argo.enabled "multitenant_gateway") (index .Values.argo.enabled "alerting-monitor") (index .Values.argo.enabled "edgenode-observability") }}
          - prometheus/observability-tenant-controller
          {{- end }}
