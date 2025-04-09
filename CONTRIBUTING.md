# How to contribute
For information on submitting pull requests and issues, refer to the [Contributor Guide](https://docs.openedgeplatform.intel.com/edge-manage-docs/main/developer_guide/contributor_guide/index.html).

## License

edge-manageability-framework is licensed under the terms in [LICENSE](/LICENSES/Apache-2.0.txt).
By contributing to the project, you agree to the license and copyright terms therein and release your contribution under these terms.

## Get Started

### System Requirements

Your development machine must meet the following minimum requirements:

- Operating System: Ubuntu 22.04
- Processor: 32 cores or more with Intel VT-x and VT-d support enabled in BIOS
- Memory: 64 GB RAM or more
- Disk Space: 256 GB SSD or more

### Software Requirements

Your development machine must have the following software installed:

- [asdf v0.16.5 or higher](https://asdf-vm.com/guide/getting-started.html)
- [Docker](https://docs.docker.com/get-docker/)

### Set up development environment

TODO: Add a section about locally building the changes and testing them in the local environment.

TODO: Add section about exposing the Orchestrator API and UI to the local network.

To set up your development environment, follow these steps:

1. Ensure your development machine meets the [system](#system-requirements) and [software](#software-requirements)
   requirements.

1. Clone the repository:

   ```shell
   git clone https://github.com/open-edge-platform/edge-manageability-framework edge-manageability-framework
   cd edge-manageability-framework
   ```

1. Install APT dependencies:

    ```shell
    sudo apt-get update -y

    sudo apt-get install -y \
        python3.10-venv \
        unzip
    ```

1. Install `asdf` plugins:

   ```shell
    for plugin in golang jq mage; do
        asdf plugin add "${plugin}"
        asdf install "${plugin}"
    done

    mage asdfPlugins
   ```

1. Edit `terraform/orchestrator/terraform.tfvars` with your desired configuration.

    ```shell
    nano terraform/orchestrator/terraform.tfvars
    ```

    If you have Docker Hub credentials, you can add them to the `terraform/orchestrator/terraform.tfvars` file:

    ```hcl
    docker_username = "your_docker_username"
    docker_password = "your_docker_password"
    ```

    If you require a proxy for internet access, you can add it to the `terraform/orchestrator/terraform.tfvars` file:

    ```hcl
    http_proxy = "http://your_proxy:port"
    https_proxy = "http://your_proxy:port"
    no_proxy = "cluster.onprem,your_other_domains"
    ```

1. (Optional) Use locally built artifacts to deploy orchestrator

    Build repo archive and installer packages and move them to default directories

    ```shell
    mage tarball:onpremFull
    sudo rm -r repo_archives
    mkdir -p repo_archives
    mv onpremFull_edge-manageability-framework_$(head -1 VERSION).tgz repo_archives/
    cd on-prem-installers
    mage build:all
    export TF_VAR_deploy_tag=$(mage build:debVersion)
    sudo rm -r ../dist
    mv dist ..
    cd ..
    ```

    Edit `terraform/orchestrator/terraform.tfvars` to use locally built artifacts.

    ```hcl
    use_local_build_artifact = true
    ```

1. Start the deployment of the Orchestrator. This usually takes 15 minutes to install the platform elements (e.g., RKE2,
   Gitea, PostgreSQL, etc).

   ```shell
   mage deploy:onPrem
   ```

1. Once the previous command returns, you will be able to access the RKE2 cluster using the `kubectl` command.

    ```shell
    export KUBECONFIG=terraform/orchestrator/files/kubeconfig

    kubectl get pods -A
    ```

1. The
   deployment is likely not complete yet. To check the status of the deployment, you can run:

    ```shell
    mage deploy:waitUntilComplete
    ```

    This command will block until the deployment is complete.

1. Add Orchestrator server TLS certificate to the system's trusted store:

    ```shell
    mage gen:orchCA deploy:orchCA
    ```

1. Configure the development machine to use the edge network DNS server. This is required to resolve the
   Orchestrator server hostnames.

    ```shell
    mage deploy:edgeNetworkDNS
    ```

1. Validate the network configuration by running the following command:

    ```shell
    ping web-ui.cluster.onprem
    ```

    If the ping is successful, it means the DNS resolution and routing is working correctly.

1. You can execute end-to-end tests using a virtual Edge Node to validate the deployment:

    ```shell
    mage test:e2eOnPrem tenantUtils:createDefaultMtSetup test:onboarding
    ```

1. Congratulations 🎉 You have successfully set up your development environment for the Edge Orchestrator. You can now
   start developing and testing your changes. You can now reach the Orchestrator UI at
   [https://web-ui.cluster.onprem](https://web-ui.cluster.onprem). To get the default `admin` password, run:

    ```shell
    mage orchPassword
    ```

1. To tear down the deployment and reset the network, run:

    ```shell
    mage undeploy:onprem clean
    ```

### Architecture

The development environment is based on a single-node RKE2 cluster running inside a virtual machine. The following
components are installed:

- RKE2 (Kubernetes)
- Gitea (Git server)
- PostgreSQL (Database)
- Traefik (Ingress controller)
- ArgoCD (Continuous Deployment)
- Cert-Manager (TLS certificate management)
- Edge Orchestrator (the main application)
- Various utility functions and tools (orch-utils)

The GitHub Actions runner environment mirrors the local development environment by deploying the same components. This
ensures that the code functions consistently across both environments, allowing for reliable testing and validation of
changes made to the codebase.

![Orchestrator Edge Node CI/Dev Machine Architecture](/docs/images/orchestrator-edge-node-CI-architecture.drawio.svg)

## Development Guidelines

## Coding Standards

Consistently following coding standards helps maintain readability and quality. Adhere to the following conventions:

#### Golang

1. Follow the guidelines in [Effective Go](https://golang.org/doc/effective_go.html).
1. Use `gofmt` to format your code.
1. Write clear and concise comments for exported functions, types, and packages.
1. Use idiomatic Go constructs and avoid unnecessary complexity.
1. Ensure that your code is well-tested and includes unit tests for all functions.
1. Use descriptive variable and function names that clearly convey their purpose.
1. Avoid global variables and prefer dependency injection where possible.
1. Handle errors gracefully and provide meaningful error messages.
1. Code must pass `mage lint:golang` to ensure proper formatting.

#### Helm

1. Follow the [Helm Best Practices](https://helm.sh/docs/chart_best_practices/).
1. Use meaningful names for charts, templates, and values.
1. Code must pass `mage lint:helm` to ensure proper formatting.

#### Markdown

1. Use proper Markdown syntax for headings, lists, links, and code blocks.
1. Code must pass `mage lint:markdown` to ensure proper formatting.

#### Shell Script

1. Use `#!/usr/bin/env bash` at the top of your scripts to specify the shell.
1. Always use `set -o errexit` to ensure the script exits on the first error.
1. Use `set -o nounset` to treat unset variables as an error.
1. Use `set -o pipefail` to catch errors in pipelines.
1. Write clear and concise comments to explain the purpose of complex commands.
1. Use functions to encapsulate and reuse code.
1. Check the exit status of commands and handle errors appropriately.
1. Avoid using hardcoded paths; use variables and configuration files instead.
1. Ensure your scripts are idempotent and can be run multiple times without causing issues.
1. Use the long form of commands (e.g., `--verbose` instead of `-v`) for clarity.
1. Code must pass `mage lint:shell` to ensure proper formatting.

#### Terraform

1. Follow the [Terraform Style Guide](https://developer.hashicorp.com/terraform/language/style).
1. Code must pass `mage lint:terraform` to ensure proper formatting.

#### YAML

- Use proper YAML syntax for indentation, lists, and key-value pairs.
- Ensure that your YAML files are valid and well-structured.
- Code must pass `mage lint:yaml` to ensure proper formatting.

### Testing

1. Write unit, integration, and E2E tests for your code.
1. Ensure all static analysis and tests pass before submitting a pull request.
1. Aim for high test coverage to ensure code reliability.

### Continuous Integration

1. Submit a pull request (PR) to the `main` branch of the repository.
1. Wait for the CI to run and verify that all checks pass before merging.
1. If your PR is a work in progress, mark it as a draft to indicate that it's not ready for review yet.
1. Ensure that your code passes all continuous integration (CI) checks.
1. Address any feedback or requested changes from the CI or code reviewers.
1. If your PR introduces new features or changes existing functionality, ensure that it includes appropriate tests. If
   it fixes a bug, include a test that demonstrates the bug and verifies the fix whenever possible. This helps prevent
   the bug from reoccurring in the future.
1. Use descriptive commit messages that clearly explain the changes made.
1. Break down large changes into smaller, manageable commits to make it easier for reviewers to understand.
1. Ensure that your code is well-documented and includes comments where necessary to explain complex logic or decisions.
1. Keep your PR focused on a single change or feature to make it easier for reviewers to provide feedback.
1. Respond to code reviews in a timely manner and be open to feedback.
1. If your PR is related to a specific issue, reference that issue in the PR description to provide context.
1. Pin all dependencies to a specific **patch** version at a minimum in your code to ensure reproducibility.
1. Code should be reusable and portable across platforms. Avoid writing code that is tightly coupled to a specific CI
  environment. All code that runs in CI should be able to run locally as well.
1. CI workflows should primarily be executing the same Mage commands that a developer would run locally. There should
  not be any "magic" in the CI that is not also available locally.

### Documentation

1. Update the documentation to reflect your changes.
1. Write clear and concise docstrings for all functions, classes, and modules.
