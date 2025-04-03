 [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0) [![Virtual Integration](https://github.com/open-edge-platform/edge-manageability-framework/actions/workflows/virtual-integration.yml/badge.svg?branch=main)](https://github.com/open-edge-platform/edge-manageability-framework/actions/workflows/virtual-integration.yml?query=branch%3Amain)

# Orchestrator Deploy

TODO: Add GHA status badges.

TODO: Update all links once the GitHub repository is created.

## Overview

Welcome to the Edge Orchestrator project! This repository contains the source code and documentation for deploying and
managing the Edge Orchestrator, a comprehensive platform designed to facilitate the deployment, management, and
orchestration of edge computing resources.

### Key Components

#### edge-manageability-framework

The [edge-manageability-framework](https://github.com/open-edge-platform/edge-manageability-framework) repository is the central hub for deploying the Edge Orchestrator. It includes Argo CD applications, Helm charts, and deployment scripts necessary for setting up the orchestrator in various environments, including on-premise and cloud-based setups.

#### orch-utils

The [orch-utils](https://github.com/open-edge-platform/orch-utils) repository provides various utility functions and tools that support the deployment and management of the Edge Orchestrator. This includes Kubernetes jobs, Helm charts, Dockerfiles, and Go code for tasks such as namespace creation, policy management, and Traefik route configuration.

## Get Started

See the [Documentation](https://github.com/open-edge-platform) to get started using edge-manageability-framework.

TODO: what docs (link above) has the Get Started Guide?

TODO: Use Make targets before releasing source code.

### Lint

```sh
mage lint:all
```

### Test

```sh
mage test:go
```

### Build

```sh
echo TODO
```

### Release

```sh
echo TODO
```

## Develop

To develop edge-manageability-framework, the following development prerequisites are required:

- [Go](https://go.dev/doc/install)
- [Mage](https://magefile.org/)
- [asdf](https://asdf-vm.com/guide/getting-started.html)
- [Docker](https://docs.docker.com/get-docker/)

To build and test edge-manageability-framework, first clone the repository:

```sh
git clone https://github.com/open-edge-platform/edge-manageability-framework edge-manageability-framework
cd edge-manageability-framework
```

Then, install the required install tools:

```sh
mage asdfPlugins
```

## Contribute

To learn how to contribute to the project, see the [Contributor's Guide](/CONTRIBUTING.md).

## Community and Support

To learn more about the project, its community, and governance, visit the [Edge Orchestrator
Community](https://github.com/open-edge-platform).

## License

Copyright 2025 Intel Corporation

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.