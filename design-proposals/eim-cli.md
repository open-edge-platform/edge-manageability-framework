# Design Proposal: EIM CLI

Author(s): damiankopyto

Last updated: 30/04/2025

## Abstract

The Edge Orchestrator requires a management via a CLI tool, an overarching EMF CLI will be designed/implemented as per the Orchestrator CLI ADR (https://github.com/open-edge-platform/edge-manageability-framework/pull/246/files).
As part of this CLI a number of EIM specific actions/features/workflows should be supported.
This design proposal will focus on the details required for the EIM support.

## Proposal

This document proposes to include the EIM related functionality as subset of commands of the EMF CLI. The workflows called out in the following section are to be supported and represented through commands of the EMF CLI, the usability and functionality of the commands should be on par with the UI usage. The CLI will call the necessary APIs in order to execute the actions.

## Workflows

The following workflows are to be supported initially - the list may be expanded in the future if necessary functionality needs to be supported via CLI. It is assumed that the Bulk Import Tool [(BIT)](https://github.com/open-edge-platform/infra-core/tree/main/bulk-import-tools) will be integrated as part of the CLI effort.

> **ASSUMPTION/DISCLAIMER**: I did not delve into ~~BIT and~~ existing CLI yet before asking myself these questions.
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

**Local Options**

The local options associated with the `host` object/noun would help with filtering and providing arguments to the commands:

- `-o` `registered`/`onboarded`/`provisioned`/`all` - helps filter out the output of `list` command/verb ( TODO maybe instead of `-o` for output change to `-s` for status???)
- `--import-from-csv` `my-hosts.csv`
> **NOTE**: TODO - Team I am not yet convinced on using the register/onboard/provision flags as options to the create/delete objects/nouns rather than using as verbs directly - will welcome feedback
- `--register` `<arguments>` - local flag for the `create` command - enables only registering host during create - takes in argument in particular order necessary for registering the host (detail TBD)
- `--onboard` `<arguments>` - local flag for the `create` command - enables only onboarding of already registered host during create - takes in argument in particular order necessary for onboarding of the host (detail TBD)
- `--provision` `<arguments>` - local flag for the `create` command - enables only provisioning of already onboarded host during create - takes in argument in particular order necessary for provisioning of the host (detail TBD)
- `--auto-provision` - optional local flag that only takes effect when `--register` flag is invoked - enables auto provisioning of node when it connects
- `--auto-onboarding` - optional local flag that only takes effect when `--register` flag is invoked - enables auto onboarding of node when it connects
- `--deauthorize-only` - optional local flag that only takes effect when `delete` flag is invoked - does not delete host fully but only deauthorizes it
- `--force-delete` - optional local flag that only takes effect when `delete` flag is invoked - deletes host without deauthorization

**List  host information:**

The list command is responsible for listing all deployed hosts, in tabular format with the amount of information equivalent to what is present in UI: `Name`/`Host Status`/`Serial Number`/`Operating System`/`Site`/`Workload`/`Actions`/`UUID`/`Processor`/`Latest Updates`/`Trusted Compute`   
**TODO** this list should probably be scaled back by default and allow for expansion with `-o wide`? Looking for feedback.

- List Registered Host (ie. `emfctl list hosts -o registered`)
- List Onboarded Host (ie. `emfctl list hosts -o onboarded`)
- List Provisioned Host (ie. `emfctl list hosts -o provisioned`)
- List All Host (ie. `emfctl list hosts`) (the default behaviour)

**Get host information:**

The get command is responsible for describing the detailed information about a particular host.
The output is more verbose and includes information from the `list` with addition of the detail captured under `Status Details`/`Resources`/`Specifications`/`I/O Devices`/`OS Profile`/`Host Labels` - likely to be displayed as a mixture human readable sections and tabular? TODO get feedback on how aligned with UI we need this to be could be a table for each subsection.

- Get Host (ie. emfctl get host myhost)

**Bulk import of multiple Hosts**

The BIT tool allows for importing a number of hosts through CSV file, on import it registers, onboards and provisions the hosts - a hostname is autogenerated for each added host.
From the current understanding the tool does not support registration only or onboarding only, but it will support handling (onboarding/provisioning) of existing registered hosts if the hosts are added to CSV. With this assumption in mind the proposal is that CLI only uses bulk import for "fully provisioned" hosts (unless/until the BIT supports register only for bulk import).

The bulk import would be invoked with `--import-from-csv` option. Since with bulk import the host names are autogenerated the `host` command/noun would either have to take argument when the flag is provided, or the BIT tool would need to be expanded to accept a name that could be used as a prefix for auto generated host names.

**TODO** - Look for feedback - I did not give this much thought yet - how do we want to integrate the BIT tool - integrate it into the CLI codebase (then do we need CSV - could it be `yaml` instead) - or integrate to be used with CLI, installed as dependency and CLI calls BIT tool?

- Create multiple hosts (register/onboard/provision) (ie. `emfctl create host --import-from-csv my-hosts.csv`)

**Register Host:**

The create command is responsible for creation of the host within the Edge Orchestrator - the creation process is split into multiple stages - it is proposed to support the registration stage only with the `--register` option to be provided as part of the `host` object/noun followed by the arguments needed to successfully register the host.

*Required info: `Host Name`/`Serial Number`/`UUID`*  
*Optional info: `Auto Onboarding`/`Auto Provisioning`*

- Register Host (ie. `emfctl create host myhost --register <arguments>`) 

**Onboard Host:**

This option of the `create` command will support onboarding of any registered host using the `--onboard` option followed by arguments needed to successfully onboard a registered host.

- Onboard Host (ie. `emfctl create host myhost --onboard <arguments>`)

**Provision Host:**

This option of the `create` command will support provisioning of any onboarded host using the `--provision` option followed by arguments needed to successfully provision a registered host.

- Provision Host (ie. `emfctl create host myhost --provision <arguments>`)

> NOTE: Need to establish what the default behaviour should be for `create host myhost` without argument (in UI it is equivalent of `--provision <args>` + `--auto-onboarding`)

**Deauthorize Host:**

- Deauthorise Host (ie. `emfctl delete host myhost` `--deauthorize-only`)

**Deauthorize and Delete Host:**

- Deauthorise and Delete Host (ie. `emfctl delete host myhost`)

**Delete Host without deauthorize:**

- Delete Host (ie. `emfctl delete host myhost --force-delete`)

**Other:**

> **NOTE/QUESTION**: Editing Host, viewing metrics, scheduling maintenance are not considered to be supported in the initial phase of the EIM related CLI commands - should they be?

### Ability to create & manage Locations/regions/sub-regions/sites

**Local options**

The local options associated with the `location`/`region`/`sub-region`/`site` object/noun would help with filtering and providing arguments to the commands:

- `--region-type` - to be used with `create region/subregion`
- `--region` `<arg>` - region name/names to be used with sub-region
- `--latitude` - latitude for site
- `--longititude` - longtitude for site

> NOTE: TODO Do we need to support advanced region settings in this release?

**List locations/regions/sub-regions/sites**

> NOTE: TODO Not yet sure what is actually useful for the user to list vs how it's going to be used to edit and manage the regions. Is there a point in having different commands to list different levels of location where individually they do not contain all that much info if advanced settings are not enabled?

List locations will display the hierarchical tree of the location created

- List locations (ie. `emfctl list location`)

List regions only

- List all regions only (ie. `emfctl list region`)

List all sub-regions in a parent region in a form of a tree

- List all sub-regions in a parent region (ie. `emfctl list sub-region`)

List all sites in a parent region/subregion in a form of a tree

- List all sites in a parent region (ie. `emfctl list site`)

**Get regions/sub-regions/sites**

TODO - is there a point to all below if they will basically display same information - technically if I want to create a region/subregion/site I should be able to retrieve with same command but the value seems minimal???

Get region details including sub regions and sites below.

- Get region (ie. `emfctl get region myregion`)

Get sub region details including regions above/below and sites below

- Get sub-region ie (`emfctl get sub-region mysubregion --region myregion mysubregion mysubregionssubregion`)

Get site details including the regions/subregions above

- Get site details ie (`emfctl get site mysite --region myregion mysubregion`)

**Create location/regions/sub-region**

Create a new region

- Create region (ie. `emfctl create region myregion --region-type <type>`)

Create a new sub region in a region or another sub region

- Create region (ie. `emfctl create sub-region mysubregion --region-type <type> --region myregion <...>`)

Create a new site in a region/subregion or it's subregion

- Create site (ie. `emfctl create site mysite --longtitude 0 --latitude 0 --region myregion <...>`)

**Update region**

Update a region

- Update region (ie. `emfctl update region myregion --region-type <type>`)

Update a sub region in a region or another sub region

- Update region (ie. `emfctl update sub-region mysubregion --region-type <type> --region myregion <...>`)

Update a site in a region/subregion or it's subregion

- Update site (ie. `emfctl update site mysite --longtitude 0 --latitude 0 --region myregion <...>`)

**Delete location/region/sub-region**

Delete a region

- Delete region (ie. `emfctl delete region myregion --region-type <type>`)

Delete a sub region in a region or another sub region

- Delete region (ie. `emfctl delete sub-region mysubregion --region-type <type> --region myregion <...>`)

Delete a site in a region/subregion or it's subregion

- Delete site (ie. `emfctl delete site mysite --longtitude 0 --latitude 0 --region myregion <...>`)

### OS Profile/Provider management

> **NOTE**: The management of the OS profile and the Provider will be supported by CLI for testing scenarios and must be safeguarded from user error (it is generally not a set of features that are exposed to user and are managed internally)

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
