#!/bin/bash

set -e

RELEASE_NAME="postgresql-cluster"
NAMESPACE="orch-database"
CHART="oci://ghcr.io/cloudnative-pg/charts/cluster"
VERSION="0.3.1"

# DATABASE INPUT (from your original values.yaml)
DATABASES=(
  "orch-app app-orch-catalog"
  "orch-platform platform-keycloak"
  "orch-platform vault"
  "orch-infra inventory"
  "orch-infra alerting"
  "orch-iam iam-tenancy"
  "orch-infra mps"
  "orch-infra rps"
)

GENERATED_VALUES="generated-values.yaml"

wait_for_pod() {
    local POD_NAME="$1"
    local NAMESPACE="$2"
    local TIMEOUT="${3:-300}"   # default 5 minutes
    local INTERVAL="${4:-15}"   # default 15 seconds

    echo "Waiting up to $TIMEOUT seconds for pod '$POD_NAME' in namespace '$NAMESPACE' to be Ready (checking every $INTERVAL seconds)..."

    local elapsed=0
    while [ $elapsed -lt $TIMEOUT ]; do
        if kubectl get pod "$POD_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
            # Pod exists, now wait for Ready condition
            if kubectl wait --for=condition=ready pod/"$POD_NAME" -n "$NAMESPACE" --timeout="${INTERVAL}s" >/dev/null 2>&1; then
                echo "✅ Pod '$POD_NAME' is Ready!"
                return 0
            fi
        else
            echo "Pod '$POD_NAME' not found yet, waiting..."
        fi
        sleep $INTERVAL
        elapsed=$((elapsed + INTERVAL))
    done

    echo "⚠️ Timeout reached. Pod '$POD_NAME' is not Ready after $TIMEOUT seconds."
    return 1
}

generate_values() {
    echo "Generating dynamic values.yaml..."

    cp values.yaml $GENERATED_VALUES

    echo "  roles:" >> $GENERATED_VALUES

    for entry in "${DATABASES[@]}"; do
        ns=$(echo $entry | awk '{print $1}')
        name=$(echo $entry | awk '{print $2}')

        user="${ns}-${name}_user"
        secret="${ns}-${name}"

        cat >> $GENERATED_VALUES <<EOF
    - name: ${user}
      ensure: present
      login: true
      passwordSecret:
        name: ${secret}
EOF
    done

    echo "  initdb:" >> $GENERATED_VALUES
    cat >> $GENERATED_VALUES <<EOF
    database: postgres
    owner: orch-database-postgresql_user
    localeCType: "en_US.UTF-8"
    localeCollate: "en_US.UTF-8"
    secret:
      name: "orch-database-postgresql"
    postInitSQL:
EOF

    for entry in "${DATABASES[@]}"; do
        ns=$(echo $entry | awk '{print $1}')
        name=$(echo $entry | awk '{print $2}')

        db="${ns}-${name}"
        user="${db}_user"

        cat >> $GENERATED_VALUES <<EOF
      - CREATE DATABASE "${db}";
      - BEGIN;
      - REVOKE CREATE ON SCHEMA public FROM PUBLIC;
      - REVOKE ALL ON DATABASE "${db}" FROM PUBLIC;
      - CREATE ROLE "${user}";
      - GRANT CONNECT ON DATABASE "${db}" TO "${user}";
      - GRANT ALL PRIVILEGES ON DATABASE "${db}" TO "${user}";
      - ALTER DATABASE "${db}" OWNER TO "${user}";
      - COMMIT;
EOF
    done
}

ACTION=$1

if [ "$ACTION" = "install" ]; then
    kubectl get ns $NAMESPACE >/dev/null 2>&1 || kubectl create ns $NAMESPACE

    generate_values

    helm upgrade --install $RELEASE_NAME $CHART \
        --version $VERSION \
        -n $NAMESPACE \
        -f $GENERATED_VALUES \
	--wait --timeout 5m;
    wait_for_pod postgresql-cluster-1 $NAMESPACE

elif [ "$ACTION" = "uninstall" ]; then
    helm uninstall $RELEASE_NAME -n $NAMESPACE || true
    rm -f $GENERATED_VALUES

else
    echo "Usage: $0 {install|uninstall}"
    exit 1
fi
