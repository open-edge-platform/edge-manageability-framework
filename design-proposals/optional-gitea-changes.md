# Eliminate Gitea as a pre-installer dependency

As part of the [platform installer simplification](platform-installer-simplification.md), one of the goals is to decouple
the platform from a mandatory internal Gitea instance, making it an optional component required only for specific
features (like App Orchestration). This document outlines the below architectural and code changes introduced.

## 1. Optional Gitea Installation

The most significant change is that the internal Gitea service is no longer installed by default. It is now
conditionally deployed based on whether **App Orchestration (AO)** is enabled for the target cluster.

### Logic & Implementation

The system determines whether to install Gitea by checking the cluster's configuration profile for the inclusion of the
App Orchestration component.

#### Magefile (`mage/`)

- **Detection**: A new method `isAOEnabled(targetEnv)` was added to `mage/config.go`.
  - It reads the cluster configuration file (`orch-configs/clusters/<env>.yaml`).
  - It parses the `clusterValues` list.
  - It returns `true` if it finds a reference to `enable-app-orch.yaml`.
- **Deployment**: In `mage/deploy.go`, the `gitea` function now wraps the helm install command and account creation
  logic in a conditional block:

  ```go
  aoEnabled, _ := (Config{}).isAOEnabled(targetEnv)
  if aoEnabled {
      // Install Gitea helm chart
      // Create accounts (argocd, apporch, clusterorch)
  }
  ```

#### On-Prem Installer (`on-prem-installers/`)

- **Shell Script**: `onprem/onprem_orch_install.sh` performs a similar check using `grep`.
  - It scans the generated profile YAML for `enable-app-orch.yaml`.
  - It sets the environment variable `INSTALL_GITEA="true"` or `"false"`.
  - This variable is passed to the `apt-get install` command for the Gitea package.
- **Go Installer**: The `cmd/onprem-orch-installer/main.go` binary reads the `INSTALL_GITEA` environment variable.
  - If `false`, it skips the step of pushing artifact tarballs to the internal Gitea repo.
  - It modifies the `getGiteaServiceURL` function to handle cases where the service might not exist (though currently
    it defaults to a placeholder if not installed).

## 2. Deployment Repository Source Change

The default source of truth for the Orchestrator's ArgoCD applications has shifted from the internal Gitea instance to
the public GitHub repository.

### Key Changes

- **Default URL**:
  - In `mage/config.go` and `installer/cluster.tpl`, the `deployRepoURL` default value was changed from:
    `https://gitea-http.gitea.svc.cluster.local/argocd/edge-manageability-framework`
    to:
    `https://github.com/open-edge-platform/edge-manageability-framework`
- **ArgoCD Configuration**:
  - `mage/argo.go`: The `repoAdd` function was updated. It now checks if credentials (`gitUser`, `gitToken`) are
    provided. If they are empty (which is the case for public GitHub), it adds the repository without authentication
    flags.
- **Removal of Sync Logic**:
  - Previously, `mage deploy` would automatically synchronize the local `orch-configs` and `argocd` directories to the
    internal Gitea instance using `updateDeployRepo`.
  - This logic has been removed from the main deployment flow in `mage/deploy.go`. The assumption is that deployments
    now pull from upstream, or users must manually configure a different repo if they want to deploy local changes.

## 3. Configuration Refactoring

Several changes were made to how cluster configurations are processed to support this flexibility.

- **Template Merging (`mage/config.go`)**:
  - The `parseClusterValues` function was enhanced to support "self-merging". It can now read a cluster template and
    merge it with itself to resolve internal references.
  - Logic was added to programmatically merge the `proxyProfile` into the cluster values, reducing the reliance on
    complex Helm template logic for proxy settings.
- **Cluster Template (`orch-configs/templates/cluster.tpl`)**:
  - Simplified the template by removing explicit conditional inclusions for proxy profiles, as this is now handled by
    the Go code in `mage`.
