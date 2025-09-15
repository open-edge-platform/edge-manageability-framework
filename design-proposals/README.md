# Proposing Features and Changes to Edge Manageability Framework (EMF)

## Introduction

The development process of components in the Open Edge Platform is
design-driven.
Significant changes to the different components, installation scripts,
bare metal agents, APIs simulators, libraries, or tools
must be first discussed, formally documented and agreed upon before they
can be implemented.

This document describes the process for proposing, documenting, and
implementing changes.

To learn more about the different components please check
the [Edge Manageability Framework documentation](https://docs.openedgeplatform.intel.com/edge-manage-docs/main/index.html)

## The Proposal Process

The proposal process is the process for reviewing a proposal and reaching
a decision about whether to accept or decline the proposal.

1. The proposal author
   creates [a Github issue](https://github.com/open-edge-platform/edge-manageability-framework/issues)
   with the `Feature Request` template describing biefely the new feature.\
   Note: There is no need for a design proposal document at this point.\

2. A discussion on the issue aims to triage the Feature Request with the
   Maintainers of the overall project and of the specific sub-project
   into one of three outcomes:
    - accept the feature request, or
    - decline the feature request, or
    - ask for a [Design Document](#design-documents), which if merged
      becomes an `Architecture Decision Record` (ADR).

   If the feature request is accepted or declined, the process is done.
   Otherwise the discussion is expected to identify concerns that
   should be addressed in more detail in the design document.

3. If requested the proposal author writes
   a [Design Document](#design-documents)
   to work out details of the proposed design and address the concerns raised
   in the initial discussion.
   The author create a Pull Request in the `edge-manageability-framework repo`
   with a `Proposal` label in it and links
   the Pull Request in the original feature issue.

4. Once comments and revisions on the design doc wind down, there is a final
   discussion on the issue, to reach one of two outcomes:
    - accept design document and related feature by merging the Pull Request or
    - decline proposal, by closing the Pull Request without merging.

5. If the Pull request with the Design Document is merged, it means the design
   is accepted in the form it's described
   and it becomes an **ADR, Architecture Decision Record**. If the Pull Request

After the Pull Request is merged or closed and it's corresponding design is
accepted or declined (whether after step
2 or step 4), implementation work proceeds in the same way as any other
contribution.

## Detail

### Goals

- Make sure that proposals get a proper, fair, timely, recorded evaluation with
  a clear answer.
- Make past proposals easy to find, to avoid duplicated effort.
- If a design doc is needed, make sure contributors know how to write a good
  one.

### Definitions

- A **proposal** is a suggestion filed as a Feature Request GitHub issue,
  identified by having
  the Proposal label.
- A **design document**, is the expanded form of a proposal,
  written when the proposal needs more careful explanation and consideration.
- An **ADR, Architecture Decision Record** is the merged version of the **design
  document**.

### Scope

The proposal process should be used for any notable change or addition to the
language, libraries and tools.
“Notable” includes (but is not limited to):

- New features, such as the addition of a new component or a new user flow.
- Major API changes.
- UI changes.
- Any other visible behavior changes in existing functionality.
- Adoption or use of new protocols, protocol versions, cryptographic algorithms,
  and the like,
  even in an implementation.

Since proposals begin (and will often end) with the filing of an Feature Request
issue, even
small changes can go through the proposal process if appropriate.
Deciding what is appropriate is matter of judgment we will refine through
experience.
If in doubt, file a proposal.

There is a short list of changes that are typically not in scope for the
proposal process:

- Making API changes in internal packages, since those APIs are not publicly
  visible.
- Internal per component changes or bug fixes that do not affect the overall
  user experience or any other element.

Again, if in doubt, file a proposal, best case is immediately accepted.

### Design Documents

As noted above, some (but not all) proposals need to be elaborated in design
document, that if accepted becomes an
Architecture Decision Record (ADR)

- The design document should be checked in
  to [the proposal directory](https://github.com/open-edge-platform/edge-manageability-framework/tree/main/design-proposals/)
  as `NNNN-shortname.md`,
  where `NNNN` is the project name (e.g. app-orch) and `shortname` is a short
  name (a few dash-separated words at most).

- The design doc should follow [the template](./design-proposal-template.md).

- The design doc should address any specific concerns raised during the initial
  discussion.

- It is expected that the design doc may go through multiple checked-in
  revisions.

- For easier review on GitHub, design documents should comply with the `markdownlint` rules defined in this repository.
  These rules will be enforced by the CI pipeline.
  Writers can verify compliance locally by running the `make lint` command.

### Quick Start for Experienced Committers

Experienced committers who are certain that a design doc will be
required for a particular proposal can skip steps 1 and 2 and include the design
doc
with the initial issue.

In the worst case, skipping these steps only leads to an unnecessary design doc.

### Proposal Review

The proposal will be discussed in the pull request and if needed one or more
online calls will be established.
At the end of the desicussion the repo maintainer(s) for the involved code will
have the last say for moving the
proposal in one of the following 3 stages.

#### Active

Issues in the Active column are reviewed weekly in the different teams
to watch for emerging consensus in the discussions.
The maintainers may also comment, make suggestions,
ask clarifying questions, and try to restate the proposals to make sure
everyone agrees about what exactly is being discussed.

#### Accepted

Once a proposal is marked Accepted, the Pull Request is merged or the Issue
closed, the Proposal-Accepted label
is applied,
and the accepted design will be in the `/design-proposals` folder.

One can apply a release label to signify which release of the Edge Manageability
Framework release will contain
the proposal.

#### Declined

A proposal is marked as Declined if a complete re-work is needed, change is not
required anymore or not applicable.
Once a proposal is Declined, the Pull request or the Feature Request it is
closed.

## Help

If you need help with this process, please contact the Project's maintainers
contributors by posting to
the [Discussions](https://github.com/open-edge-platform/edge-manageability-framework/discussions)
.

To learn about contributing to Edge Manageability Framework in general, see the
[contribution guidelines](https://docs.openedgeplatform.intel.com/edge-manage-docs/main/developer_guide/contributor_guide/index.html)
.
