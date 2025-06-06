# Proof of Concept for Layer 2 Networking in Orchestrator

This file contains notes and code for proving the networking design as defined in the [EMT-S Scale Provisioning design
proposal](https://github.com/open-edge-platform/edge-manageability-framework/blob/4d801d24d402871ee664fbedcd3f964d43a784b2/design-proposals/emts-scale-provisioning.md?plain).

## Deployment

### Prerequisites

1. On-premise Orchestratror fully deployed on the system
   1. Edge Network is configured with a bridge interface `virbr1`
1. `kubectl` is configured with root permissions to manage the Kubernetes cluster
1. `nmap` is installed on the system to verify network connectivity
   1. On Ubuntu, install with `sudo apt install nmap`

### Steps

1. Create a pod the deploys a DHCP server with host networking

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: dhcp-server
  namespace: default
spec:
  hostNetwork: true  # Use the host's network namespace
  containers:
  - name: dhcp-server
    image: alpine:latest
    # env:
    # - name: http_proxy
    #   value: ""  # Optional, if you need to set a proxy
    # - name: https_proxy
    #   value: ""  # Optional, if you need to set a proxy
    # - name: no_proxy
    #   value: "localhost,127.0.0.1"
    ports:
      - containerPort: 67
        hostPort: 67
        protocol: UDP
      - containerPort: 68
        hostPort: 68
        protocol: UDP
    command: ["sh", "-c"]
    args:
    - |
      # Install dnsmasq for DHCP server and PXE boot options
      apk add --no-cache dnsmasq

      # Create dnsmasq configuration file to add PXE TFTP server option
      cat > /etc/dnsmasq.conf << EOF
      # Enable logging for troubleshooting
      log-queries
      log-dhcp

      # Disable DNS functionality, we only want DHCP
      port=0

      # Enable DHCP server functionality - assign addresses directly
      dhcp-range=192.168.99.100,192.168.99.200,255.255.255.0,12h

      # Use the node's network interface that is connected to network where Edge Nodes will connect
      interface=ens3 # Replace with your actual network interface name
      bind-interfaces

      # PXE boot options configuration
      # Option 66 (next-server) and Option 67 (bootfile-name)
      dhcp-option-force=66,192.168.99.40
      dhcp-option-force=67,signed_ipxe.efi

      # Alternative PXE boot configuration - works with most PXE clients
      dhcp-boot=signed_ipxe.efi,tinkerbell-nginx.cluster.onprem,192.168.99.40

      # TFTP server is disabled, we're only providing DHCP PXE options
      # enable-tftp
      # tftp-root=/var/lib/tftp
      EOF

      # Start dnsmasq service to handle DHCP with PXE options
      dnsmasq --no-daemon --log-queries=extra --log-dhcp --log-debug --conf-file=/etc/dnsmasq.conf
    securityContext:
      capabilities:
        add: ["NET_ADMIN"]  # Required for DHCP server to manage network interfaces
      # allowPrivilegeEscalation: false
      # readOnlyRootFilesystem: true
      # runAsUser: 1000
      # runAsGroup: 1000
      # runAsNonRoot: true
      seccompProfile:
          type: RuntimeDefault
  restartPolicy: OnFailure  # Ensures the pod restarts if it crashes
EOF
```

1. View the logs of the DHCP server pod to ensure it is running correctly

```shell
kubectl logs -f dhcp-server
```

1. Modify the virtual network to disable DHCP so the DHCP server in the pod can handle DHCP requests

```shell
# Open terraform/edge-network/main.tf and set dhcp.enabled to false

# Apply the changes to the edge network configuration
mage deploy:edgenetwork
```

1. Verify that the DHCP server is responding to DHCP requests on the bridge interface `virbr1`

```shell
sudo nmap -e virbr1 --script broadcast-dhcp-discover --script-args=timeout=20s
```

1. Verify output from the `nmap` command, which should show the DHCP server responding with the PXE boot options:

```plaintext
sudo nmap -e virbr1 --script broadcast-dhcp-discover --script-args=timeout=20s
Starting Nmap 7.80 ( https://nmap.org ) at 2025-06-06 22:04 UTC
Pre-scan script results:
| broadcast-dhcp-discover: 
|   Response 1 of 1: 
|     IP Offered: 192.168.99.117
|     DHCP Message Type: DHCPOFFER
|     Server Identifier: 192.168.99.10
|     IP Address Lease Time: 2m00s
|     TFTP Server Name: 192.168.99.40\x00
|     Bootfile Name: signed_ipxe.efi\x00
|     Renewal Time Value: 1m00s
|     Rebinding Time Value: 1m45s
|     Subnet Mask: 255.255.255.0
|     Broadcast Address: 192.168.99.255
|_    Router: 192.168.99.10
WARNING: No targets were specified, so 0 hosts scanned.
Nmap done: 0 IP addresses (0 hosts up) scanned in 3.38 seconds
```

### Conclusion

The above steps demonstrate how to deploy a DHCP server in a Kubernetes pod using host networking, configure it to
provide PXE boot options, and verify that it is functioning correctly. This setup allows for Layer 2 networking in the
Orchestrator environment, proving that it is possible to enable Edge Nodes to boot from the network where a DHCP server
is deployed inside of Kubernetes.

### Clean up

```shell
kubectl delete pod dhcp-server

# Re-enable DHCP in the virtual network configuration
mage deploy:edgenetwork
```
