==============================================
Design Proposal: EMF Install/Upgrade of Decomposed EIM with Custom Configuration
==============================================

**Authors:** EMF Platform Team  
**Date:** 2025-11-12  

-----------------
Context
-----------------
EIM currently deploys as a monolithic stack regardless of customer requirements, resulting in unnecessary infrastructure overhead.  
The **EIM Modular Decomposition Strategy** introduces a **customer configuration profile** to enhance flexibility and efficiency.

This approach enables deployment of customized profiles based on operational needs, reducing resource usage and simplifying management.

-----------------
Decision
-----------------
Implement an **EIM Custom Configuration Profile Framework** for decomposed EIM with the following capabilities:

- **Deployment**: Custom profile–based EIM deployment  
- **EIM Agent Enable/Disable**: Onboarding-specific agents can be enabled or disabled as per customer requirements  
- **vPRO Enable/Disable**: vPRO component enable/disable control  
- **Upgrade Support**: EIM custom profile upgrade capability  
- **Day-2 Profile OS Update**: Preserve previously enabled/disabled agent states during OS updates  
- **Orch-CLI Support**: Support onboarding and configuration via `orch-cli`  

-----------------
EIM Component Selection
-----------------
- EIM (vPRO Enabled)  
- EIM (vPRO Disabled) 

-----------------
EIM EN Agent Selection (Enable/Disable)
-----------------
- CO  
- O11Y  
- PMA  
- xxx
- xxx

-----------------
Upgrade Support Framework
-----------------
**EMF V1 → V2 Smooth Upgrade for Custom Configuration Workflow**

The upgrade process ensures that all agent and vPRO states are preserved across EMF versions.  
Compatibility checks and rollback mechanisms are integrated to ensure reliable transitions.

-----------------
Day-2 Operations with State Preservation
-----------------
- OS updates retain prior agent/vPRO enablement configuration  
- Profile integrity maintained during and after OS updates  

-----------------
Implementation Framework
-----------------
**Agent Enable/Disable Implementation**  
- Individual agent control during installation and upgrades  
- Onboarding-specific agent activation based on customer requirements  
- Agent state persistence across EMF version upgrades  
- Validation of agent dependencies and conflicts  

**vPRO Component Control Implementation**  
- Dynamic vPRO enable/disable during EMF deployment  
- vPRO state preservation across upgrades  

**Upgrade Support Implementation**  
- EMF V1 → V2 migration with preserved configuration  
- Smooth transition mechanisms for all components  
- Rollback capabilities for failed upgrades  
- Compatibility validation between versions  

**Day-2 Operations Implementation**  
- OS update workflows with preserved profile states  
- Agent configuration integrity during system updates  
- vPRO configuration persistence through updates  
- Health monitoring and validation post-updates  

-----------------
Success Criteria
-----------------
- **Agent Management**: 100% customer control over individual agent enable/disable  
- **vPRO Control**: Complete vPRO enable/disable capability  
- **Upgrade Success**: EMF V1→V2 upgrades with zero configuration loss  
- **Day-2 Integrity**: OS updates maintain configured agent and vPRO states  
- **Performance**: Optimized resource utilization through modular deployment  
