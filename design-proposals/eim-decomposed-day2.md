# Design Proposal: Day 2 - Edge Node Updates in Modular EIM

Author(s) Edge Infrastructure Manager Team

Last updated: 12/01/26

## Abstract

In decomposed EIM, some scenarios will provide option to perform OS updates on edge nodes that were not provisioned with EIM. They may have customer specific OS installed, and the customer may want to updatedate the OS to their custom OS. This proposal explores the possible solutions and their limitations to identify the best design allowing for such an update.

## Background and Context


### EIM managers that participate in day 2 update:

- Inventory — Stores the Edge Node instances and the OS Resource related to each instance. An OS Resource includes OS version information, whereas an Instance Resource contains a link to the inventory resource representing the currently installed OS Resource.
- Maintenance Manager — The Resource Managers responds to status updates from the PUA with payloads that include an update schedule and information about the requested update from the Inventory. After a successful OS update, it receives updated OS version information from the PUA and sets the current version of Edge Microvisor Toolkit for the relevant Edge Node instance in the Inventory.
- OS Resource Manager — The OS Resource Manager maintains a cache of OS Resources, Tenant Resources, Provider Resources and Instance Resources. The cache for Tenant Resourcesd, Provider Resources and Instance Resource is updated based on notifications from the Inventory. OS Resource Manager searches the cache to verify the existence of OS Resources per all tenants. If any OS Resource is missing, OS Resource Manager creates it and adds to the Inventory. To update the cache of OS Resources per tenant, the resource manager periodically queries the Release Service for OS profile manifests associated with a specific EMF release as configured in the os-resource configuration. It updates existing OS profile details based on the latest information from the Release Service, ensuring that any changes are reflected in the system. When the Edge Orchestrator is upgraded to a version with a new osProfileRevision, the OS Resource Manager discovers new OS profiles for the Edge Microvisor Toolkit corresponding to the updated tag and generates the appropriate OS Resources in the Inventory.

### Edge Node Agents that participate in day 2 update:

- Platform Update Agent (PUA) — A Bare Metal Agent installed on the Edge Node, it is responsible for initiating communication with the Maintenance Manager (MM) on the Edge Orchestrator side and starting the update process. Communication is driven by periodic requests from the PUA to the MM, including the status of the Edge Node update.
- In-Band Manageability - A daemon (inbd) and client (inbc) that  provides in-band management capabilities for edge devices, including firmware updates, configuration management, and system operations. INBM provide a set of conmmnads used by the PUA to perform the OS update on Edge Node with mutable OS. Unused for mutable OS update, but PUA always requires it to be installed.

### OS Profile

( https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/advanced_functionality/custom_os_profile.html )

Currently EMF allows users to create and manage custom OS profiles that can be used to provision edge nodes. By defining a custom OS profile, users can specify custom built OS flavors of Ubuntu and Edge Microvisor Toolkit (EMT) as an edge node’s operating system. This enables a more tailored and efficient deployment process for edge nodes, allowing users to leverage their own OS images that are optimized for their specific use cases. These profiles CANNOT be used for day2 updates. 

However, here are the instructions for creating the custom OS profile (https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/advanced_functionality/custom_os_profile.html#prerequisites). 
Other than that, it is required that the old and new OS profile must have the same profile name. Also the EIM must use the version of the os-profile that released the OS profile (https://github.com/open-edge-platform/infra-core/tree/main/os-profiles). These reqquirements assure compatibility between the OS versions and the EIM.

#### LIMITATIONS
- Custom OS profile can used for only Day0 operation like edge node provisioning. Whereas Custom OS profile cannot be used for Day2 operations like OS upgrade using maintenance manager. (MAY NOT BE TRUE as OS resource mangers doesn't set the desiredOS any more - it is the MM that checks Inventory for more recent version of the OS with the same 'name' and 'profile_name')
- OS resource manager doesn’t manage the custom OS profiles hence it does not get the CVEs information of the operating systems

### Scheduling Updates

OS Updates must be scheduled by the admin person by creating a maintenance window though the CLI or Web UI in advance. This proces results in creation of a new schedule resource (single or repeated) in the inventory. The schedule resource is created per a single host or per region or site (to cover all host within it). Thus, it makes the schedule creation dependant on the region.(https://docs.openedgeplatform.intel.com/edge-manage-docs/dev/user_guide/advanced_functionality/host_schedule_main.html) When Maintenenance Manager retrieves scheduleles per hosts, a filter is used that looks for schedules associated with given hostID, siteID of the host, and regionID of the host.

#### LIMITATIONS
- with custom region ID and site ID user can schedule single host updates or all host updates only, because they all will be in one EIM region/site.

### OS Update Policy

OS update procedure that requires providing some configuration from the user requires creation of OS Update Policy that becomes linked to the instance resource. The OS update Policy may contain a link to the OS resource, or may just require the update to the latest available version. The policy is required in all cases of immutable OS update. In case of a simple mutable OS update, the OS Update Policy is not required. It is only required if admin wants to install new pacakges or/and update kernel commandline.  

### OS Update History

For every started day 2 update a new OS Update Run resource is created. It is linked to instance by instanceID. and to the OS Update Policy by its ID.

## Requirements

- No provisioning through EIM, assign custom OS resource ID to instance, custom region and site ID to host.
- Update EN to custom OS profile and push OS image and the manifest to the Release Service (custom?) 

## Questions

- will user use their custom release service? do we need OS resource manager to fetch the OS manifests from there?
- can custom OS profile be used for day 2 update?
- what about site and region? with custom region ID and site ID user can schedule only single host updates or all host updates, because they all will be in one EIM region/site.
- is region and site ID required by host at all? - how host will work without them? (adding them to host is not part of provisioning in the onboarding-manager)



