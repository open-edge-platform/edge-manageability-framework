# Design Proposal: Onboarding Deployment Packages from Github Repositories

Author(s): Scott Baker

Last updated: 2025-04-16

## Abstract

This proposal describes a mechanism for importing a deployment package from an opensource
application repository into the Orchestrator.

## Problem Statement

The convention chosen for opensource applications wishing to distribute their deployment packages
is that deployment packages will be placed in yaml format into well-known `deployment-package/` directories. This
convention has already been adopted for AI Suite applications at Intel
(ex: [PDD](https://github.com/open-edge-platform/edge-ai-suites/tree/main/manufacturing-ai-suite/pallet-defect-detection/deployment-package)),
and endorsed as part of the Edge Software Catalog (ESC).

Current mechanisem in 3.0 requires a user to clone the application repository and then import the deployment
package into the Orchestrator using their web browser. This process is complicated by the fact that many
repositories contain multiple deployment packages, and they are often located in subdirectories, and not
all AI Suite repositories use the same subdirectory naming convention.

This proposal seeks to eliminate the need to clone a `git` repo, navigate the `git` repo, and import. The use of `git`
is a skillset that casual users of the orchestrator may not have, and the complex nature of the evolving repositories
makes navigation difficult.

## Proposal 1: Tarball-based import

Note: The term `tarball` is defined as a `.tar.gz` archive that contains a set of individual files and/or directories,
packaged together into a single file.

Allowing a deployment package to be imported as a single tarball is
work that is already scoped for application orchestration and GUI. We could leverage this capability to simplify the
upload/download interaction between opensource repository and orchestrator. This will require some prep steps be done
to the opensource repo to make a tarball available.

Application owner preparation steps:

- The opensource application owner shall create a tarball of their yaml files, and check that
  archive into their opensource repository alongside the yaml source files. The application owner may wish
  to use CI to automate this process.

- The opensource application owner shall place a link to the tarball file into their documentation.

Orchestrator user steps:

- Follow the link in the application's documentation to download the tarball to their local computer.
  This is a simple operation that can be done with a web browser, and results in a single file being downloaded.

- Use their browser in the orchestrator's GUI to import the tarball from their local computer.

Although the user's computer is still used as a transient storage location for the VBU app, this proposal eliminates the
need to use complex tools such as `git` to clone repositories, and eliminates the need to navigate the repository's tree
to locate the proper deployment package.

## Proposal 2: Import a specific deployment package from a repository (requires more GUI work)

- The existing [Import Deployment Package](/edge-manage-docs/main/user_guide/package_software/import_deployment)
  screen shall be updated with an `Import from Github Repository` button.

- The user will paste the URL of the repo they wish to import into the GUI.

- The GUI will pass the URL to the application catalog service. For example, `/v3/projects/{project-name}/catalog/list_packages_from_repo?url=<url>`.

- The application catalog shall scan the repository and return a list of deployment packages that were found in the
  repository. After returning the list, the catalog service shall discard any transient state used to generate the list.

- The GUI will display the list of deployment packages that were found and allow the user to choose one.

- The GUI will pass the `URL` and `name` of the chosen deployment package to the catalog service
  for import, for example `/v3/projects/{project-name}/catalog/import_package_from_repo?url=<url>&package=<package>`.

- The application catalog service will extract the selected package from the repository and
  parse the yaml files there to extract a deployment package, and add that deployment package to the catalog.
  After loading the deployment package, the catalog service shall discard any transient state used to extract the DP.

- The GUI shall display a confirmation screen.

This proposal requires scanning repositories to find deployment packages and extracting
deployment packages from a repository. A naive approach is for the Catalog Service to internally clone
the repository to a temporary section of its local file system and use file operations to examine
the repository.

A more sophisticated approach may be use github REST APIs to examine the contents of a repository
without having to fully clone the repo. As the repositories may be large and there are only a few
relatively small yaml files that are of interest, this may be more efficient.

Note: The above URLs use query arguments for example illustrative purposes. The implementation may use POST
with a JSON payload instead. This is an implementation detail.

## Proposal 3: Import all deployment packages from a repository (requires less GUI work)

This is the same as the preceding proposal, but with a simplification that it eliminates the step required
to generate list of DPs and for the user to select a DP from the list.

- The existing `Import Deployment Package
  <https://docs.openedgeplatform.intel.com/edge-manage-docs/main/user_guide/package_software/import_deployment.html>`
  screen shall be updated with an `Import from Github Repository` button.

- The user will paste the URL of the repo they wish to import into the GUI.

- The GUI will pass the `URL` to the catalog service
  for import, for example `/v3/projects/{project-name}/catalog/import_package_from_repo?url=<url>`.

- The application catalog service shall extract all deployment packages from the repository and
  add all of them to the application catalog.

- The GUI shall display a confirmation screen.

## Rationale

Other approaches have been considered include:

- Requiring the user to clone the repo to their computer and import through their browser.
  This is the current solution. Requiring the user to clone to their computer requires additional
  tools and skills, consumes resources on their computer, and presents additional user experience
  burden.

- An API between Edge Software Catalog and Orchestrator could be created to allow one service to push to
  (or pull from) the other. This would involve additional complexity regarding authentication and
  entitlement from ESC to Orchestrator or vice-versa. It would require additional implementation effort
  from the ESC.

- The App Orch Tenant Controller (AOTC) is capable of installing deployment packages for extensions and could be used to
  install application deployment packages. However, the AOTC is a one-size-fits-all solution that bootstraps a project
  with a predefined set of packages. It is not designed to install new packages at runtime. There may be 50 or
  more AI applications with published deployment packages, and it would be undesirable to preload that many into
  the catalog at project creation time. Similarly, there may be a need to import deployment packages at any
  time.

## Affected components and Teams

- Application Orchestration Application Catalog.

- GUI.

- CLI.

## Implementation plan

### Proposal 1

Application Orchestration team shall modify the catalog service to accept tarball uploads and to extract the DP from them.

GUI team shall remove any impediments (if there are any) to uploading tarballs via the existing import command.

### Proposal 2 / 3

Application Orchestration team shall implement two APIs:

- `list_packages_from_repo`. Given a repository URL, clones the repo and returns a list of deployment packages
  that are inside it.

- `import_package_from_repo`. Give a repository URL and a package directory, clones the repo, extracts the
  deployment package, and adds it to the application catalog.

GUI team shall implement the changes to the GUI to allow URLs to be specified, to display the
list of deployment packages and to allow the user to select one, and to confirm the success or
failure of the operation.

If/when a CLI is committed to, Application Orchestration team shall update the CLI.

## Decision

The tentative decision is to implement Proposal 1 at this time. Proposal 1 mainly leverages existing planned work, and
nothing in Proposal 1 prevents taking up the other proposals at a future date.

## Open issues (if applicable)

An open issue is what to do in the case of closed-source repos that require authentication. This
proposal does not address that situation, which at this time is not considered a requirement.
