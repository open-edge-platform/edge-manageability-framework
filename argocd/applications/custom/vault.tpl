# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

global:
  imagePullSecrets:
  {{- with .Values.argo.imagePullSecrets }}
    {{- toYaml . | nindent 4 }}
  {{- end }}

server:
  {{- if .Values.argo.vault.ha}}
  # Run Vault in "HA" mode. There are no storage requirements unless the audit log
  # persistence is required.  In HA mode Vault will configure itself to use Consul
  # for its storage backend.  The default configuration provided will work the Consul
  # Helm project by default.  It is possible to manually configure Vault to use a
  # different HA backend.
  ha:
    enabled: true
    replicas: {{.Values.argo.vault.replicas}}
  {{- else}}
  standalone:
  {{- end}}
    # config is a raw string of default configuration when using a Stateful
    # deployment. Default is to use a PersistentVolumeClaim mounted at /vault/data
    # and store data there. This is only used when using a Replica count of 1, and
    # using a stateful set. This should be HCL.
    # Note: Configuration files are stored in ConfigMaps so sensitive data
    # such as passwords should be either mounted through extraSecretEnvironmentVars
    # or through a Kube secret.  For more information see:
    # https://www.vaultproject.io/docs/platform/k8s/helm/run#protecting-sensitive-vault-configurations
    config: |
      ui = true
      # Internal cluster traffic will leverage the mTLS service mesh
      listener "tcp" {
        tls_disable = 1

        address = "[::]:8200"
        cluster_address = "[::]:8201"
      }
  {{- if and .Values.argo.vault.autoUnseal (ne .Values.argo.namespace "onprem")}}
      seal "awskms" {}

  # extraEnvironmentVars is a list of extra environment variables to set with the stateful set. These could be
  # used to include variables required for auto-unseal.
  extraEnvironmentVars:
    http_proxy: {{.Values.argo.proxy.httpProxy}}
    https_proxy: {{.Values.argo.proxy.httpsProxy}}
    # yamllint disable-line rule:line-length
    no_proxy: {{.Values.argo.proxy.noProxy}}
    AWS_REGION: {{.Values.argo.aws.region}}
    VAULT_AWSKMS_SEAL_KEY_ID: alias/vault-kms-unseal-{{.Values.argo.clusterName}}

  # extraSecretEnvironmentVars is a list of extra environment variables to set with the stateful set.
  # These variables take value from existing Secret objects.
  extraSecretEnvironmentVars:
    - envName: AWS_ACCESS_KEY_ID
      secretName: vault-kms-unseal
      secretKey: AWS_ACCESS_KEY_ID
    - envName: AWS_SECRET_ACCESS_KEY
      secretName: vault-kms-unseal
      secretKey: AWS_SECRET_ACCESS_KEY

  # https://jira.devtools.intel.com/browse/NEXENPL-1126
  # enable liveness probe such that pod is restarted when auto-unseal failed
  livenessProbe:
    enabled: true
  {{- end}}

  extraInitContainers:
    # This initContainer consumes Postgres credential via secret and converts it into a config file that can be consumed by vault
    - name: storage-config
      image: alpine:3.18.2
      securityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop:
            - ALL
        seccompProfile:
          type: RuntimeDefault
      command: [sh, -c]
      args:
        - echo "storage \"postgresql\" { connection_url = \"postgres://$PGUSER:$PGPASSWORD@$PGHOST:$PGPORT/$PGDATABASE\" ha_enabled=\"$HA_ENABLED\" }" > /vault/userconfig/vault-storage-config/storage.hcl
      volumeMounts:
        - mountPath: /vault/userconfig/vault-storage-config
          name: vault-storage-config
      env:
        - name: PGUSER
          valueFrom:
            secretKeyRef:
              # TODO Unify the database name and secret between local and cloud deployments
              name: vault-{{.Values.argo.database.type}}-postgresql
              key: PGUSER
        - name: PGPASSWORD
          valueFrom:
            secretKeyRef:
              name: vault-{{.Values.argo.database.type}}-postgresql
              key: PGPASSWORD
        - name: PGHOST
          valueFrom:
            secretKeyRef:
              name: vault-{{.Values.argo.database.type}}-postgresql
              key: PGHOST
        - name: PGPORT
          valueFrom:
            secretKeyRef:
              name: vault-{{.Values.argo.database.type}}-postgresql
              key: PGPORT
        - name: PGDATABASE
          valueFrom:
            secretKeyRef:
              name: vault-{{.Values.argo.database.type}}-postgresql
              key: PGDATABASE
        - name: HA_ENABLED
          value: {{ .Values.argo.vault.ha | default false | quote }}
    # This initContainer creates database tables for vault
    - name: init-table
      image: bitnamilegacy/postgresql:14.5.0-debian-11-r2
      securityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop:
            - ALL
        seccompProfile:
          type: RuntimeDefault
      command: [bash, -x, -c]
      args:
        - |
          EXIST=$(psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDATABASE -t -A -c \
            "SELECT EXISTS (SELECT FROM pg_tables WHERE tablename  = 'vault_kv_store' OR tablename = 'vault_ha_locks');"
          )
          if [[ $EXIST == "t" ]]; then
            echo "Tables already exist, skipping initialization"
            exit 0
          fi

          psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDATABASE -c \
          "CREATE TABLE vault_kv_store (
            parent_path TEXT COLLATE \"C\" NOT NULL,
            path        TEXT COLLATE \"C\",
            key         TEXT COLLATE \"C\",
            value       BYTEA,
            CONSTRAINT pkey PRIMARY KEY (path, key)
          );
          CREATE INDEX parent_path_idx ON vault_kv_store (parent_path);
          CREATE TABLE vault_ha_locks (
            ha_key      TEXT COLLATE \"C\" NOT NULL,
            ha_identity TEXT COLLATE \"C\" NOT NULL,
            ha_value    TEXT COLLATE \"C\",
            valid_until TIMESTAMP WITH TIME ZONE NOT NULL,
            CONSTRAINT ha_key PRIMARY KEY (ha_key)
          );"
      env:
        - name: PGUSER
          valueFrom:
            secretKeyRef:
              name: vault-{{.Values.argo.database.type}}-postgresql
              key: PGUSER
        - name: PGPASSWORD
          valueFrom:
            secretKeyRef:
              name: vault-{{.Values.argo.database.type}}-postgresql
              key: PGPASSWORD
        - name: PGHOST
          valueFrom:
            secretKeyRef:
              name: vault-{{.Values.argo.database.type}}-postgresql
              key: PGHOST
        - name: PGPORT
          valueFrom:
            secretKeyRef:
              name: vault-{{.Values.argo.database.type}}-postgresql
              key: PGPORT
        - name: PGDATABASE
          valueFrom:
            secretKeyRef:
              name: vault-{{.Values.argo.database.type}}-postgresql
              key: PGDATABASE
{{- with .Values.argo.resources.vault.server }}
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
{{- with .Values.argo.resources.vault.injector }}
injector:
  resources:
    {{- toYaml . | nindent 4 }}
{{- end }}
