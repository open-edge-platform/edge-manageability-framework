# E2E Cloud Installer Testing

This document describes the structure, intent, how to run, and how to extend
end-to-end tests for the Orchestrator cloud installer.

## Scope

The installer automation test logic implementation is expected to be specific
for the Orchestrator version in the current branch (main, release-*, etc). The
only exception on this limitation is this current 24.05.1 preerelease version
will of necessity support 24.03.2 installation to support upgrade test
automation from the released software.

The test logic is expected to be updated for each new Orchestrator version and
the pipeline exceution layer should handle pulling and calling the correct
install AutoInstall implemenation for each version that is part of a test
scenario.

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

Sample pipeline implementation is described by the `jenkins-*.sh` files. These
are not intended to be run directly, but to be used as a reference for creating
a Jenkins pipeline. They can be run in a developer environment for test
purposes.
