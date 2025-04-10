# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: manager
          args:
            - '--diagnostics-address=:8080'
            - '--insecure-diagnostics=true'
          ports:
            - containerPort: 8080
              name: metrics
              protocol: TCP
