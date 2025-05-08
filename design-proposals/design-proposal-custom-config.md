# Design Proposal: Custom EdgeNode Configuration 

This document outlines the detailed process for incorporating custom configurations specific to each edge node. It also explains how these configurations can be associated with instances, ensuring that they are applied during the provisioning phase. By following the steps provided, users can effectively manage and customize their edge node settings to meet specific requirements at the time of instance creation.

## Current supported mechanism

Currently, all edge node configurations are managed at a global, regional, or site-specific level. There is no existing mechanism to implement custom configurations on a per-edge node basis during the provisioning stage. 

As a result, adjustments such as changes to proxy settings and the configuration of multiple interface IPs are considered day-two operational activities. To perform any configuration updates on an edge node, users are required to manually access the node and apply the changes directly.

This limitation means that any specific customization needed for individual edge nodes cannot be automated during the initial setup. Instead, users must wait until the provisioning process is complete and then manually intervene to make the necessary adjustments. This approach can be time-consuming and may lead to inconsistencies in configuration across different nodes.

## Solution options

All options utilize cloud-init to provide custom configurations or per edge-node configurations. There is no doubt that cloud-init is the preferred tool, as the configuration needs to be done only once at bootup. Cloud-init, an open-source application developed by Canonical, is used to bootstrap Linux images in a cloud computing environment. It is a powerful tool for automating the initial setup of cloud instances, including configuring networking, storage, users, and packages. Cloud-init is exposed by hyperscale’s for VM configuration; for instance, AWS provides it as [user data](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/user-data.html#userdata-linux), while Azure offers it as [custom data](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/using-cloud-init).

The following options where evaluated to configure using cloud-init. 

1. Option 1 : 
    Only exposing the configuration required through a dedicated API to the orchestrator API user. The API's will have list of configurations that the user want to add and call the EIM API. The EIM shall add those configuration to the existing cloud-init curated by onboarding manager. 

2. Option 2 : 
    Exposing a template with the configs through a dedicated API to the user. The user shall populated with desired configurations in predefined XML or yaml template form and call the EIM API. The EIM onboarding manager shall add those configuration to the existing or new cloud-init.

3. Option 3 :
    The complete cloud-init file will be exposed to the user through a dedicated API. The user can create a cloud-init file, add the desired configurations, and call the EIM API. The EIM onboarding manager will then add this new cloud-init file to the Edge Node, allowing the cloud-init tool to configure the node accordingly.

  The ****Option 3*** offered more advantages when compared to others.

  *Scalability*: As new configurations or policies are defined, they can be seamlessly integrated into the existing setup with out changing to EIM API or Inventory. But with Option 1 and 2 changes will be needed in EIM. 

  *Flexibility*: Cloud-init supports a wide range of configuration options, including user data scripts, cloud-config directives, and more. This flexibility allows users to customize their instances extensively without being limited by predefined templates. Extending the Template or parameters (Option 1, 2) in API was be additional effort in furture maintenance. 

  *Integration and Reusability*: Cloud-init integrates seamlessly with various cloud platforms and services, ensuring that configurations are applied consistently across different environments. If the user have there existing cloud-init it can be *re-used* in EMF for custom configuration. This is not possible with Option 1 and 2.

  *Industry Standard*: Cloud-init is widely recognized and used in the industry for initializing cloud instances. It has extensive documentation and community support, making it a reliable choice. More parameter validation type check more documentation  etc will be required in EIM to handle it.

## Proposed solution

In this solution, the cloud-init configuration will be exposed to EMF users through UI/APIs. EMF users will be able to add their cloud-init YAML files, which will be utilized by the cloud-init tool during edge node provisioning.

The interfaces proposed shall not restrict it to only cloud-init file yaml format, in future the solution can be extented to different type of file formats such as bash script or json file format.

```mermaid
sequenceDiagram
%%{wrap}%%
autonumber
  participant user as User
  box rgb(235,255,255) Edge Orchestrator

  participant inv as Inventory / API
  participant om as Onboarding Manager
  end
  
  note over user : User creates a custom cloud-init config yaml.

  user ->> inv : Create custom-config resource providing cloud-init config yaml.
  note over inv : API does basic yaml validation of the cloud-init config file.
  inv ->> user : custom-config resourceID 

  note over user : User associates the custom-config resource with the host instance, through bulk import tool or UI or through Inventory APIs.  
  user ->> inv : Update or Create instance resource. 
  note over inv : The Instance resource now holds the mapped cloud-init resource name. The Instance API now has a param to  cloud-init resource.

  box LightYellow Edge Node(s)
  participant ctl as Cloud-Init tool
  end

  note over inv, ctl : During Edge Node provisioning
  om ->> ctl : custom-config could-init is moved to EdgeNode for cloud-init tool to consume.

  note over ctl : Along with other configs the cloud-init tool includes the custom-config on first OS boot-up.

```
**Assumption**:
- The user has validated the cloud-init config that will be added to the custom-config resource.
- The user will associate the custom-config resource before EdgeNode provisioning.
- Failures can only be debugged through logs captured by observability.

**Work flow**
- *Step 1*: The user shall invoke infra-core Inventory APIs to create a custom-config resource using the user-provided cloud-init config YAML.
- *Step 2*: After performing basic YAML validation, the Inventory shall store the custom-config in the database and return a custom-config resource ID.
- *Step 3*: The orchestrator user can associate the custom-config resource ID with the host instance. This association can be done through the Bulk Import Tool, UI, or Inventory instance API. After associating the resource ID, the user updates the Inventory.
- *Step 4*: During the onboarding and provisioning of the EdgeNode, the Onboarding Manager copies the user cloud-init file into the EdgeNode along with the default EdgeNode provisioning cloud-init file. The cloud-init tool then picks up the user/custom cloud-init config during the first boot. 

### Changes required for implementation 

#### 1. New custom-config resource and data model
A new custom-config resource will be added to the Infra-core data model, which captures information needed for a creating config on the edge node. The custom-config resource will contain the following fields:

- custom-config-file : The config file is the user provided configuration file. The file can be of different supported format specified in config-type. Below is the data-model of the custom-config-file. 


- custom-config-type : The configuration can be of different type such as cloud-init, bash file or json. The custom-config-type bash file or json can be for future use.

***Data-Model*** example of custom-config :

  ```yaml
  // Custom configuration type
  enum CustomConfigType {
  CUSTOM_CONFIG_TYPE_UNSPECIFIED = 0;
  CUSTOM_CONFIG_TYPE_CLOUD_INIT = 1;
  }
  // CustomConfigResource describes custom configuration 
  message ConfigTemplate {
    // configuration file.
    string config = 1 [
      (ent.field) = {
        optional: false
        immutable: true
      },

      (buf.validate.field).string = {
        min_len: 1
        max_len: 16384
        pattern: "^[A-Za-z0-9-/_\\[\\]\\.\\\\]*$"
      },
      (buf.validate.field).ignore = IGNORE_IF_UNPOPULATED
    ];

    CustomConfigType config_type = 2;
  }
  ```
  The instance resource data-model has to be updated to include custom-config resource.

  The relationship between an instance and its custom configuration shall be one-to-one. Each instance can be associated with only one pre-created custom configuration. We will not have more than one custom configuration or cloud-init associated with an instance, as most configurations can be consolidated into a single cloud-init file.

#### 2. EIM API enahncement

A new set of APIs will be added to support custom-config resource handling. These APIs should enable Create, Read, and Delete operations on custom-config resources. 

Additionally, the instance resource APIs must be extended to support the addition and updating of custom-config resources. Custom-config resources shall not be permitted to be deleted while the instance is active or associated. 

#### 3. Changes in Onboarding Manager 
During provisioning, the Onboarding Manager shall check the instance. If the custom-config contains the cloud-init file, it will be copied along with the existing default EMF EdgeNode cloud-init. Since the custom-config is optional, no action is required if the cloud-init file is not present. There will be no error handling for user cloud-init failures. 

#### 4. Updates to Bulk Import Tool 
The Bulk Import Tool has to be enhances to :
- Include custom-config resourceID in .csv file

  Ex: Here cc-8s150fg1 is the resource ID of custom-config
  ```csv
  Serial,UUID,OSProfile,Site,Secure,RemoteUser,CustomConfigNameMetadata,Error - do not fill
  2500JF3,4c4c4544-2046-5310-8052-cac04f515233,os-7d650dd1,site-08c1e377,true,localaccount-9dfb57cb,custom-config-1,key1=value1&key2=value2,
  ```
- Update Bulk Import Tool to check and include custom-config resource ID into the instance creation API. 

#### 5. UI enhancements
The UI should provide an option for users to create custom configurations cloud-init data. This feature can be designed similarly to the application profile YAML edit text input block under Deployments.
To assist users, sample helper templates can be included. 

Here is a sample UI with cloud-init configuration for updating IP address and proxy settings.

  ![cloud-init sample](./images/cloud-init.png)

The UI shall provide interface to the user during Host registration page to associate or Map the earlier created custom-config. The UI can be similar to that of LocalAccount Page were it shows the list of LocalAccount to associate.

Once the provisioning is completed, the cloud-init data should be immutable; no edit or delete options should be available. Users should only be able to view the cloud-init configuration under host details.


## Opens
1. To include single or multiple cloud-init configs.
2. Do we need to support more than one file format.