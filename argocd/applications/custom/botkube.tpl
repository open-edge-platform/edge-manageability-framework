# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

communications:
  default-group:
    webhook:
      enabled: true
      url: "http://cert-synchronizer-service.orch-gateway.svc.cluster.local/updatecert"
      bindings:
        # -- Notification sources configuration for the webhook.
        sources:
          - k8s-certificate-events
extraEnv:
            {{- if .Values.argo.proxy }}
            {{- if .Values.argo.proxy.httpProxy }}
            - name: HTTP_PROXY
              value: "{{ .Values.argo.proxy.httpProxy }}"
            - name: http_proxy
              value: "{{ .Values.argo.proxy.httpProxy }}"
            {{- end }}
            {{- if .Values.argo.proxy.httpsProxy }}
            - name: HTTPS_PROXY
              value: "{{ .Values.argo.proxy.httpsProxy }}"
            - name: https_proxy
              value: "{{ .Values.argo.proxy.httpsProxy }}"
            {{- end }}
            {{- if .Values.argo.proxy.noProxy }}
            - name: NO_PROXY
              value: "{{ .Values.argo.proxy.noProxy }}"
            - name: no_proxy
              value: "{{ .Values.argo.proxy.noProxy }}"
            {{- end }}
            {{- end }}
settings:
  clusterName: "certman-test-cluster"
  log:
    level: info

# -- Configures security context to manage user Privileges in Pod.
# [Ref doc](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod).
# @default -- Runs as a Non-Privileged user.
securityContext:
  runAsNonRoot: true
  runAsUser: 101
  runAsGroup: 101

# -- Configures container security context.
# [Ref doc](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container).
containerSecurityContext:
  privileged: false
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  capabilities:
    drop:
      - ALL
  seccompProfile:
    type: "RuntimeDefault"
sources:
  'k8s-certificate-events':
    displayName: "Cert-Manager Created Events"
    # -- Describes Kubernetes source configuration.
    # @default -- See the `values.yaml` file for full object.
    botkube/kubernetes:
      context: &default-plugin-context
        # -- RBAC configuration for this plugin.
        rbac:
          group:
            # -- Static impersonation for a given username and groups.
            type: Static
            # -- Prefix that will be applied to .static.value[*].
            prefix: ""
            static:
              # -- Name of group.rbac.authorization.k8s.io the plugin will be bound to.
              values: ["botkube-plugins-default"]
      enabled: true
      config:
        namespaces:
            include:
              - "orch-gateway"
        event:
           # -- Lists all event types to be watched.
          types:
           - create
           - update
           - delete
           - error
        resources:
            - type: cert-manager.io/v1/certificates
              updateSetting:
                includeDiff: true
                fields:
                  - status.renewalTime

{{- with .Values.argo.resources.botkube }}
resources:
  {{- toYaml . | nindent 2 }}
{{- end }}
