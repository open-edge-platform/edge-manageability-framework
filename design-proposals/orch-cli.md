# Design Proposal: Orchestrator Command Line Interface

Author(s): Scott Baker

Last updated: 2025-04-29

## Abstract

This ADR describes the design of a command line interface (CLI) for the
Orchestrator. The CLI is a cross-component tool that may be used from a Linux, Windows, or
Mac environment, such as from the administrator's laptop. The CLI may also may be leveraged
as a building block in other environments such as CI/CD or testing.

The CLI described here uses the `Catalog CLI` as a starting point and inspiration. However, this
ADR assumes that syntax and semantics of the CLI may diverge from the existing `Catalog CLI` and
this ADR is not constrained by that existing implementation. As such, examples given in this
ADR may not necessarily work with the existing catalog CLI.

## Proposal

The CLI implements a set of verbs, a set of nouns, and a set of common options that may
be used to interact with the orchestrator.

The CLI is not intended to be an automation framework in and of itself. It is intended to support
a set of relatively tightly scoped operations, such as adding and removing specific objects from
the orchestrator. The CLI may be leveraged by automation tools that might perform more complex
sets of steps. In this respect the CLI and the `mage targets` in EMF have different purpose as
the mage targets implement multi-step workflows, end-to-end tests, etc.

The CLI follows a `verb - noun - subject` pattern. For example,

- `cli get application nginx`. The verb is `get`. The noun is `application` and the subject
  is `nginx`. This particular CLI command would return the application called nginx from the
  Orchestrator and print them to the console in a human-readable format.

The name of the CLI presented in this documents has been generalized to `cli`. Proposed names include
`orchctl`, `emfctl`, `orch`, or `oc`.

### Syntax

This section describes the syntax of the CLI.

#### Verbs

Note that some verbs may have synonyms. For example, `set` and `update` are synonyms for the same
verb.

##### Session Management

- `login <user-id>` ... log in to the Orchestrator. The CLI shall prompt for a password and retrieve
  a refresh token. The refresh token is cached locally until it is expired or a `logout` verb is used.

- `logout` ... log out of the Orchestrator. Any active login state, including the refresh token, is
  discarded.

##### CRUD Operations

- `list` ... lists objects on the Orchestrator. The `list` verb produces a list of all objects
  on the Orchestrator, with an optional filter. List should return tabular summary-level information
  about the objects.

- `get` / `describe` ... retrieves objects from the Orchestrator. `get` is for retrieving single objects whereas
  `list` is for retrieving multiple objects. `get` should retrieve more verbose information about the
  object than `list`.

- `create` ... creates an object on the Orchestrator. Additional noun-specific parameters are used to
  describe the properties of the object to be created.

- `delete` ... deletes an object from the Orchestrator.

- `set` / `update` ... modifies one or more values in an existing object on the Orchestrator.

- `apply` ... create or modify an object from a yaml or json specification, from stdin or from a local
  file.

##### Configuration and utility

- Configuration of the CLI tool. This should be treated no differently than any other verb/noun
  combination using `get` and `update`. i.e. `cli update config --endpoint https://my-orchestrator.com/`.
  NOTE: Current implementation uses a special-purpose syntax,
  `cli config set endpoint https://my-orchestrator.com/`, to be revised.

- Code Completion. Use `get completion bash`.
  NOTE: Current implementation uses a special-purpose syntax, to be revised.

- `upload <directory>` ... upload items to the Orchestrator. This is currently a catalog-specific
  operation and uploads a directory of yaml files to the orchestrator.

- `version` ... return the version number of the CLI. We should also consider implementing an API on
  the orchestrator to allow querying the version of the orchestrator, so orch version and be reported
  alongside client version. Remote API is TBD.

#### Nouns

This ADR does not seek to exhaustively list the set of nouns that are available on the CLI. Each
component shall add their own nouns as appropriate. For example,

- `cli list applications` ... The `application` noun applies to applications in the catalog.

- `cli list registry` ... The `registry` noun applies to registries in the catalog.

#### Subjects

Subjects are additional information or context related to a noun. Different nouns may support
different subjects. For convenience sake, most nouns support a name subject and optionally for those
objects that are versioned, a version subject. For example,

- `cli get application nginx` ... returns the first nginx application, agnostic to the version.

- `cli get application nginx 0.0.1` ... return exactly version 0.0.1 of the nginx application.

If additional subjects beyond name and version are required, then they should be specified with additional
filters. For example,

- `cli get application nginx --deployed true` ... get the nginx application that is currently deployed.

#### Global Options

- `-h` / `--help` ... return help. The help returned may be contextualized to the verb or noun that is
  being used. For example `cli -h` returns global help whereas `cli create application -h` return help
  that is relevant to creating applications.

- `-n` / `--noauth` ... disables authentication checks, only useful in a development environment.

- `-p` / `--project` ... sets the project that will be used for the current verb. Required for verbs that
  act on projects. The CLI configuration shall allow a default project to be specified, so that `-p` does
  not need to be repeated for each command.

- `-v` / `--verbose` ... enables verbose output.

- `-vv` / `--veryverbose` ... enables even more verbose output, such as the printing of API calls.

- `-o json` / `-o yaml` ... instead of returning human-readable output, return output in `yaml` or `json`.

#### Verb-Specific and/or Noun-Specific options

Verbs and nouns may have additional non-global options as necessary. For example, the `create application`
verb/noun pair includes the options `--chart-name` and `--chart-registry` and `--chart-version`.

#### Examples

- `cli login sample-project-admin` ... log in as a user.

- `cli -p acme list applications nginx` ... list all nginx applications in a tabular format.

- `cli -p acme list applications nginx -o yaml` ... list all nginx applications and emit the output as yaml.

- `cli -p acme create application nginx 0.0.1 --chart-name nginx --chart-registry bitnami --chart-version 0.0.1`
  ... create the nginx application using the specified parameters.

- `cli -p acme delete application nginx 0.0.1` ... delete the nginx application.

- `cli -p acme update application nginx 0.0.1 --chart-registry dockerhub` ... change the chart-registry for an
  application.

- `cli -p acme apply -f nginx.yaml` ... create the objects contained inside nginx.yaml.

- `cli logout` ... log out.

### Miscellaneous Guidelines

- The CLI seeks to minimize the boundaries between teams and components. For example, the following approaches
  are not desirable:

  - `cli update app-orch application foo` ... app-orch is a team/subsystem designator that is not relevant to
    the user.

  - `cli update catalog application foo` ... catalog is a component name. It is not relevant to the user that
    applications are managed by the catalog service.

- The CLI should avoid distinguishing between singular and plural nouns. For example `list applications`
  and `list application` should be equivalent commands.

- The CLI stores its local state in the file (in a Linux environment) `~/.orch-cli/orch-cli.yaml`. This state
  includes the orchestrator endpoint, keycloak endpoint, default projects, logged in username, and logged in
  refresh token. This file should have permissions set appropriately.

- The CLI generally uses the single `api.` endpoint for the orchestrator. There are occasionally places where
  it may infer the names of additional endpoints. For example, during login it will infer the `keycloak.` endpoint
  by taking the `api.` endpoint and substituting `keylocak` for `api`. This may be done with other endpoints
  as necessary.

- Project, org, and user management is treated like any other nouns in this proposal, and the specifics of
  project management should be determined in their own ADR. For example, `cli list projects` and
  `cli create project` are plausible verb/noun combinations for project management.

## Rationale

A fully autogenerated approach was discussed, but we feel this would too closely tie the CLI to the syntax
and structure of the API. The CLI is meant to abstract the operations of the API into a more human-consumable
format.

- There may be times when a particular CLI operation leads to multiple API calls.

- Related objects may be internally resolved by the CLI as necessary. For example, a user might link objects A
  and B together by name, even if the API represents the link by an internal identifiers or uuids.

- If there are future breaking changes to the API, then the CLI may seek to insulate the user from those changes.

- Additional context or explanation may be provided in the CLI, such as returning objects in a human-readable format.

- The CLI may contain additional guardrails to "fail early" or to fail in a human readable manner when the user
  performs an incorrect operation.

- The CLI may seek to hide or mask sensitive information such as tokens or passwords.

## Affected components and Teams

Application Orchestration, Cluster Orchestration, EIM, Platform.

## Implementation plan

The CLI is to be implemented in the `go` programming language, using the popular `cobra` and `viper`
go libraries.

The CLI shall include a Makefile and github actions that facilitate building it for Linux, Windows, and
Mac.

### Code structure

The CLI shall be implemented in a modular way.

Each noun shall be placed in a file `<noun>.go` in the `internal/cli` directory. For example,

- `internal/cli/application.go` ... implements the noun `application` and its associated verbs.

TO-DO: Here is may make sense to separate code by subsystem for easier maintainability. For example,
`internal/cli/app/application.go`, `internal/cli/cluster/cluster-templates.go`, etc. Discuss.

## Distribution

- Distribution of Binaries. Proposal is to make them a github release, so they may be easily
  downloaded by an end-user. Additional scanning or signing of binaries may be necessary and this
  will need to be integrated into CI/CD.

## Open issues (if applicable)

- Session / Context management. It would be convenient if the CLI could store the context for
  multiple orchestrator sessions and then easily switch between them.

- Filtering and Naming. This proposal adopts the Catalog convention where most objects are named by
  their `name` and `version`. Other systems, such as EIM may have more complex naming to consider
  such as naming by `uuid` or `id` in addition to `name`.
