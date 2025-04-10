# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
apiVersion: v1
kind: Service
metadata:
  name: capi-controller-metrics
  namespace: capi-system
  labels:
    control-plane: controller-manager
    app: metric-svc
spec:
  ports:
    - name: metrics
      port: 8080
      protocol: TCP
      targetPort: metrics
  selector:
    control-plane: controller-manager
  type: ClusterIP
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: capi-controller-metrics
  namespace: capi-system
spec:
  endpoints:
  - port: metrics
    scheme: http
    path: /metrics
  namespaceSelector:
    matchNames:
    - capi-system
  selector:
    matchExpressions:
    - key: prometheus.io/service-monitor
      operator: NotIn
      values:
      - "false"
    matchLabels:
      control-plane: controller-manager
      app: metric-svc
---
apiVersion: v1
kind: Service
metadata:
  name: capi-operator-metrics
  namespace: capi-operator-system
  labels:
    control-plane: controller-manager
    app: metric-svc
spec:
  ports:
    - name: metrics
      port: 8080
      protocol: TCP
      targetPort: metrics
  selector:
    control-plane: controller-manager
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: capi-operator-metrics
  namespace: capi-operator-system
spec:
  endpoints:
  - port: metrics
    scheme: http
    path: /metrics
  namespaceSelector:
    matchNames:
    - capi-operator-system
  selector:
    matchExpressions:
    - key: prometheus.io/service-monitor
      operator: NotIn
      values:
      - "false"
    matchLabels:
      control-plane: controller-manager
      app: metric-svc
---
apiVersion: v1
kind: Service
metadata:
  name: capi-rke2-metrics
  namespace: capr-system
  labels:
    control-plane: controller-manager
    app: metric-svc
spec:
  ports:
    - name: metrics
      port: 8080
      protocol: TCP
      targetPort: metrics
  selector:
    control-plane: controller-manager
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: capi-rke2-metrics
  namespace: capr-system
spec:
  endpoints:
  - port: metrics
    scheme: http
    path: /metrics
  namespaceSelector:
    matchNames:
    - capr-system
  selector:
    matchExpressions:
    - key: prometheus.io/service-monitor
      operator: NotIn
      values:
      - "false"
    matchLabels:
      control-plane: controller-manager
      app: metric-svc
