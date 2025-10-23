# Understanding Architecture Decision Records (ADRs)

## What is an ADR?

An **Architecture Decision Record (ADR)** is a document that captures an important architectural decision made in
the Edge Manageability Framework, along with its context and consequences. In this project, ADRs are the accepted
and merged versions of design proposals.

ADRs serve as a historical record of why certain decisions were made, helping current and future contributors
understand the rationale behind the architecture and design choices in the project.

## Why Do We Use ADRs?

### 1. **Preserving Context**

Software projects evolve over time, and team members come and go. Without proper documentation, the reasons behind
important decisions can be lost. ADRs preserve the context and reasoning that led to each decision, making it easier
for new contributors to understand the project's evolution.

### 2. **Reducing Repeated Discussions**

When a decision has been thoroughly discussed and documented, future contributors can reference the ADR instead of
rehashing the same debates. This saves time and helps maintain consistency.

### 3. **Enabling Better Decisions**

By documenting alternatives considered and trade-offs evaluated, ADRs help future decision-makers learn from past
decisions. They can see what was tried, what worked, what didn't, and why.

### 4. **Improving Onboarding**

New team members can read ADRs to quickly understand how the system got to its current state and the philosophy
behind key architectural choices.

### 5. **Facilitating Code Reviews**

During code reviews, ADRs provide context for why certain patterns or approaches are used, making reviews more
informed and constructive.

## How to Read an ADR

Each ADR in this repository follows a standard template with the following sections:

### **Title**

The title should be clear and descriptive, often prefixed with the component name (e.g., `app-orch-`, `cluster-orch-`,
`eim-`).

### **Author(s) and Date**

Identifies who proposed the change and when, providing attribution and temporal context.

### **Abstract**

A short summary that explains:

- What problem is being solved
- Who is affected by this problem (personas/user flows)
- The high-level solution being proposed

**Reading Tip:** Start here to quickly understand if this ADR is relevant to your current work or question.

### **Proposal**

The detailed technical specification of the proposed change, including:

- Precise descriptions of what will change
- Technical details and specifications
- Architecture diagrams and flow charts
- API changes or additions
- User interface modifications

**Reading Tip:** This is where you'll find the "what" - the actual solution being implemented.

### **Rationale**

This section explains the "why" behind the decision:

- Alternative approaches that were considered
- Trade-offs between different options
- Advantages of the chosen approach
- Disadvantages or limitations acknowledged
- Why alternatives were rejected

**Reading Tip:** This is often the most valuable section for understanding the decision-making process. If you're
wondering "why didn't they do X instead?", the answer is likely here.

### **Affected Components and Teams**

Lists which parts of the Edge Manageability Framework are impacted:

- Edge Infrastructure Manager (EIM)
- Edge Cluster Orchestrator (CO)
- Edge Application Orchestrator (AO)
- UI, CLI, Observability, Platform Services, etc.
- Teams responsible for implementation

**Reading Tip:** Use this to understand the scope and complexity of the change.

### **Implementation Plan**

Describes how the proposal will be executed:

- Who will implement each part
- Timeline and milestones
- How it fits into release cycles
- Dependencies on other work

**Reading Tip:** This helps you understand when features will be available and who to contact for more information.

### **Open Issues**

Documents known problems or questions without solutions at the time of writing.

**Reading Tip:** If you're working on related features, you might help solve these open issues.

## Finding Relevant ADRs

### By Component

ADRs are typically named with a component prefix:

- `app-orch-*.md` - Application Orchestrator
- `cluster-orch-*.md` or `co-*.md` - Cluster Orchestrator
- `eim-*.md` - Edge Infrastructure Manager
- `orch-*.md` - Cross-component or general orchestrator features
- `emf-*.md` - Edge Manageability Framework platform-level
- `vpro-*.md` - vPro device management features

### By Topic

You can search for ADRs by topic or keyword:

```bash
# Search for ADRs related to a specific topic
grep -r "keyword" design-proposals/*.md

# List all ADRs
ls design-proposals/*.md
```

### In the README

The [design-proposals README](./README.md) explains the full proposal process and how design documents become ADRs.

## Common Questions About ADRs

### Q: Are ADRs permanent?

**A:** Yes, ADRs document decisions made at a point in time. If a decision needs to be changed later, a new ADR should
be created that supersedes the old one, explaining why the change was needed. The old ADR remains for historical
context.

### Q: What if I disagree with an ADR?

**A:** If an ADR is already merged and implemented, you can propose a change through a new design proposal that
explains why the previous decision should be reconsidered. Include new information, changed requirements, or lessons
learned from implementation.

### Q: Do I need an ADR for my change?

**A:** According to the [proposal process](./README.md), you need an ADR for:

- New features or major functionality
- Significant API changes
- UI/UX changes
- Changes to protocols, security, or cryptographic algorithms
- Any change that affects multiple components or the overall architecture

Small bug fixes or internal implementation details typically don't need an ADR.

### Q: How detailed should an ADR be?

**A:** ADRs should be detailed enough that someone unfamiliar with the project can understand:

- What problem you're solving
- What solution you chose
- Why you chose it over alternatives
- What the implications are

Include diagrams, code examples, and API specifications where helpful, but avoid unnecessary verbosity.

### Q: Can I update an ADR after it's merged?

**A:** Minor clarifications, typo fixes, and updates to implementation status can be made to existing ADRs. However,
substantive changes to the decision itself should be documented in a new ADR that references the original.

## Best Practices for Using ADRs

### For Contributors

1. **Read relevant ADRs** before starting work on a feature to understand existing decisions
2. **Reference ADRs** in code comments, PR descriptions, and discussions to provide context
3. **Update ADRs** if you discover the implementation differs from the plan (add notes in the document)
4. **Propose new ADRs** when you identify gaps or need to change existing decisions

### For Reviewers

1. **Check for ADRs** when reviewing significant changes - ensure architectural decisions are documented
2. **Verify alignment** with existing ADRs during code reviews
3. **Ask for ADRs** when reviewing changes that should be documented but aren't

### For Project Maintainers

1. **Ensure completeness** - ADRs should be complete before merging
2. **Facilitate discussion** - use ADRs to structure technical discussions
3. **Maintain organization** - keep ADRs well-organized and easy to discover

## Example: Reading an ADR

Let's walk through how to read an ADR effectively using a hypothetical example:

### Scenario

You're working on the Application Orchestrator and need to understand how Helm charts are converted to Deployment
Packages (DPs).

### Steps

1. **Find the ADR:** Look for `app-orch-helm-to-dp.md` in the design-proposals directory

2. **Read the Abstract:** Quickly understand that this ADR describes the conversion process and who it affects

3. **Skim the Proposal:** Get a high-level understanding of the technical approach

4. **Dive into Rationale:** Understand why this approach was chosen over alternatives like:
   - Direct Helm deployment without conversion
   - Using a different packaging format
   - Alternative conversion strategies

5. **Check Affected Components:** See which parts of the system are involved

6. **Review Implementation Plan:** Understand the current status and who implemented it

7. **Note Open Issues:** Be aware of any known limitations or future work needed

### Using What You Learned

Now you can:

- Implement features that align with the documented architecture
- Understand the trade-offs and design constraints
- Contribute improvements while respecting the original design rationale
- Propose changes if you discover new information that affects the decision

## Additional Resources

- [Design Proposal Process](./README.md) - How to create and submit design proposals
- [Design Proposal Template](./design-proposal-template.md) - Template for new proposals
- [Edge Manageability Framework Documentation](https://docs.openedgeplatform.intel.com/edge-manage-docs/main/index.html)
  \- Full product documentation
- [Contribution Guidelines](https://docs.openedgeplatform.intel.com/edge-manage-docs/main/developer_guide/contributor_guide/index.html)
  \- How to contribute to the project

## Getting Help

If you have questions about ADRs or the proposal process:

- Post in [GitHub Discussions](https://github.com/open-edge-platform/edge-manageability-framework/discussions)
- Contact project maintainers
- Ask in your team's communication channels

---

**Remember:** ADRs are living documents that serve the project and its contributors. They should help, not hinder,
productive work. When in doubt about whether you need an ADR or how detailed it should be, ask the maintainers
or start a discussion.
