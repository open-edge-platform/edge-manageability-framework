# Design Proposal: Orchestrator Deployment Experience Improvement

Author(s): Charles Chan

Last updated: May, 27, 2025

## Abstract

The current orchestrator installer in 3.0 suffers from a number of architectural and operational issues that impede
development velocity, cloud support, and user experience:

- The cloud installer and infrastructure provisioning are implemented as **large monolithic shell scripts**.
  This not only makes them difficult to maintain but also renders unit testing nearly impossible.
- The installer is **only tested late in the hardware integration pipeline (HIP)**, which delays feedback and makes bugs harder to trace and resolve.
- The on-prem installer was developed in parallel but shares little code or structure with the cloud installer.
  This results in **inconsistent behaviors and duplicated logic between cloud and on-prem** deployments.
- There is **no clear boundary between infrastructure provisioning and orchestrator** setup,
  making it difficult to port components to another cloud provider or isolate issues during upgrade and testing.
- **Upgrade support was added as a second thought** and lacks proper design.
- **Error handling is poor**; raw error logs are surfaced directly to users with no actionable remediation,
  and failures require rerunning entire stages without guidance.

This proposal aims to significantly improve the deployment experience of EMF across multiple environments (AWS, Azure, and on-prem).
The new installer will prioritize user experience by offering a streamlined, zero-touch installation process after configuration,
along with clear error handling and actionable feedback.
It will also increase cloud portability through clear infrastructure abstraction and support for Azure.
Finally, by replacing monolithic shell scripts with modular Go components and adding test coverage, we will enable faster iteration and more frequent releases.

## Proposal

### Scope of work

- A **unified installer** that supports AWS, Azure, and on-prem targets.
- A **text user interface (TUI) configuration builder** that guides users through required inputs,
  with the ability to preload values from environment variables or prior installations.
  It should minimize user input. For example, scale profiles for infra and orchestrator can be automatically applied according to user-specified target scale.
- A **well-defined abstraction between infrastructure and orchestrator** logic that enables independent testing and upgrading,
  as well as the ability to plug in new cloud providers via Go modules.
- Every module should be **toggled independently** and have minimal external dependency.
- Installer should support **upgrade** and **uninstall** from a previous version
- We should be able to run on-prem installer on the machine where EMF is being deployed.
  It should not require an aditional admin machine.
- Be compatible with upcoming Azure implementation and ongoing replacement of kind with on-prem in Coder

### Out of scope

- (EMF-3.2) A clear **progress visualization** showing the overall progress
- (EMF-3.2) **Diff previews** should be available during upgrade flows, showing schema migrations or configuration changes.
- (EMF-3.2) **Wrapped and actionable error messages**. Raw logs should be saved to files, and restarts should be possible from the point of failure.
- (EMF-3.2) Installer should support **orchestrator CLI integration** (e.g. `cli deploy aws`) and parallel execution of non-dependent tasks.
- Optimizing total deployment time, as current durations are acceptable.
- Full automation of post-deployment IAM/org/user configuration (users will be guided to complete this manually).

### Design Principles

#### General

- All operations must be idempotent and safe to retry.
- Unit tests will be written for each module, avoiding reliance on slow end-to-end tests.
- `pre` and `post` hooks will be supported in both the config builder and each installer stages (maybe even steps, TBD),
  which is useful for schema migration and backup/restore during upgrade.
- Maintain a better hierarchy of edge-manageability-framework top level folder
  - Nest `pod-configs`, `terraform`, `installer`, `on-prem-installer` under `installer`

#### Installer

- Once a configuration file is created, the installation should require no further user interaction.
- All shell scripts (e.g., `provision.sh`) will be replaced with Go code.
- Variable duplication across platforms will be eliminated using shared Terraform variable naming and outputs.
- Use of global variables and relative paths will be minimized.
- Developers should be able to modify, build, and run installer locally
- The same installer should be able to handle multiple deployments.
  The context of target cluster should be derived from the supplied user config.

#### Config Builder

- Configuration will be reduced to only the fields required for the selected environment.
- Full YAML config will be rendered for user review and advanced modification.
- Prior configurations can be loaded and migrated forward during upgrades.
- Schema validation will ensure correctness before proceeding.
- The default mode should be interactive, but it should also have an non-interactive mode
  - e.g. CI may want to invoke config builder to validate configuration before kicking off the deployment

#### Progress Visualization / Error Handling

- A text-based progress bar will display milestones, elapsed time, estimated remaining time, and current stage.
- Stage verification will occur both before (input validation) and after (desired state validation) each module runs.
- Logs will be saved to a file and only shown to users when necessary. The default view will focus on high-level progress and status.

### Installation Workflow

![Installer Workflow](./deploy-experience-improvement.svg)

#### Stage 0: Configuration

This stage involves collecting all necessary user input at once using the TUI config helper.
The configuration is stored as a single YAML file.

Input:

- Account info, region, cluster name, cert, etc.

Output:

- `User Config` – hierarchical YAML file used in all subsequent stages.

#### Stage 1: Infrastructure

Provisions the raw Kubernetes environment, storage backend, and load balancer.
The infrastructure module uses provider-specific backends (e.g., AWS, Azure, or on-prem), registered via Go interfaces.

Input:

- `User Config`
- `Runtime State` (e.g., generated network info)

Output:

- Raw Kubernetes environment
- Storage class
- Load balancer setup

#### Stage 2: Pre-Orchestrator

Performs setup that must be completed before Argo CD can take over.
This includes injecting secrets, setting up namespaces with required labels, importing TLS certs, and installing Gitea and Argo CD.

Input:

- Kubernetes cluster from Stage 1
- `User Config` (e.g., TLS certificates)
- `Runtime State` (e.g., database master password)

Output:

- Cluster in a ready state for Argo CD bootstrapping

Design Constraint:

- Infrastructure modules and orchestrator modules must remain decoupled.
  Only the installer mediates exchange of infra-specific info via:
  - Rendered Argo CD configuration or ConfigMap (for non-sensitive values like S3 URL)
  - Kubernetes secrets (for sensitive values like credentials)

#### Stage 3: Orchestrator Deployment

Deploys the Argo CD root app and monitors progress until all apps are synced and healthy.

Input:

- `User Config` (e.g., cluster name, target scale)
- `Runtime State` (e.g., S3 bucket name)

Output:

- All orchestrator Argo CD apps are synced and healthy
- DKAM completes the download and signing of OS profiles

#### Stage 4: Post-Orchestrator

Provides post-deployment guidance to users on setting up IAM roles, multi-tenant organizations, and user access.

Output:

- Display helpful links and CLI instructions
- (Next release: better integrated with orchestrator CLI)

### Implementation Details

- **Secrets Management:**
  Secrets required during installation runtime will be stored in memory or in a secure state file.
  Secrets needed post-deployment will be persisted as Kubernetes secrets.

- **Configuration and State Management:**
  Both `User Config` and `Runtime State` will be stored as a single structured YAML file,
  both persisted locally or in the cloud, similar to Terraform state files.
  These configurations will be versioned, enabling version specific upgrade logic such as configuration schema and/or data migration
  The config builder will support loading previous configurations, migrating them to the latest schema,
  and prompting for any new required attributes.
  We should leverage 3rd party libraries such as [Viper](https://github.com/spf13/viper) to handle configurations.

- **Configuration Consumption:**
  Each installer module will implement a config provider that parses the *User Config* and *Runtime State*, and generates module-specific configuration (e.g., Helm values, Terraform variables).

- **Upgrade Workflow:**
  During upgrade, the installer will generate a new configuration and display a diff preview to the user before proceeding.

- **Modular Infrastructure Provider Interface:**
  Infrastructure providers (AWS, Azure, On-Prem) will implement a shared Go interface and register themselves as plug-ins. This abstraction ensures separation from orchestrator logic and allows easy extension to new cloud backends.

- **Programmatic and orchestrator CLI Integration:**
  The installer must support both CLI usage (e.g., `cli deploy aws`) and programmatic invocation for integration with other tools like the Orch CLI.

- **Parallel Execution:**
  Dependencies between steps should be explicitly defined.
  Tasks that are independent will be executed in parallel to optimize installation time.
  This is a requirement for the standalone edge node mass provisioning.

- **Logging and Error Handling:**
  All logs will be dumped to a file automatically.
  Modules will return standardized error codes with consistent logging behavior across the system.

- **Output validation:**
  We should validate the output of each step and ensure the system is in the desired state before proceeding.
  The validation logic should be shared across different cloud provider implementations, ensuring a consistent behavior across different environments.

## Rationale

[A discussion of alternate approaches that have been considered and the trade
offs, advantages, and disadvantages of the chosen approach.]

## Affected components and Teams

- Foundational Platform Service
- CI/CD
- Documentation

## Implementation plan

- Design - interface between installer and modules, config format
- Design - Cloud Upgrade
- Design - On-Prem Upgrade
- Common - Implement installer framework and core logic
- Stage 0: interactive config helper
- Stage 1 - AWS - Reimplement as installer module
  - Implement Cloud upgrade from 3.0
- Stage 1 - On-Prem - Reimplement as installer module
  - Implement On-Prem update from 3.0
- Stage 2 - Implement common pre-orch jobs (cloud)
- Stage 2 - Implement common pre-orch jobs (onprem)
- Stage 3 - Monitor Argo CD deployment
- Nightly tests for Cloud upgrade
- Nightly tests for On-Prem upgrade
- Deployment Doc - Cloud deployment
- Deployment Doc - On-Prem deployment
- CI and release automation - installer binary

Required Resources: 7.5 FTE, 6 weeks (2 sprints)

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]
