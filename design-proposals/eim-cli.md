# Design Proposal: EIM CLI

Author(s): damiankopyto

Last updated: 29/04/2025

## Abstract

The Edge Orchestrator requires a management via a CLI tool, an overarching EMF CLI will be designed/implemented as per the ADR [TODO](ADR_URL).
As part of this CLI a number of EIM specific actions/features/workflows should be supported.
This design proposal will focus on the details required for the EIM support.

## Proposal

This document proposes to include the EIM related functionality as subset of commands of the EMF CLI. The workflows called out in the following section are to be supported and represented through commands of the EMF CLI, the usability and functionality of the commands should be on par with the UI usage.

## Workflows

The following workflows are to be supported initially - the list may be expanded in the future if necessary functionality needs to be supported via CLI. It is assumed that the Bulk Import Tool [(BIT)](https://github.com/open-edge-platform/infra-core/tree/main/bulk-import-tools) will be integrated as part of the CLI effort.

> **ASSUMPTION/DISCLAIMER**: I did not delve into BIT and existing CLI yet before asking myself these questions.
> 
> **QUESTION**: As evident below some of the command interactions will require a lot of information for example registering a host - what approach should be taken within CLI  
> 1. Long command? It's tiresome ie.  
> `emfctl eim register host myhost ABCDEFG 12345678 --autoonboard --autoprovision`  
> 2. Interactive prompt? It would not allow to register a bulk of hosts &#128579; . Does that fit into the overarching design (don't know yet did not have chance to explore &#128579; )  
> `emfctl eim register host myhost`  
> `enter serial:`  
> `ABCDEFG`  
> `enter UUID`  
> `12345678`  
> `do you want to auto onboard the host?`  
> `yes`  
> `do you want to auto provision the host?`  
> `no`
> 3. From yaml?  
> `emfctl eim register host -f - <<EOF`  
> `hostname: myhost`  
> `serial: ABCDEFG`  
> `uuid: 12345678`  
> `autoonboard: true`  
> `autoprovision: false`  
> `EOF`
>
> **NOTE**: The commands and the order of commands at this stage is in no way final - they are just placeholders until we agree on actual command structure overall.

### Ability to register/onboard/provision a host

**List the host information:**

- GET Registered Host (ie. emfctl eim get hosts???)
- GET Onboarded Host
- GET Provisioned Host
- GET All Host

**Register Host:**

*Required info: `Host Name`/`Serial Number`/`UUID`*  
*Optional info: `Auto Onboarding`/`Auto Provisioning`*

- REGISTER Host (ie. `emfctl eim register host \<hostname\> \<someinfo\>`) (TODO actual commands across all functions)

**Onboard Host:**

- ONBOARD Host (ie. `emfctl eim onboard host \<hostname\>`)

**Provision Host:**

- PROVISION Host (ie. `emfctl eim provision host \<hostname\>`)

**Deauthorize Host:**

- DEAUTHORIZE Host (ie. `emfctl eim deauthorize host \<hostname\>`)

**Delete Host:**

> **NOTE**: `--force-delete` delete flag should be supported otherwise hosts that are not deauthorized should not be allowed to delete??

- DELETE Host (ie. `emfctl eim delete host \<hostname\>`)

**Other:**

> **NOTE/QUESTION**: Editing Host, viewing metrics, scheduling maintenance are not considered to be supported in the initial phase of the EIM related CLI commands - should they be?

### Ability to create & manage Locations/regions/sub-regions etc


### OS Profile/Provider management

> **NOTE**: The management of the OS profile and the Provider will be supported by CLI for testing scenarios and must be safeguarded from user error (it is generally no a set of features that are exposed to user and are managed internally)

TODO

## Rationale

The rationale of this feature is a common sense approach. Any CLI tool designed to support the Edge Orchestrator should provide a user with a simple way of managing the Edge entities such as profiles/locations/host from a local command line.

## Affected components and Teams

The EMF CLI and teams working on it.

## Implementation plan

The App Orchestration teams takes a lead on the overarching design/rules for the EMF CLI since ground work has already been done via the Catalogue CLI.

Once the design for the EMI portion has been agreed internally within the EIM team, and agreed with other teams in terms of integration with overall EMF CLI the EIM features will be added in one at a time. The commands/workflows will be implemented adhering to the original design for EMF CLI.

## Open issues (if applicable)

EMF CLI final design still in progress.
