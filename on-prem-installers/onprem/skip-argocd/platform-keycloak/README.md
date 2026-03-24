Platform Keycloak — Helm deploy bundle

Contents
- `values.yaml` — single Helm values file to edit before deploy
- `platform-keycloak.sh` — installer script (install|uninstall|status)

Prerequisites
- `helm` and `kubectl` available in PATH
- Kubernetes cluster access and permission to create namespace `orch-platform`
- Ensure the Helm chart repository containing `common/charts/keycloak-instance` is added and updated, for example:
  ```bash
  helm repo add myrepo https://charts.example.com
  helm repo update
  ```
- Required secrets must exist in the target cluster:
  - Secret `platform-keycloak` containing `username` and `password`
  - Secret `platform-keycloak-postgresql` (PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD)

Quick start
```bash
chmod +x platform-keycloak.sh
# install (uses values.yaml by default)
./platform-keycloak.sh install

# uninstall (optionally delete namespace)
./platform-keycloak.sh uninstall
./platform-keycloak.sh --delete-namespace uninstall

# override chart or version (example uses chart name and pinned version)
./platform-keycloak.sh --chart common/charts/keycloak-instance --version 26.1.2 install
```

Notes
-- This folder is self-contained; edit `values.yaml` only. The script will not install `helm` or `kubectl` for you.
