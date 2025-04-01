# How to use

## Deploy an ENiVM

This setup assumes that you have a Proxmox Host available.

Update the `example.lab.tfvars` file with your Proxmox Host setup.

Run the command below:

```hcl
terraform apply --var-file=example.lab.tfvars -var vm_cores=16 -var vm_memory=65536
```

Input the values for the following parameters:

```bash
orch_fqdn       // The FQDN of the orchestrator
orch_org        // An organisation created on the orchestrator
orch_project    // A project created within that organisation
orch_user       // The user associated with the organisation/project
orch_password   // The password of that user
```

## Connect to an ENiVM

The ENiVM will be deployed in a kind cluster.

To access that VM, follow these steps:

1. Connect to the Proxmox VM that was created in the previous section.

    ```bash
      ssh ubuntu@<IP of ENiVM node>
    ```

1. Create a `service.yaml` with the spec below:

    ```yaml
    apiVersion: v1
    kind: Service
    metadata:
      name: test-vm
    spec:
      ports:
      - port: 22
        targetPort: 22
        nodePort: 30019
      selector:
        kubevirt.io/domain: vm1
      type: NodePort
    ```

1. Deploy a service into the kind cluster. The run the command below to install the service:

    ```bash
    kubectl apply -f service.yaml
    ```

1. Retrieve the IP address of the node:

    ```bash
    docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' kind-control-plane
    ```

1. The previous command should produce an IP e.g., `172.18.0.2`. Use this IP to connect to the Kubevirt VM. The login details can be found in the `cloud-init` section of the `enivm.tftpl` file.

    ```bash
    ssh lp4273@172.18.0.2 -p 30019
    ```
