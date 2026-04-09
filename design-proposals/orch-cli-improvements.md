# Design: Orch-CLI output refactoring and usability improvements

Author(s): Damian Kopyto
Last updated: 09/04/2026

## Abstract

The orch-cli is a tool that enables a user to interact with the Edge Orchestrator and manage the resources associated with it. As of release 2026.0 it provides functionality for most of the desired features in place within that release. While the necessary functionality is in place and any new functionalities and mechanisms can be developed, it is crucial to further improve the user experience and versatility of the tool as we are moving from UI usage to CLI first approach. A number of optimizations and improvements have been identified that can elevate both how the CLI is experienced as a tool to evaluate the EOM product, as well as simplify and consolidate the outputs of the CLI under the hood at code level.

## Goals

The following are the improvements we would like to achieve to improve the usability, readability, and versatility of the orch-cli:

- Consolidated and optimized output table formatter for all resource outputs
- YAML and JSON format outputs
- Single host creation without the bulk import
- Simplified input provision to easy the evaluation of the product

### Table formatting and custom outputs

Currently the outputs of the CLI commands to display individual features follow the pattern of tailored print functions specific to each feature. While the pattern is similar in almost all the feature printouts, the actual output may be inconsistent across these command outputs. Furthermore, the outputs are pretty much preset and with exception of a few individual, tailor-made filters, the CLI does not allow for display of custom set of fields, easy filtering or sorting of the output's fields/values.

The goal is to introduce a common table formatter which can be used to create outputs across most features, the individual features would have a default fields to use with the formatter and would enable custom override of the fields to display. As a stretch goal the formatter should accept filtering and sorting.

Advantages of such approach include:

* Code cleanup and consolidation via reuse of the formatter to print outputs across most features
* Cleaner, more predictable outputs improve user experience
* Easily customizable outputs allow better insights for power users
* Sorting and filtering allows for faster and more precise insights into the deployment, especially at scale

One thing to keep in mind in regards to filtering is that the EIM API already provides a degree of filtering and some feature commands allow using it, so there may need to be a distinction between the two types of filters.

A base for this implementation is already established through this experimental branch that contains an implementation of a formatter library. The same implementation can be tweaked and expanded or used as an inspiration for a new code base. See the [experimental branch](https://github.com/open-edge-platform/orch-cli/compare/main...experiment/output-format) for reference.

### YAML and JSON output formats

The CLI currently lacks the functionality to display outputs in a more machine-readable way. The required output formats are YAML and JSON. For each feature command, `--output yaml` and `--output json` flags shall be provided that will enable the display of data in these formats. The major requirement would be to convert and display the data as-is when it comes from the API. As a stretch goal, consideration should be made if these outputs can also be customizable to display certain fields or sort or filter data.

### Single host creation

Originally, the CLI's host onboarding/creation was based and built as a bulk import tool for Edge Nodes. Even the creation of a single host required creating a CSV file and importing the host via bulk import. Introducing support for creation of a single host without using the bulk import via a single-line command will simplify the user experience, especially for someone wanting to quickly test or evaluate the EOM. A simplified `create host` command shall be implemented that allows creating a single host without using bulk import, improving the experience for quick testing and evaluation.

### Simplifying user inputs

**Resource identification by name:** The EOM API/CLI has a limitation where resource names for certain resources are not unique in the inventory, and unique identification is done via the resource ID. This means that the user needs to figure out the resource ID and then provide it, which for quick evaluation purposes may be tiresome. As part of the improvements to the CLI, the ability to input the resource by its name should be made available if only a single instance of this resource name exists. If multiple resources exist, an error should be thrown—or as a stretch goal, a suggestion prompt should appear listing the resource IDs that can be used. The major goal is to do this for the site resource for single host creation (evaluate the feasibility of using resource name in bulk creation), but a stretch goal is to expand to other resources and commands.

## CI/CD and unit test implications

The above changes will definitely affect the unit tests, which will need to be fixed accordingly. Additionally, the current CI/CD validation pipeline will need to be inspected for impacts due to the outputs changing and fixed as needed.

## Documentation

The documentation will need to be updated for these changes. The existing content may need to reflect new outputs. New content should be added to reflect and explain the new output formats and how to use them. The changes to how single host creation will now be possible should be documented, along with notes around selecting resources by name where necessary.
