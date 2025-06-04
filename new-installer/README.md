# New Orchestrator Installer

## Overview

The New Orchestrator Installer is a modernized deployment tool for the Edge Manageability Framework (EMF)
introduced in version 3.1.
It replaces the previous monolithic shell script-based installers with a modular,
extensible Go-based system that provides improved error handling,
better user experience, and more consistent behavior across deployment targets.

## Key Features

- **Modular Architecture**: Clear separation between infrastructure provisioning and orchestrator setup
- **Multiple Environment Support**: Designed to support AWS and on-premises deployments with a unified codebase
- **Pluggable Infrastructure**: Easy to extend for new cloud providers
- **Idempotent Operations**: All steps are safe to retry
- **Interactive Configuration**: A text-based user interface (TUI) for guided setup
- **Complete Lifecycle Management**: Support for install, upgrade, and uninstall operations
- **Label-based Targeting**: Selectively run specific installation stages

## Architecture

For more detailed information about the design decisions behind this architecture,
see the [Deployment Experience Improvement](design-proposals/deployment-experience-improvement.md) design proposal.

## Build and Test

To build the installer and config builder:

```shell
mage -v NewInstaller:Build
```

To test the installer and config builder:

```shell
mage -v NewInstaller:Test
```
