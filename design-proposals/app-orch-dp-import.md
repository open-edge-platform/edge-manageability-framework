# Design Proposal: Onboarding Deployment Packages from Github Repositories

Author(s): Scott Baker

Last updated: 2025-04-16

## Abstract

This proposal describes a mechanism for importing a deployment package from an opensource
application repository into the Orchestrator.

## Proposal

The convention chosen for opensource applications wishing to distribute their deployment packages
is that deployment packages will be placed in yaml format into well-known `deployment-package/` directories. This
convention has already been adopted for AI Suite applications at Intel, and endorsed as part of the
Edge Software Catalog (ESC).

Current mechanisem in 3.0 requires a user to clone the application repository and then import the deployment
package into the Orchestrator using their web browser. This proposal seeks to eliminate the need
to clone and import.

This proposal is complicated by the fact that many repositories contain multiple deployment
packages, and they are often located in subdirectories, and not all AI Suite repositories use the
same subdirectory naming convention.

There are two variants of this proposal:

### Sub-Proposal - Import a specific deployment package from a repository (requires more GUI work)

- The existing `Import Deployment Package` screen shall be updated with an `Import from URL` button.

- The user will paste the URL of the repo they wish to import into the GUI.

- The GUI will pass the URL to the application catalog service. For example, `/v3/projects/{project-name}/catalog/list_packages_from_repo?url=<url>`.

- The application catalog shall scan the repository and return a list of deployment packages that were found in the
  repository.

- The GUI will display the list of deployment packages that were found and allow the user to choose one.

- The GUI will pass the `URL` and `name` of the chosen deployment package to the catalog service
  for import, for example `/v3/projects/{project-name}/catalog/import_package_from_repo?url=<url>&package=<package>`.

- The application catalog service will extract the selected package from the repository and
  parse the yaml files there to extract a deployment package, and add that deployment package to the catalog.

- The GUI shall display a confirmation screen.

### Sub-Proposal - Import all deployment packages from a repository (requires less GUI work)

- The existing `Import Deployment Package` screen shall be updated with an `Import from URL` button.

- The user will paste the URL of the repo they wish to import into the GUI.

- The GUI will pass the `URL` to the catalog service
  for import, for example `/v3/projects/{project-name}/catalog/import_package_from_repo?url=<url>`.

- The application catalog service shall extract all deployment packages from the repository and
  add all of them to the application catalog.

- The GUI shall display a confirmation screen.

Note: The above URLs use query arguments for example illustrative purposes. The implementation may use POST
with a JSON payload instead. This is an implementation detail.

### Scanning and extracting deployment packages from github repositories

The above proposals mention scanning repositories to find deployment packages and extracting
deployment packages from a repository. A naive approach is for the Catalog Service to internally clone
the repository to a temporary section of its local file system and use file operations to examine
the repository.

A more sophisticated approach may be use github REST APIs to examine the contents of a repository
without having to fully clone the repo. As the repositories may be large and there are only a few
relatively small yaml files that are of interest, this may be more efficient.

The specific choice of method is left as an implementation decision.

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

## Affected components and Teams

- Application Orchestration Application Catalog.

- GUI.

- CLI.

## Implementation plan

Application Orchestration team shall implement two APIs:

- `list_packages_from_repo`. Given a repository URL, clones the repo and returns a list of deployment packages
  that are inside it.

- `import_package_from_repo`. Give a repository URL and a package directory, clones the repo, extracts the
  deployment package, and adds it to the application catalog.

GUI team shall implement the changes to the GUI to allow URLs to be specified, to display the
list of deployment packages and to allow the user to select one, and to confirm the success or
failure of the operation.

If/when a CLI is committed to, Application Orchestration team shall update the CLI.

## Open issues (if applicable)

An open issue is what to do in the case of closed-source repos that require authentication. This
proposal does not address that situation, which at this time is not considered a requirement.
