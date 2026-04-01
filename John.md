https://mermaid.ai/open-source/syntax/flowchart.html
```mermaid
A@{ shape: manual-file }
```
```mermaid
  flowchart LR
    A@{ shape: cloud, label: "Comment"  }
    info
    subgraph Cloud[" "]
        shape cloud
        Apps[Apps]
        Infra[Infra]
    end

```

```mermaid
flowchart TD

    %% LEFT: Cloud
    subgraph Cloud[" "]
    direction LR
    APPS@{ shape: cloud, label: "Apps"  }
    Infra@{ shape: cloud, label: "Infra"  }
    end

    %% EDGE SITES
    SF["Edge Nodes at Customer Site<br>(San Francisco)"]
    ATL["Edge Nodes at Customer Site<br>(Atlanta)"]
    NYC["Edge Nodes at Customer Site<br>(New York City)"]

    Cloud -.-> SF
    Cloud -.-> ATL
    Cloud -.-> NYC

    %% RIGHT SIDE
    subgraph EO["Edge Orchestrator"]
        direction TB
        subgraph Orch[" "]
        direction TB
        WebUI[Web-UI]

        subgraph OrchestrationLayer[" "]
            direction LR
            AppOrch["Application<br>Orchestration"] ~~~
            ClusterOrch["Multi Edge Cluster<br>Orchestration"] ~~~
            InfraMgmt["Edge Infrastructure<br>Management"] 
        end

        Platform["Foundational Platform Services<br/>(Identity and Access Mgmt, Secrets Mgmt,<br/>API Gateway, Observability, etc.)"]

        AWS[AWS* Infrastructure / On-Prem Datacenter]
end
        %% EDGE NODE

        subgraph EdgeNode["Edge Node"]
            direction TB
            subgraph AppsRow[" "]
                direction LR
                CA1[Customer Apps] ~~~
                CA2[Customer Apps] ~~~
                CA3[Customer Apps]
            end

            K8s[Kubernetes* Cluster]
            OS[Edge Node OS, Packages, Agents]
            HW["Edge Node Hardware<br/>(Intel® Xeon® processor, Intel® Core™ processor)"]

            %% Invisible ordering inside Edge Node
            AppsRow ~~~ K8s
            K8s ~~~ OS
            OS ~~~ HW
        end
        %% Invisible ordering inside EO
        WebUI ~~~ Orch
        Orch ~~~ Platform
        Platform ~~~ AWS
        AWS ~~~ EdgeNode
    end


    Cloud -.-> |"Cloud-based<br>Orchestration"|EO


    %% Styling
    classDef grey fill:#eeeeee,stroke:#666,stroke-width:1.5px
    classDef blue fill:#1f4fbf,color:#fff,stroke:#1f4fbf;
    classDef lightblue fill:#1fb6d9,color:#000,stroke:#1fb6d9;

    class EO,EdgeNode,AppsRow,Orch,OrchestrationLayer grey;
    class WebUI,AppOrch,ClusterOrch,InfraMgmt,Platform,AWS blue;
    class CA1,CA2,CA3,K8s,OS,HW lightblue;
```