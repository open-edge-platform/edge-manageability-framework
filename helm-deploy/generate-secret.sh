#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Script to generate a secure password and create Kubernetes secrets for Keycloak and Postgres
# Creates namespaces if they do not exist

set -euo pipefail

# Function to generate a strong random password
generate_password() {
    lowercase=$(tr -dc 'a-z' < /dev/urandom | head -c 1)
    uppercase=$(tr -dc 'A-Z' < /dev/urandom | head -c 1)
    digit=$(tr -dc '0-9' < /dev/urandom | head -c 1)
    special=$(tr -dc '!@#$%^&*()_+{}|:<>?' < /dev/urandom | head -c 1)
    remaining=$(tr -dc 'a-zA-Z0-9!@#$%^&*()_+{}|:<>?' < /dev/urandom | head -c 21)
    password=$(echo "$lowercase$uppercase$digit$special$remaining" | fold -w1 | shuf | tr -d '\n')
    echo "$password"
}

# Function to create/update Keycloak secret
create_keycloak_secret() {
    local namespace="$1"
    local password="$2"

    # Ensure namespace exists
    if ! kubectl get namespace "$namespace" &> /dev/null; then
        echo "Namespace '$namespace' not found. Creating..."
        kubectl create namespace "$namespace"
    fi

    # Delete existing secret if it exists
    kubectl -n "$namespace" delete secret platform-keycloak --ignore-not-found

    # Apply new secret
    kubectl -n "$namespace" apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: platform-keycloak
stringData:
  username: "admin"
  password: "$password"
  admin-password: "$password"
EOF

    echo "Keycloak secret created/updated in namespace '$namespace'."
}

# Function to create/update Postgres secret
create_postgres_secret() {
    local namespace="$1"
    local password="$2"

    # Ensure namespace exists
    if ! kubectl get namespace "$namespace" &> /dev/null; then
        echo "Namespace '$namespace' not found. Creating..."
        kubectl create namespace "$namespace"
    fi

    # Delete existing secret if it exists
    kubectl -n "$namespace" delete secret "$namespace-postgresql" --ignore-not-found

    # Apply new secret
    kubectl -n "$namespace" apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: "$namespace-postgresql"
  annotations:
    cnpg.io/reload: "false"
type: kubernetes.io/basic-auth
stringData:
  username: "$namespace-postgresql_user"
  password: "$password"
EOF

    echo "Postgres secret created/updated in namespace '$namespace'."
}

# Check kubectl
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed or not in PATH."
    exit 1
fi

# Generate passwords
keycloak_password=$(generate_password)
postgres_password=$(generate_password)

# Create secrets
create_keycloak_secret "orch-platform" "$keycloak_password"
create_postgres_secret "orch-database" "$postgres_password"

# Output generated passwords
echo "Generated Keycloak password: $keycloak_password"
echo "Generated Postgres password: $postgres_password"
