# E2E Cloud Installer Testing

This document describes the structure, intent, how to run, and how to extend end-to-end tests for the Orchestrator cloud installer.

## Scope

The installer automation test logic implementation is expected to be specific for the Orchestrator version in the current branch (main, release-*, etc). The only exception on this limitation is this current 24.05.1 preerelease version will of necessity support 24.03.2 installation to support upgrade test automation from the released software.

The test logic is expected to be updated for each new Orchestrator version and the pipeline exceution layer should handle pulling and calling the correct install AutoInstall implemenation for each version that is part of a test scenario.

## Usage

```sh
usage: auto_install.py [-h] [-m {install,upgrade,uninstall}] [-p {full}]

Script to automate workflows for the Edge Orchestrator cloud installer within Jenkins E2E validation pipelines.

optional arguments:
  -h, --help                                show this help message and exit
  -m, --mode {install,upgrade,uninstall}    Specify the mode (install, upgrade, uninstall)
  -p, --product {full}                      Specify the product (full)
```

## Environment Variables

| Variable                   | Description                                                                   |
|----------------------------|-------------------------------------------------------------------------------|
| **Automation workflow**    |                                                                               |
| `AUTO_INSTALL_LOGPATH`     | The path for the installer execution output log file.                         |
| `ENABLE_TIMING_DATA`       | Flag indicating whether timing data is enabled. (true/false)                  |
| `ENABLE_INTERACTIVE_DEBUG` | Flag indicating whether interactive debugging is enabled. (true/false)        |
| **Cluster installation**   |                                                                               |
| `AWS_REGION`               | AWS region for the installed cluster.                                         |
| `AWS_ACCOUNT`              | AWS account for the installed cluster.                                        |
| `CLUSTER_NAME`             | The name of the cluster.                                                      |
| `CLUSTER_DOMAIN`           | The parent domain of the cluster.                                             |
| `STATE_BUCKET_PREFIX`      | Shared state bucket for the cluster being installed.                          |
| `STATE_PATH`               | Local path mounted in the installer container to store shared state data.     |
| **Cluster configuration**  |                                                                               |
| `USE_TEST_PROVISION_CONFIG`| Path to the test provision config file.                                       |
| `DISABLE_AUTOCERT`         | Flag indicating whether auto certificate is disabled. (true/false)            |
| `CLUSTER_PROFILE`          | The scaling profile for cluster(default, 100en, 1ken, 10ken)                  |
| **Account initialization** |                                                                               |
| `ENABLE_ACCOUNT_INIT`      | Flag to specify whether account initialization is run on install. (true/false)|
| `ENABLE_ACCOUNT_RESET`     | Flag to specify whether account reset is run on uninstall. (true/false)       |
| `RS_REFRESH_TOKEN`         | The refresh token for the installation workflow.                              |
| **Proxy settings**         |                                                                               |
| `https_proxy`              | HTTPS proxy setting.                                                          |
| `socks_proxy`              | SOCKS proxy is required if `https_proxy` is set.                              |

## Scenarios

Sample pipeline implementation is described by the `jenkins-*.sh` files. These are not intended to be run directly, but to be used as a reference for creating a Jenkins pipeline. They can be run in a developer environment for test purposes.

### Clean 24.05 Install and verify functionality

1. Run the install operation:

    ```sh
    python $test_path/auto_install.py --mode uninstall --product full
    ```

    * *This script blocks until the provisioning phase is complete and the `argocd` installation phase has started.*
    * *Returns 0 if the provisioning phase is successful, and a non-zero value if it fails.*

2. Wait for `argocd` installation to complete. This is best done using the standard CI wait operation. You will need to ensure the tunnel to access the target cluster management is up. That process is shown in `jenkins-await-install.sh` which use installer saved state data to set up the tunnel and call:

        ```sh
        mage -v deploy:waitUntilComplete
        ```

3. Perform acceptance/install test cases

### Uninstall 24.05 and verify removal

1. Run the uninstall operation:

    ```sh
    python $test_path/auto_install.py --mode uninstall --product full
    ```

    * *This script blocks while AWS resources are deprovisioned.*
    * *Returns 0 if the provisioning phase is successful, and a non-zero value if it fails.*

2. Perform acceptance/uninstall test cases

### Upgrade Orchestrator 24.03.2 to 24.05 and verify functionality

TBD: Currently not available - Requires "legacy" version of auto_install.py that supports 24.03.2 commandline options.

In general, the Installer automation logic is expected to be on the branch that implements a specific release, and the pipeline execution layer should handle pulling and calling the correct install AutoInstall implementation for each version that is part of a test scenario.

However, the automation logic was created after the release branch for 24.03.2 was branched and frozen, so the automation logic for that version will be in a separate `e2e-tests/installer/legacy` folder in the `main` and `release-24.05.1` branches.

The `legacy` folder will be removed from `main` when `release-24.05.1` is released. `release-24.08.0` is not expected to have a `legacy` folder.

### Upgrade Orchestrator and Test EN from 24.03.2 to 24.05.1 and verify functionality

TBD: Currently not available. EN upgrade implementation is not yet complete. Installer workflow changes are not planned at this time, but this workflow needs to be validated.

## Scenarios

Sample pipeline implementation is described by the `jenkins-*.sh` files. These are not intended to be run directly, but to be used as a reference for creating a Jenkins pipeline. They can be run in
a developer environment for test purposes.

### Clean 24.05 Install and verify functionality

1. Run the install operation:

    ```sh
    python $test_path/auto_install.py --mode uninstall --product full
    ```

    This blocks until the provisioning phase is complete and the `argocd` installation phase has started. The script will return 0 if the provisioning phase is successful, and a non-zero value if it fails.

2. Wait for `argocd` installation to complete. This is best done using the standard CI wait operation. You will need to ensure the tunnel to access the target cluster management is up. That process is shown in `jenkins-await-install.sh` which use installer saved state data to set up the tunnel and call:

    ```sh
    mage -v deploy:waitUntilComplete
    ```

3. Perform acceptance/install test cases

### Uninstall 24.05 and verify removal

1. Run the uninstall operation:

    ```sh
    python $test_path/auto_install.py --mode uninstall --product full
    ```

    This blocks until the AWS resources are deprovisioned. The script will return 0 if the provisioning phase is successful, and a non-zero value if it fails.

2. Perform acceptance/uninstall test cases

### Upgrade Orchestrator 24.03.2 to 24.05 and verify functionality

TBD: Currently not available - Requires "legacy" version of auto_install.py that supports 24.03.2 commandline options.

In general, the Installer automation logic is expected to be on the branch that implements a specific release, and the pipeline execution layer should handle pulling and calling the correct install AutoInstall implementation for each version that is part of a test scenario.

However, the automation logic was created after the release branch for 24.03.2 was branched and frozen, so the automation logic for that version will be in a separate `e2e-tests/installer/legacy` folder in the `main` and `release-24.05.1` branches. It will be removed from `main` when `release-24.05.1` is released. `release-24.08.0` is not expected to have a `legacy` folder.

### Upgrade Orchestrator and Test EN from 24.03.2 to 24.05.1 and verify functionality

TBD: Currently not available. EN upgrade implementation is not yet complete. Installer workflow changes are not planned at this time, but this workflow needs to be validated.
