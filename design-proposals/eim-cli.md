# Design Proposal: EIM CLI

Author(s): damiankopyto

Last updated: 12/05/2025

## Abstract

The Edge Orchestrator requires a management via a CLI tool, an extensible EMF CLI will be designed/implemented as per the Orchestrator CLI ADR (<https://github.com/open-edge-platform/edge-manageability-framework/pull/246/files>).
As part of this CLI a number of EIM specific actions/features/workflows should be supported.
This design proposal will focus on the details required for the EIM support.

- **Goal** CLI tool should provide simple user experience for common workflows of EIM. Advanced workflows should be possible but only as an extended operation.
- **Define** The user experience of the CLI tool should align with industry-standard tools such as kubectl, providing a familiar and intuitive interface. Since the Edge Infrastructure Manager (EIM) is primarily intended for customers to manage fleets of edge nodes, the CLI should be optimized for fleet management as the default use case. Managing individual edge nodes should be treated as an exception rather than the norm.
- **Identify** common use cases and workflows.
  - Pre-registration of fleet of edge nodes (including Kubernetes)
  - Get the status of Fleet of edge nodes (in a specific region/site)
    - default should be ENs not assigned to region/site.
    - Treat region and sites as namespace `-n`.
    - `-A` should be all sites and regions.
    - `-o wide` should give status of pending Update and CVE status.
  - Update the all the edge nodes in the fleet that has pending Update (in a specific region/site)
    - Treat region and sites as namespace `-n`.
    - `-A` should be all sites and regions.
  - Update the all the edge nodes in the fleet that has pending CVE Update (in a specific region/site)
    - Treat region and sites as namespace `-n`.
    - `-A` should be all sites and regions.
  - De-register fleet of edge nodes matching a criteria (in a specific region/site)
    - Treat region and sites as namespace `-n`.
    - `-A` should be all sites and regions.
  - De-register a single edge node
  - Describe a specific edge node
  - Describe a OS profile/resource
- **Establish** command patterns, flags, and output formats.
- **Ensure** usability, consistency, and extensibility.

## Proposal

This document proposes to include the EIM related functionality as subset of commands of the (EMF CLI is a placeholder name) EMF CLI. The workflows called out in the following section are to be supported and represented through commands of the EMF CLI, the usability and functionality of the commands should be on par with the UI usage. The CLI will call the necessary APIs in order to execute the actions. The CLI may also be a vehicle for introducing new functionality before it gets reflected in the UI.

## Workflows

The following workflows are to be supported initially - the list may be expanded in the future if necessary functionality needs to be supported via CLI. It is assumed that the Bulk Import Tool [(BIT)](https://github.com/open-edge-platform/infra-core/tree/main/bulk-import-tools) will be integrated as part of the CLI effort.

> **NOTE**: The commands and the order of commands at this stage is in no way final - they are just placeholders until we agree on actual command structure overall.  
> **NOTE**: Not all the commands/functionalities and associated flags and option may be implemented in the initial version of the tool.

### Ability to register/onboard/provision a host

>**NOTE**: The host CRUD operations and integrating BIT are top priority in 3.1 release.

#### Local Options

The local options associated with the `host` object/noun would help with filtering and providing arguments to the commands:

- `--registered`/`onboarded`/`provisioned`/`all`/`deauthorized`/`unknown`/`error` - helps filter out the output of `list` command/verb
- `--import-from-csv` `my-hosts.csv` - used to provision a bulk of hosts using BIT
- `--auto-provision` - optional local flag that only takes effect when `--register` flag is invoked - enables auto provisioning of node when it connects
- `--auto-onboarding` - optional local flag that only takes effect when `--register` flag is invoked - enables auto onboarding of node when it connects
- `--edit-config` `<arguments>` - set of arguments to aid the edition of host
- `--cluster-template` `<argument>` - a flag accepting name of the template to be used for cluster deployment.

#### List  host information

The list command is responsible for listing all deployed hosts, in tabular format with the amount of information equivalent to what is present in UI: `Name`/`Host Status`/`Serial Number`/`Operating System`/`Site`/`Workload`/`Actions`/`UUID`/`Processor`/`Latest Updates`/`Trusted Compute`
> **NOTE** - the output will be scaled back by default and expanded with -o wide option.

- List Registered Host (ie. `emfctl list hosts -o registered`)
- List Onboarded Host (ie. `emfctl list hosts -o onboarded`)
- List Provisioned Host (ie. `emfctl list hosts -o provisioned`)
- List All Host (ie. `emfctl list hosts`) (the default behaviour)

#### Get host information

The get command is responsible for describing the detailed information about a particular host.
The output is more verbose and includes information from the `list` with addition of the detail captured under `Status Details`/`Resources`/`Specifications`/`I/O Devices`/`OS Profile`/`Host Labels` in UI - likely to be displayed as a mixture human readable sections and tabular? TODO get feedback on how aligned with UI we need this to be could be a table for each subsection.

- Get Host (ie. `emfctl get host myhost`)

#### Bulk import of multiple Hosts

The BIT tool allows for importing a number of hosts through CSV file, on import it registers, onboards and provisions the hosts - a hostname is autogenerated for each added host.
From the current understanding the tool does not support registration only or onboarding only (ie. the OSProfile is a mandatory field), but it will support handling (onboarding/provisioning) of existing registered hosts if the hosts are added to CSV. With this assumption in mind the proposal is that CLI only uses bulk import for "fully provisioned" hosts (unless/until the BIT supports register only for bulk import).

The bulk import would be invoked with `--import-from-csv` option. Since with bulk import the host names are autogenerated the `host` command/noun would either have to take argument when the flag is provided, or the BIT tool would need to be expanded to accept a name that could be used as a prefix for auto generated host names.

Additionally it is expected that the BIT will allow for a combination of provisioning host and deploying cluster using a specific template - this the `import host` command should accept a `--cluster-template` flag.

BIT will be integrated into the CLI.
BIT would be the de-facto command to onboard/provision an Edge Node or a number of Edge Nodes into the EMF.

- Create multiple hosts (register/onboard/provision) (ie. `emfctl import host --import-from-csv my-hosts.csv`)

#### Register Host

The register command is responsible for registration of a host within the Edge Orchestrator only.

*Required info: `Host Name`/`Serial Number`/`UUID`*  
*Optional info: `Auto Onboarding`/`Auto Provisioning`*

- Register Host (ie. `emfctl register host myhost)

#### Onboard Host

The `onboard` command is responsible for an onboarding of an already successfully registered host.

- Onboard Host (ie. `emfctl onboard host myhost`)

> **NOTE**: By default the auto-provisioning is off but if the auto-provisioning is set up in provider the onboarded host will provision automatically this should be communicated to user (either through CLI or documentation)

#### Provision Host

The provision command is responsible for provisioning an already onboarded node.

*Required info: `OSProfile`/`Site`/`Secure`/`RemoteUser`/`Metadata`*

- Provision Host (ie. `emfctl provision host my-host`)

#### Edit Host

Registered/Onboarded/Provisioned hosts can be edited

*Required info: `Name`/`Site`/`Region`/`Metadata`*

- Edit host (ie `emfctl edit host myhost --edit-config <arguments>`)

#### Deauthorize Host

- Deauthorise Host (ie. `emfctl deauthorize host myhost`)

#### Delete Host

- Delete Host (ie. `emfctl delete host myhost`)

> NOTE: This should require user confirmation

#### Other

> **NOTE/QUESTION**: Viewing metrics, scheduling maintenance are not considered to be supported in the initial phase of the EIM related CLI commands - should they be?

### Ability to create & manage Locations/regions/sub-regions/sites

> **NOTE**: The ability to manage locations is not a priority for 3.1 release.

#### Local options

The local options associated with the `location`/`region`/`sub-region`/`site` object/noun would help with filtering and providing arguments to the commands:

- `--region-type` - to be used with `create region/subregion`
- `--region` `<arg>` - region name/names to be used with sub-region
- `--latitude` - latitude for site
- `--longititude` - longtitude for site

> NOTE: TODO Do we need to support advanced region settings in this release?

#### List locations/regions/sub-regions/sites

> NOTE: TODO Not yet sure what is actually useful for the user to list vs how it's going to be used to edit and manage the regions. Is there a point in having different commands to list different levels of location where individually they do not contain all that much info if advanced settings are not enabled?

List locations will display the hierarchical tree of the location created

- List locations (ie. `emfctl list location`)
- List all regions only (ie. `emfctl list region`)
- List all sub-regions only (ie. `emfctl list sub-region`)
- List all sites only (ie. `emfctl list site`)

#### Get regions/sub-regions/sites

Get region details including sub regions and sites below.

- Get region including sub regions and sites below. (ie. `emfctl get region myregion`)
- Get sub-region including sub regions and sites below. (`emfctl get sub-region mysubregion --region myregion mysubregion mysubregionssubregion`)
- Get site details including the regions/subregions above (`emfctl get site mysite --region myregion mysubregion`)

#### Create location/regions/sub-region

Create a new location

- Create region (ie. `emfctl create region myregion --region-type <type>`)
- Create region sub region in a region or another sub region (ie. `emfctl create sub-region mysubregion --region-type <type> --region myregion <...>`)
- Create site in a region/subregion or it's subregion (ie. `emfctl create site mysite --longtitude 0 --latitude 0 --region myregion <...>`)

**Update region**

Update a location

- Update region (ie. `emfctl update region myregion --region-type <type>`)
- Update subregion  in a region or another sub region (ie. `emfctl update sub-region mysubregion --region-type <type> --region myregion <...>`)
- Update a site in a region/subregion or it's subregion (ie. `emfctl update site mysite --longtitude 0 --latitude 0 --region myregion <...>`)

**Delete location/region/sub-region**

Delete a location

- Delete region (ie. `emfctl delete region myregion --region-type <type>`)
- Delete a sub region in a region or another sub region (ie. `emfctl delete sub-region mysubregion --region-type <type> --region myregion <...>`)
- Delete a site in a region/subregion or it's subregion (ie. `emfctl delete site mysite  --region myregion <...>`)

### OS Profile/Provider management

>**NOTE**: The host OS Profile management is a priority for 3.1 release.  
>**NOTE**: The provider management is not a priority for 3.1 release.

#### List OS Profiles

Lists all available profiles in tabular format

- List profiles (ie. `emfctl list osprofile`)

#### Get OS Profile

Describes a particular OS profile in tabular format

- Get OS profile (ie. `emfctl get osprofile "Ubuntu 22.04.5 LTS"`)

> **NOTE**: The management of the OS profile and the Provider will be supported by CLI for testing scenarios and must be safeguarded from user error (it is generally not a set of features that are exposed to user and are managed internally)

#### Create OS Profile

Create OS profile takes an input from a file

- Create OS profile (ie. `emfctl create osprofile myprofile </path/to/profile>`)

#### Update OS Profile

> TODO not all fields in OS profile are updatable (really only ubuntu profile are updateable apart from name???)

- Update OS profile (ie. `emfctl update osprofile myprofile --<all relevant options>`)

#### Delete OS Profile

> TODO - Does the API prevent deletion of original profiles and profile in use? What about a scenario where user installs fresh orchestrator and deletes all profiles as first action? What sort of safeguards do we have in API vs what is needed in CLI?

- Delete OS profile (ie. `emfctl delete osprofile myprofile`)

#### List Provisioning Provider

- List provisioning provider (ie. `emfctl list provider`)

#### Get Provisioning Provider

- Get the details of the provider (ie. `emfctl get provider providername`)

#### Update Provisioning Provider

The update of provisioning provider is a combined create/delete action

- Update the Provisioning provider (ie. `emcfctl update provider providername --configuration <deafultOS/autoprovision/defaultLocalAccount/osSecurityFeatureEnable>`)

> TODO Are we planning to support any other provider than infra_onboarding? Is there a point in delete/create for now?

#### Create Provisioning Provider

- Create the Provisioning provider (ie. `emcfctl create provider providername --configuration <deafultOS/autoprovision/defaultLocalAccount/osSecurityFeatureEnable>`)

#### Delete Provisioning Provider

- Delete the provisioning provider (ie `emfctl delete provider providername`)

### Single click update

A single click update will be a feature that enables to run an update on a given edge node without scheduling maintenance (maintenance may be schedule under the hood but not part of user experience).

> **TODO/TBD** provide arguments that need to be associated with update via flag or from file.

- Update host (ie. `emcfctl run-maintenance host my-host`)

### Retrieve update version/history

In 3.1 it is expected that the day 2 updates and information about them will be tracked, a command to retrieve this information should be included in CLI.

> TODO/TBD figure out what a how info is tracked to display in user readable way at a later stage

- Update host (ie. `emcfctl audit host my-host`)

### Other

> QUESTION/TODO - What about CLI for remote ssh? do we need this in this release?  
> QUESTION/TODO - What about CLI for scheduling maintenance to node?  Yes - single click update.  
> QUESTION/TODO - What about CLI for scheduling maintenance to site?  
> QUESTION/TODO - What about CLI for vPRO? - At some point?  
> QUESTION/TODO - What about CLI for per Edge Node config? - At some point?  

## Rationale

The rationale of this feature is a common sense approach. Any CLI tool designed to support the Edge Orchestrator should provide a user with a simple way of managing the Edge entities such as profiles/locations/host from a local command line.

## Affected components and Teams

The EMF CLI and teams working on it.

## Implementation plan

The EIM functionality will be contributed into the existing Catalog CLI tool, the EIM team will focus on EIM specific functionality and provide feedback for improvements to overall CLI experience where needed.

Once the design for the EMI portion has been agreed internally within the EIM team, and agreed with other teams in terms of integration with overall EMF CLI the EIM features will be added in one at a time. The commands/workflows will be implemented adhering to the original design for EMF CLI.

3.1 features:
- Basic Host CRUD + BIT integration
- Basic OS Profile CRUD
- Single Click update
- Audit Day2 versions

Beyond 3.1:
- Provider management
- Improvement and complex combinations for CRUD operations
- Location management
- vPro support
- Per EN configuration
- Scheduled Maintenance

## Open issues (if applicable)
