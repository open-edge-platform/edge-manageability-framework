# Design: Orch-CLI output refactoring and usability improvements

Author(s): Damian Kopyto
Last updated: 09/04/2026

## Abstract

The orch-cli is a tool that enables a user to interact with the Edge Orchestrator and manage the resources associated with it. As of release 2026.0 it provides functionality for most of the desired features in place within that release. While the neccessary functionality is in place and any new functionalities and mechanisms can be developed, it is crucial to further improve the user experience and versality of the tool as we are moving from UI usage to CLI first approach. A number of optimizations and improvement have been identified that can elevate both how the CLI is experienced as a tool to evaluate the EOM product, as well as simplify and consolidate the outputs of the CLI under the hood at code level.

## Goals

The following are the improvements we would like to achieve to improve the usability, readability and and the versality of the orch-cli:

- Consolidated and optimized output table formatter for all resource outputs
- YAML and JSON format outputs
- Single host creation without the bulk import
- Simplified input provision to easy the evaluation of the product

### Table formatting and custom outputs

Currently the outputs of the CLI commands to display individual features/resources follow the pattern of a tailored print functions specific to each feature/resource. While the pattern is simliar in almost all the feature/resource printounts the actual output may be inconsistent across these command outputs. Futhermore the outputs are pretty much preset and with exception of few individual, tailor made filters the CLI does not allow for display of custom set of fields, easy filtering or sorting of the output's fields/values.

The goal is to introduce a common table formatter which can be used to create outputs across most features/resources, the individual features/resources would have a default fields to use with the formatter and would enable custom override of the fields to display. As a stretch goal the formatter should accept filtering and sorting.

Advantages of such apporach include:

* Code cleanup and consolidation via reuse of the formatter to print outputs across most features/resources
* Cleaner more predictable outputs improve user experience
* Easliy customazible outputs allow better insights for the power users
* Sorting and filtering allows for faster and more precise insights into the deployment especially at scale

One thing to keep in minf in regards to filtering is that the EIM API already provides a degree of filtering and some feature commands allo using it, so there may need to be a distinction between the two types of filters.

A base for this implementation is already established through this experimental branch that contains an implementation of a formatter library. The same implementation can be tweaked and expanded or used as an inspiration to new code base see <https://github.com/open-edge-platform/orch-cli/compare/main...experiment/output-format>

### YAML and JSON output formats

The CLI currently lacks the functionality to display the outputs in a more machine readable way. One of the required output formats is YAML. For each feature/resource command a --output yaml flag shall be provided that will enable the display of data in this format. The major requirement would be to convert and display the data as is when it comes from the API. As a stretch goal consideration should be made if also these outputs can be customisable to display certain fields or sort or filter data.

### Single host creation

The CLI currently lacks the functionality to display the outputs in a more machine readable way. One of the required output formats is JSON. For each feature/resource command a --output JSON flag shall be provided that will enable the display of data in this format. The major requirement would be to display the data as is when it comes from the API. As a stretch goal consideration should be made if also these outputs can be customisable to display certain fields or sort or filter data.

### Simplifying user inputs

Two simplifaction are required to elevate the user experience:

1. Originally the CLI's tool host onboarding/creation was based and build as bulk import tool of Edge Nodes. Even the creation of a single host required to create a CSV file and create the host via bulk import. Introducing support for creation of a single host without using the bulk import using a single line command will silpify the user experience especially for someone wanting to quickly test or evaluate the EOM - therefore this simplfied usage of `creat host` command shall be implemented.
   
2. The EOM API/CLI has a limitation where resource names for certain resources are not unique and unique identification is done via the resource ID. This means that the user needs to figure out the resource ID and the provide it, which for qick evaluation purposes may be tiresome. As part of the imporovements to the CLI a possibility to input the resource by it's name should be made available if only single instance of this resource name exists, if multiple resources exist an error should be thrown - or as a stretch goal a suggestion window should appear suggesting the resource IDs that can be used. The major goal is to do this for site resource for single host creation (evaluate the feasibility of using resource name in bulk creation) - but a stretch goal is to expand to other resources and commands.

## CI/CD and unit test implications

The above changes will definately affect the unit tests which will need to be fixed accordnigly. Additionally the current CI/CD validation pipeline will need to be inspected to impacts due to the outputs changing and fixed as needed.

## Documentation

The documentation will need to be updated for these changes, the existing content may need to reflext new outputs. New content should eb added to reflect and explain the new output formats and how to use them. The changes to how single creation will now be possible should be documented along with notes around selecting resources by name where necessary.
