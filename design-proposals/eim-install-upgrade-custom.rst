.. SPDX-FileCopyrightText: 2025 Intel Corporation
..
.. SPDX-License-Identifier: Apache-2.0

# Design Proposal: EIM Profile-Based Deployment Framework

Author(s): Sunil Parida

Last updated: 2025-12-12  

-----------------
Abstract
-----------------
This document describes the platform engineering work required to enable EIM modular deployment through profile-based configurations. The platform team will develop infrastructure to support two EIM deployment profiles (vPRO-Only and OXM-Only) that selectively activate components based on operational requirements.

-----------------
Executive Summary
-----------------
We need to build a profile-based deployment framework to support EIM decomposition. Currently, our platform deploys EIM as a monolithic stack with no mechanism for selective component activation. 

This design outlines the framework we'll build to enable two deployment profiles: **EIM-vPRO-Only** and **EIM-OXM-Only**.

**Our Deliverables:** Profile configuration framework, cluster template generation system, and deployment validation using orch-cli.

**EIM Team Input Required:** Component decomposition specifications, dependency mappings, and profile definitions.

-----------------
1. Background
-----------------

**Current Platform Limitation**

Our deployment platform currently lacks the capability to deploy EIM components selectively as profiles. All EIM subcomponents deploy together:

- vPRO components
- Policy management (Kyverno)
- Management UI
**Note:** While AO, CO, and O11y can be individually toggled in current deployments, there's no profile-based framework to deploy predefined component combinations.


--------------------
2. Problem Statement
--------------------

**What We Need to Build**

We need to develop platform infrastructure that supports selective EIM component deployment based on profiles.

**EIM Team Requirements (Input)**

The EIM team has identified two deployment profiles they need:

**Profile 1: vPRO-Only**
- Deploy only vPRO-related components
- Disable: AO, CO, O11y, Kyverno, UI
- *Note: AO, CO, O11y,UI,Kyverno must be explicitly disabled for this profile*

**Profile 2: OXM-Only**
- Deploy only bare metal/OS management components
- Disable: vPRO, AO, CO, O11y, Kyverno, UI
- *Note: AO, CO, O11y must be explicitly disabled for this profile*

**Platform Engineering Requirements**

- Implement ArgoCD application selection logic
- Create cluster template generator
- validate profile E2E use `orch-cli`
- Develop validation and dependency checking mechanisms
- Ensure upgrade compatibility
- Support installation on existing clusters without disruption

-----------------------------------
3. Measurement KPIs (EMF Platform)
-----------------------------------

**3.1 vPRO-Only Profile KPIs**

*Deployment Metrics:*
- Deployment time
- Number of pods deployed
- ArgoCD application count

*Resource Metrics:*
- CPU allocation (cores)
- Memory allocation (GB)

**3.2 OXM-Only Profile KPIs**

*Deployment Metrics:*

- Deployment time
- Number of pods deployed
- ArgoCD application count

*Resource Metrics:*

- CPU allocation (cores)
- Memory allocation (GB)
- CPU allocation (cores)
- Memory allocation (GB)
