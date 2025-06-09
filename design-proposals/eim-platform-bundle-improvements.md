# Design Proposal: Improvements on Platform Bundle integration with EIM

Author(s): Tomasz Osiński

Last updated: 14.05.2025

## Abstract

Platform Bundle is defined as a collection of OS-level files, installation scripts, configurations and packages.
It's a concept to make any OS profile supported by EIM extensible and customizable. For example,
we can have different flavors of Ubuntu/EMT images that support various use cases.
It's a mechanism that allows EN support features beyond what EIM supports currently.

Platform Bundle is already integrated with EIM and gives ability to provide custom cloud-init template
and Installer script (for Ubuntu).
For instance, the `Ubuntu with Intel extensions` OS profile uses Installer script that is curated
by EEF (Edge Enablement Framework) to install additional GPU drivers.

However, the current Platform Bundle design has the following problems:

1. Co-ownership of cloud-init template and Installer script between EIM and EEF,
   leading to de-synchronization between the two and error-prone manual processes to update
   Platform Bundle scripts any time cloud-init or Installer script is changed in the EIM codebase.
2. Currently, Platform Bundle is limited to cloud-init template and Installer script,
   while the original concept of Platform Bundle was to make it any OS-level extension.

This design proposal aims at:

- Define clear ownership of cloud-init template and Installer script between EIM and EEF,
  without impacting EEF certification process.
- Extend the Platform Bundle integration to enable providing custom OS-level files and configurations.

## Proposal

The proposal is inspired by two approaches from existing solutions:

- Azure and VM template with `customize` section
- VMware Tanzu BYOH with `OS bundle` concept

This design makes the following proposals:

**EIM:**

- Sets the sole ownership of EIM cloud init template to EIM codebase.
  It is NOT possible to overwrite EIM cloud-init template via the Platform Bundle,
  because EIM cloud-init is foundational piece that, if mis-configured, may make the entire EMF unfunctional.
- It is still possible to overwrite EIM Installer script for Ubuntu via the Platform Bundle.
  This may be needed if we want support other OS distros in the future.
- Extracts EIM cloud-init template to a separate sub-module of infra-onboarding.
  The new sub-module is versioned separately following SemVer.
  EIM cloud-init template is published to Release Service as version OCI artifact.
- OM imports EIM cloud-init template as versioned Go module.
- EIM extends the implementation of Platform Bundle to supply provisioned OS
  with arbitrary files defined as part of the Platform Bundle.
  It implies change of the platform bundle YAML format.
- `platformBundle` field of the OS resource should become R/W field from northbound API,
  so that external users can create their own custom OS profiles with arbitrary files packages.

**EEF:**

- EEF downloads EIM cloud-init template from Release Service for any EEF certification process.
  EIM cloud-init template is not curated by EEF.
- EEF should implement CI/CD to track any changes to Installer script and
  generate a new version of the curated Installer script if any change introduced.

### OS-level files as part of Platform Bundle

The proposal extends the definition of Platform Bundle to provide additional OS-level files as follows:

```yaml
platformBundle:
  files:
    - type: TARBALL
      artifact_type: OCI
      location: registry-rs.edgeorchestration.intel.com/edge-orch/en/files/platformbundle/standalone-edge-node:0.1.1
      target_path: /
    - type: RAW
      artifact_type: NON_OCI
      location: https://raw.githubusercontent.com/open-edge-platform/infra-onboarding/refs/heads/main/onboarding-manager/pkg/platformbundle/ubuntu-22.04/Installer
      target_path: /opt/intel_edge_node/Installer
```

The entire `platformBundle` field is converted to JSON as stored as `platform_bundle` field of OS resource.
Onboarding Manager is responsible for reading internal structure and generating Tinkerbell workflow accordingly.
There should be a new Tinker action that downloads files from RS and writes them to target OS location.

## Rationale

N/A

## Affected components and Teams

EIM - OS profiles, Onboarding Manager, Tinker actions

- Onboarding Manager:
  - Extract cloud-init template to a separate Go sub-module in infra-onboarding with CI/CD and SemVer versioning
  - Extend Platform Bundle library to handle new YAML format
  - Extend Tinkerbell workflows with action to download Platform Bundle files and write them to target OS file system.
- Tinker actions:
  - Implement new Tinker action that downloads Release Service artifacts (can be non-OCI to avoid implementing `oras`)
    and writes them to target OS location
- OS profiles:
  - Extend YAML of OS profiles that should be extended with Platform Bundle files

EEF framework

- Adapt EEF certification process to download EIM cloud-init template from Release Service
- Implement CI/CD to track Installer script changes and curate new Installer script for each EEF profile

Documentation

- Provide developer docs on Platform Bundle integration architecture
- Provide developer docs on how use Platform Bundle to customize OS profiles

## Implementation plan

EEF changes can be independent of EIM changes.

The main focus should be on OS-level files as part of Platform Bundle to support EMT-S scale provisioning use case.

### Phase 1 (target EMF 3.1)

The goal is to support EMT-S scale provisioning use case by extending Platform Bundle to provide tarball files.

### Post EMF 3.1

Implement full design proposal, including EEF workflows and support for other file types.

## Open issues (if applicable)

- It's still debatable if Installer script should be overridable.
  If overridable, it's still prone to errors due to sync of EIM-owned Installer and EEF-owned Installer.
  It also requires manual update of OS profiles for any new change.
