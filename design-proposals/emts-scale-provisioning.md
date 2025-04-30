# Design Proposal: Scale provisioning of EMT-S edge node supporting OXM workflow

Author(s): EIM team.

Last updated: 29.04.2025

## Abstract

The Edge Microvisor Toolkit Standalone (EMT-S) node is designed to enable enterprise customers and developers to evaluate Intel silicon-based platforms for Edge AI use cases. In this context, Original Equipment Manufacturers (OXMs) play a critical role by preparing edge nodes in bulk for end customers at their facilities before shipping them to the deployment locations. To support the OXM workflow for Edge AI use cases, it is essential to implement a scalable provisioning solution for multiple EMT-S nodes. This document outlines the design proposal for enabling large-scale provisioning of EMT-S edge nodes to meet the requirements of the OXM workflow.

### Proposal Summary

EIM leverages the Tinkerbell solution for provisioning operating systems on edge nodes. The current implementation supports OS provisioning via bootable USB or iPXE/HTTPs boot. To enable scalable provisioning of EMT-S edge nodes for the OXM workflow, this proposal suggests integrating PXE-based provisioning into EIM by utilizing the `boots` component of Tinkerbell. Additionally, EMF and EIM will support configurations to deploy this as a standalone EIM solution (to be referred in future as EIM-S), tailored for OXMs to efficiently provision edge nodes at scale. OXM will have the option of provisioning edge nodes using bootable USB, iPXE/HTTPs boot, or PXE-based provisioning. The solution will also include a user experience (UX) for pre-registering edge nodes using serial numbers, UUIDs, or MAC addresses. Furthermore, the solution will support the provisioning of different operating system profiles based on the selected identifiers. In cases where a device on the local area network (LAN) boots over PXE and is not pre-registered, the default operating system will be provisioned

### MVP requirements

Following are the MVP requirements for the scale provisioning of EMT-S edge node supporting OXM workflow:

- Provision multiple BareMetal edge nodes without onboarding for the purpose of standalone/singleton use.
- Provide deploy a service on the local network that can achieve this provisioning at scale.
- Deploy the provisioning service on the local network that support PXE Boot (BIOS/UEFI with DHCP + TFTP) boot and iPXE with HTTPs.  
- Have a UX to pre-register BareMetal edge nodes using Serial number or UUID or MAC address.
- Provision different OS profiles to different edge nodes selected based on Serial number or UUID or MAC address.
- Provision default OS when a device on the LAN boots over PXE and is not pre-registered.
- Have a ux of  collecting provisioning logs and status of edge nodes.

> Note: It might be possible for EMF-EIM to support provisioning of the EMT-S nodes. supporting this capability as part of MVP depends on any active customer requirements.
