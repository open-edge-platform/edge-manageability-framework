# Design Proposal: Importing Helm Charts into the Orchestrator

Author(s): Scott Baker

Last updated: 2025-05-12

## Abstract

The purpose of application orchestration is to deploy helm-based applications. Adoption success depends on being
able to quickly make helm-based applications available in the Orchestrator. Once available, the application can
be iterated on inside the Orchestrator to add additional Orchestrator value such as use of `Profiles` or
`Parameter Templates`. Once that additional value is added, the developer or operator may want to export their
work so it can be shared with other orchestrators or distributed to customers.

## Proposal

The proposed workflow breaks down into three phases.

- Helm Chart Import. Given a Helm Chart, automatically extract the necessary data from the Helm Chart to allow
  it to be added to the Orchestator's Catalog. In the Orchestrator, this has traditionally been called a
  `Deployment Package`.

- Orchestrator Value Add. The user adds additional Orchestrator features such as Profiles (canned/curated values
  for deploying charts), Parameter Templates (values to prompt at deployment time), Service Links, etc. Value
  Add is an optional step.

- Deployment Package Export. Once the user has added value to their `Deployment Package`, they may wish to
  be able to easily share that deployment package with other orchestrators. They should be able to export
  that `Deployment Package` in a format where it may be easily shared, published, or version controlled in
  github.

NOTE: We use the term `Deployment Package` to represent the set of objects that describe a Helm-based
application in the Orchestrator. In 3.0 this is comprised of three objects -- `Deployment Package`,
`Application`, and `Registry`. There is a separate proposal to reduce this to fewer objects. In this
proposal we not distinguish the boundaries between these objects.

### Helm Chart Import

The first phase is Helm Chart Import. We need to collect several pieces of information from the user:

- `Helm Chart OCI URL`. We assume the user has published their Helm Chart to an OCI Registry. This
  could be public sources such as Dockerhub or Bitnami, or a private source, such as a corporate
  Helm Registry. To import the Helm Chart, we need the URL to fetch it from.

- `Username` / `Password`. For private helm chart registries, we will need the username and password
  (or personal access token). For public registries like Dockerhub, a PAT may be used to prevent
  throttling.

- `Initial Values`. This is optional. The user may have an initial set of values they want to override
  when deploying. If the user chooses to supply these, then they will be used to populate a default
  profile. If not, then the default profile will be left empty. Some helm charts will install fine
  with an empty profile -- what values may need to be overridden depends on the Helm Chart.

- `Additional Flags`. They may be need for additional flags for the user to specify preference. For
  example, whether or not to include the `username` and `password` in the generated
  `Deployment Package`. We could opt for simplicity and assume the most common setting for these flags
  rather than exposing them to the user.

There are two options where this functionality can be implemented:

- `Command Line Tool`. A command line tool has already been written where the user specifies
  the necessary information (URL, optional username and password, etc) as command line arguments
  to the tool. In response the tool downloads the Helm Chart, and then generates the `Deployment Package`
  as a set of yaml files. The yaml files may then be imported to the Orchestrator by using
  the user's browser.

- `GUI Import`. The necessary information (URL, optional username and password, etc) could be collected
  via a page in the Orchestrator GUI, implemented as a series of form fields. The GUI sends the
  information to the backend where the backend will perform the same series of steps that the Command
  Line Tool would have done, and then automatically add it to the Orchestrator's Catalog.

These two options are not mutually exclusive. The `Command Line Tool` can be used for those personas
that prefer to work with the Deployment Package in yaml form. The `GUI Import` can be used for those
personas who prefer to use graphical tools. It also provides an entry point for non-technical users
who merely wish to import a Helm Chart in the most expedient way possible.

### Orchestrator Value Add

This is already implemented via a set of pages in the GUI. Once the basic `Deployment Package` is
imported, the user may make use of existing pages in the GUI to add profiles, parameter templates, and
other features to their `Deployment Package`. These workflows are already documented in the
Orchestrator User Guide. The `Helm Chart Import` provides a starting point to this optional value
add.

### Export

The final step in the workflow is to export any work that the user has done, so that it may be shared
in other orchestrators. The export will be in a `.tar.gz` file that contains the `yaml` representation
of the `Deployment Package`. Exporting this as a `.tar.gz` file is convenient to do in the user's browser
as it allows a single file to be downloaded.

As with import, there are two options

- `Command Line Tool`. The CLI could be used to reach out to the Orchestrator, download the necessary
  parts of the Deployment Package, and emit them locally as either a `.tar.gz` file or as a set of
  individual yaml files.

- `GUI Export`. A button could be added to the GUI that triggers the export by reaching out to the
  backend to generate a `.tar.gz` file, which is then downloaded via the users browser.

These two options are not mutually exclusive -- we can do both.

## Rationale

We considered both CLI and GUI approaches. CLI has the advantage that it's usable by people who do not
have access to an orchestrator, and can be used to craft `Deployment Package` that could be handed
off to another team for evaluation and use. GUI has the advantage requires a lower knowledge set and
facilitates a quick path toward bringing in Helm Charts directly.

The GUI is the more user-facing and more visible component, and we propose prioritizing GUI over CLI,
but also that we should strive to do both.

## Affected components and Teams

- Application Orchestration Application Catalog.

- GUI.

- CLI.

## Implementation plan

- App Orch. Implement Helm-to-DP command line tool, generating yaml files in a local directory.

- App Orch. Extend Catalog Service Import to allow `.tar.gz` files to be imported.

- App Orch. Implement Catalog Service API endpoint to invoke Helm-to-DP based on REST API. This
  API will block while the Helm Chart is being fetched, and return an error if the fetch is
  unsuccessful or times out.

- GUI. Implement UI page for accepting OCI URL, Username/Password, and initial values. It will call
  the REST API endpoint in the Catalog Service and return a green success box if successful, or
  the error if unsuccessful.

- App Orch. Implement Catalog Service API endpoint to export DP package and generate a tarball for
  download.

- GUI. Implement UI button or link for exporting deployment package.

## Open issues (if applicable) / Feature Requests

- Automatically adding the chart's values.yaml as the initial profile if the user did not provide their
  own values.yaml when importing. This is a feature request from internal customers. The only disadvantage
  is that some charts have a very large and verbose values.yaml. Importing it into the default Profile will
  lead to a large and verbose profile, that is identical to the values.yaml in the helm chart. For charts
  with smaller values.yaml this could make a lot of sense as a starting point.

- Automatically create parameter templates from all values.yaml profile. This also has the disadvantage
  that a large chart could produce a large quantity of parameter templates.

## Decision

Implement GUI integration and Command Line Tool, pending GUI resource availability. Add the two feature
requests above to the CLI tool, and consider adding advanced options to enable them on the GUI page, if
GUI design to incorporate advanced options is feasible.
