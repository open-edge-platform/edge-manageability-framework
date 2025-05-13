# Design Proposal: Command Line Interface for Cluster Orchestration components

Author(s): Julia Okuniewska

Last updated: 2025-05-13

## Abstract

This ADR describes the design of nouns supported by cluster-orchestration
in command line interface (CLI).

This document is complementary to general CLI design:
https://github.com/open-edge-platform/edge-manageability-framework/pull/246.

## Proposal

The CLI follows a `verb - noun - subject` pattern.

The target for cluster-orchestration part in CLI is to be able to:
- create/list/delete clustertemplates
- mark clustertemplate as default one
- create cluster with given hosts and given template (or pick default template if not specified)
- get specified cluster
- delete/force-delete specified cluster
- download edgenode's kubeconfig

### Nouns
- `clustertemplate`
- `cluster`
- `en-kubeconfig`

#### Syntax
Assumption: All CLI commands should include a `--project` or `-p` parameter.
For simplicity, this parameter is omitted in the commands provided below.

##### clustertemplate
CRUD operations for clustertemplate:
```bash
cli list clustertemplates
cli list clustertemplates --default
cli get clustertemplate <name> --version <version>
cli get clustertemplate versions <name>
cli apply clustertemplate --file <path_to_clustertemplate.yaml>
cli delete clustertemplate <name> --version <version>
```
Cluster template does not support set/update operation.
It is immutable.

Extra operations related to clustertemplate:
```bash
cli set-default clustertemplate <name> --version <version>
```
or if `--version` parameter is not passed, take the latest version for given name.

##### cluster
CRUD operations for cluster:
```bash
cli list clusters
cli list clusters --summary
cli get cluster <name>
cli get cluster details <nodeId>
cli apply cluster --file <path_to_cluster.yaml>
cli create cluster \
    --name <name> \
    --hosts 6e6422c3-625e-507a-bc8a-bd2330e07e7e:all \ # required in format <uuid:role>
    --clusterLabels key:value \ # optional
    --template <template name-version> # optional
cli delete cluster <name>
cli delete cluster <name> --force
cli set/update cluster --clusterLabels key:value, key2:value2

```

##### en-kubeconfig
operations for edgenode's kubeconfig
```bash
cli get en-kubeconfig <cluster name> --output <path_to_save_location>
```

## Rationale
The justification of this cli is to simplify user interaction with the API.

## Affected components and Teams

Cluster Orchestration Team

## Implementation plan
The CLI is to be implemented in the `go` programming language, using the popular `cobra` go library.

## Out of scope Features
- hardware information, currently retreivable via EIM CLI
- add another node to cluster, currently not supported feature in cluster-orchestration.

## Open issues (if applicable)

- should we support interactive cluster template creation
    or should cluster template be created only from file (-f flag)?
    Clustertemplate's clusterConfig is a long string with a lot of details,
    so passing this option in interactive mode might be problematic.
