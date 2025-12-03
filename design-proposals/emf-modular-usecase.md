# Design Proposal: Edge Manageability Framework Modular Use Case

Author(s): Edge Manageability Framework Architecture Team

Last updated: 2025-12-02

## Abstract

Starting 2025.02 release EMF will support capability for users to deploy full EMF or turnoff certain domains to deploy
a modular EMF instance. This design proposal outlines the customer usecases for modular EMF deployment. This document
will not address the full EMF usecase as it is already well established.

## Supported Modular Use Cases

The following modular use cases will be supported in EMF starting 2025.02 release along with the full EMF deployment.:

- **EIM+CO+AO** Modular deployment with Edge infrastructure manager, Cluster Orchestration and Application
   orchestration without Edge node and Orchestrator observability.
- **EIM** Modular deployment with Edge infrastructure manager only.
- **EIM+CO** Modular deployment with Edge infrastructure manager and Cluster Orchestration only.
- **EIM+OB** Modular deployment with Edge infrastructure manager with Edge node and Orchestrator observability.
- **EIM+CO+OB** Modular deployment with Edge infrastructure manager, Cluster Orchestration with Edge node
   and Orchestrator observability.

In all the above modular usecases only CLI and API based interfaces will be supported. GUI based interfaces will not
be supported in modular usecases.

Following EMF domains will not be supported in any of the modular usecases: AO only, CO only, OB only and AO+CO+OB.

## User Stories for the EMF Modular Use Cases

In this section we will outline the user stories for each of the modular use cases supported in EMF. These user stories
will help define the requirements and validation tests for each modular use case.

### EIM+CO+AO User Stories

EIM+CO+AO modular deployment is targeted for full EMF customers who want to manage their own observability stack but
use EIM, CO and AO from EMF. For customers to use their own observability stack, they will need setup the required data
store, dashboard and observability control plane services. The Edge node and orchestrator services will be emitting the
logs and metrics to the customer provided observability stack. The current implementation of the edge node observability
agents and collectors on the orchestrator should be flexible enough to support this use case.

```mermaid
flowchart TD
    %% Classes for styling
    classDef emf fill:#e3f2fd,stroke:#1565c0,stroke-width:2px,color:#0d47a1;
    classDef customer fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:#1b5e20;
    classDef edge fill:#fff3e0,stroke:#ef6c00,stroke-width:2px,color:#e65100;

    subgraph EMF_Scope ["EMF Orchestrator Services"]
        direction TB
        EIM[Edge Infrastructure Manager]:::emf
        CO[Cluster Orchestration]:::emf
        AO[Application Orchestration]:::emf
    end

    subgraph Edge_Scope ["Edge Infrastructure"]
        Edge_Node[Edge Node Agents]:::edge
        Obs_Agent[Observability Agent & Collector]:::edge
    end

    subgraph Customer_Scope ["Customer Managed Scope"]
        direction TB
        Obs_Stack["Customer Observability Stack<br/>(Data Store, Dashboards, Alerts)"]:::customer
    end

    %% Management Relationships
    EIM -->|Manages| Edge_Node
    CO -->|Orchestrates| Edge_Node
    AO -->|Deploys Apps| Edge_Node

    %% Data Flow
    EIM & CO & AO -.->|Logs & Metrics| Obs_Stack
    Edge_Node -.->|Logs & Metrics| Obs_Agent
    Obs_Agent -.->|Forward| Obs_Stack

    %% Styling links for data flow
    linkStyle 3,4,5,6,7 stroke:#d32f2f,stroke-width:2px,stroke-dasharray: 5 5;
```

**User story 1:** As a admin-user, I want to deploy EMF with EIM, CO and AO domains only so that I can manage my own
observability stack and configure the edge node agents, collectors, orchestrator services and collectors to use the
observability stack deployed by me.

**User story 2:** As a developer-user, I want to deploy EMF with EIM, CO and AO domains only as i am only
interested in logs i would like detailed instructions on how to get the edge node agents logs and orchestrator
services logs directly logging into the system.

### EIM User Stories

EIM modular deployment is targeted for customers who want to use EMF for edge infrastructure management only. The
customer will be responsible for managing cluster and applciation life cycle management. It might be ideal for customer
deployed application and cluster management stack might need to leverage EIM APIs to link the managed edge node
instances with their own stack. It is also possible to use EIM as a standalone edge node management solution.

```mermaid
flowchart TD
    %% Classes for styling
    classDef emf fill:#e3f2fd,stroke:#1565c0,stroke-width:2px,color:#0d47a1;
    classDef customer fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:#1b5e20;
    classDef edge fill:#fff3e0,stroke:#ef6c00,stroke-width:2px,color:#e65100;

    subgraph EMF_Scope ["EMF Orchestrator Services"]
        direction TB
        EIM[Edge Infrastructure Manager]:::emf
    end

    subgraph Edge_Scope ["Edge Infrastructure"]
        Edge_Node[Edge Node Agents]:::edge
    end

    subgraph Customer_Scope ["Customer Managed Scope"]
        direction TB
        Cust_Mgmt["Customer App & Cluster<br/>Management Stack"]:::customer
        Obs_Stack["Customer Observability Stack"]:::customer
    end

    %% Management Relationships
    EIM -->|Manages Infrastructure| Edge_Node
    Cust_Mgmt -->|Integrates via API| EIM
    Cust_Mgmt -->|Manages Apps & Clusters| Edge_Node

    %% Data Flow
    EIM -.->|Logs & Metrics| Obs_Stack
    Edge_Node -.->|Logs & Metrics| Obs_Stack

    %% Styling links for data flow
    linkStyle 3,4 stroke:#d32f2f,stroke-width:2px,stroke-dasharray: 5 5;
```

**User story 1:** As a admin-user, I want to deploy EMF with standalone EIM domain only for device lifecycle
management at fleet level. I can manage my own Application, Cluster and observability management stack without
integrating with the EIM state using the EIM APIs.  

**User story 2:** As a developer-user, I want to deploy EMF with EIM domain only for device lifecycle management
at fleet. I can manage my own Application, Cluster and observability management stack. I will integrate my application,
cluster and observability stack with the Infrastructure state maintained by EIM. For this I need detailed instructions
on how to use EIM APIs for integrating with my own stack.
