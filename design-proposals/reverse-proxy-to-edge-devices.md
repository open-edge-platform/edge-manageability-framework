# Design Proposal: Reverse Proxy to Edge Nodes

Author(s): [Frątczak Wiktor]

Last updated: [03.07.2025]

## Abstract

The Intel Open Edge platform orchestrates workloads on distributed edge devices.
In practice, these devices are often deployed in hard-to-reach locations such as factories,
telecom sites, retail outlets, or offshore wind farms, where physical access is limited and costly.
Moreover, edge nodes are typically placed behind firewalls and private networks,
complicating secure connectivity.

To address these challenges, enterprise operators require a standardized way to establish secure,
on-demand SSH sessions for diagnostics and maintenance.
This document proposes extending Intel Open Edge with native SSH session management,
providing scalable and controlled remote access across diverse deployment scenarios.

## Proposal Summary

This proposal extends Intel Open Edge with a remote access workflow for SSH connectivity.
Each edge node runs a lightweight Remote Access Agent that periodically polls the orchestrator
and, once authorized, establishes a persistent connection to the Remote Access Proxy.
On the orchestrator side, a Remote Access Manager coordinates the session lifecycle,
while the Remote Access Proxy acts as a gateway that bridges SSH connections and
exposes a WebSocket interface for the UI. The Inventory service is extended to track device
availability and session configurations.

## Requirements

To support secure and scalable remote access to edge nodes, the solution must:

* Allow users to initiate secure on-demand SSH sessions without direct exposure of edge devices.
* Provide a lightweight agent on each node, capable of establishing and maintaining an on-demand connection with the
  orchestrator’s Remote Access Proxy.
* Enable the orchestrator to control the full session lifecycle (creation, updates, timeout, teardown).
* Enhance the Inventory service to map session requests to registered devices and track its states.
* Expose a new synchronous HTTP orchestrator endpoint for uprościć na „initiating and terminating sessions.
* Enforce validation and access control to guarantee secure operations across diverse deployment environments.

## Proposed changes

The following modifications are introduced to Intel Open Edge:

* Remote Access Agent – lightweight service on each edge node that polls the Remote Access Manager
  for authorized session requests and, when instructed, establishes an on-demand persistent connection
  to the Remote Access Proxy for the duration of the session.
* Remote Access Manager – control-plane component that processes user/API requests, creates and updates session
  configurations in the Inventory service, and responds to agent polls with directives to establish or terminate
  connections based on stored session data.
* Remote Access Proxy – a scalable orchestrator-side gateway that terminates agent connections, exposes
  temporary SSH endpoints for clients and provides a WebSocket interface for browser-based access.
* Inventory Integration – the orchestrator’s Inventory Database is extended with new model for session
  configuration, device registration and state tracking.
* New API Endpoint – an orchestrator API that allows operators to initiate, monitor, and terminate
  sessions via UI or terminal tools that uses websocket.

[//]: # (TODO: TBA When remote access config will be created how should Remote Access Manager know about this, should it ask inventory periodically - imo worse, or should the inventory itself notify manager about it)

## High Overview Diagram

![img.png](images/remote-access-proxy-arch.png)

## Required Flow

1. User requests SSH session (API/UI) to specific edge Node.
2. API perform request to Remote Access Manager to create a remote access config instance.
3. Manager creates a session config and stores it in Inventory.
4. Remote Access Agent polls Manager for new configs, if its present Manager responds with one.
5. The Agent establishes an on-demand persistent tunnel to the Remote Access Proxy using the received configuration.
6. The Remote Access Proxy registers the tunnel and updates the session state to STARTED in the Inventory.

[//]: # (   Should RAP do it by itself or via Remote Access Manager )

7. Remote Access Proxy initialize reverse SSH connection to the Edge Node and exposes a temporary endpoint for the user.
8. User gets an endpoint to connect to the desired SSH connection.
9. The user connects to the Proxy endpoint, which forwards traffic through the tunnel to the edge node’s SSH server.

## Minimal Database changes

    RemoteAccessConfiguration {
      +string resource_id
      +InstanceResource instance
      +uint64 expiration_timestamp
      +uint32 local_port (optional)
      +string token (optional)
      +RemoteAccessState current_state (optional)
      +RemoteAccessState desired_state
      +string configuration_status (optional)
      +StatusIndication configuration_status_indicator (optional)
      +uint64 configuration_status_timestamp (optional)
      +string tenant_id
      +TIMESTAMP created_at
      +TIMESTAMP updated_at
    }

    RemoteAccessState {
      <<enum>>
      REMOTE_ACCESS_STATE_UNSPECIFIED = 0
      REMOTE_ACCESS_STATE_DELETED     = 1
      REMOTE_ACCESS_STATE_ERROR       = 2
      REMOTE_ACCESS_STATE_CONFIGURED  = 3
      REMOTE_ACCESS_STATE_STARTED     = 4
    }

## Security considerations

In remote access systems like Intel Open Edge, the critical design question is how to
authenticate and authorize both users and devices before establishing a session.

First we need to differentiate 4 types of communication that requires Auth mechanisms:

1. Internal communication inside Orchestrator
   <br></br>
   Agent will use access token generated by node Agent, as in current implementation
   <br></br>

2. Agent <--> Remote Access Manager
   <br></br>
   Agent will use access token generated by node Agent, as in current implementation
   <br></br>

3. Agent <--> Remote Access Proxy
   <br></br>
   Here we have 2 possibilities, we could use either Ephemeral certificates or SSH keys
   <br></br>

4. User <--> Remote Access Proxy
   <br></br>

### Reverse SSH as a Secure Mechanism for Enabling Access Between Edge Devices via Orchestrator

In distributed edge environments, devices are often deployed behind NATs or firewalls, which makes direct inbound
connections from external networks impractical or even impossible. **Reverse SSH** provides a robust and secure
mechanism to establish such connectivity.  
In this approach, the **edge device initiates an outbound SSH connection** to a central component within the
orchestrator’s domain — the **proxy manager**. Since most networks allow outbound connections, this method bypasses NAT
traversal and eliminates the need for port forwarding or VPN tunnels.

Once the reverse SSH tunnel is established, the orchestrator can securely route client connections through the proxy
manager to the target edge device. This enables administrators or automated services to execute remote commands,
transfer files, or interact with device terminals without exposing the device’s SSH port directly to the internet.

The reverse SSH mechanism also simplifies **session lifecycle management**: tunnels can be opened and closed on demand,
logged, and automatically cleaned up after a timeout or orchestration event. This approach aligns well with the *
*ephemeral, dynamic nature** of edge environments, where devices may frequently join or leave the network.

---

### Chisel as a WebSocket Transport Layer for Reverse SSH

While traditional reverse SSH relies on TCP, modern edge environments often benefit from using 
**WebSocket-based tunnels**, which can seamlessly traverse corporate proxies and restrictive firewalls.  
**Chisel** is an open-source, lightweight tunneling tool that supports exactly this functionality. In this project,
Chisel is not used as a standalone CLI tool but is **integrated as a Go library** within the `remote-access-proxy` and
`edge node agent`components, forming the **data plane** for all SSH reverse tunnels.

Chisel encapsulates SSH traffic inside a **WebSocket stream**, allowing the proxy manager to handle both SSH and
web-based client connections in a unified manner. This design enables advanced capabilities such as:

- **Bidirectional multiplexing** of multiple SSH sessions over a single WebSocket connection,
- **Secure session termination** and monitoring from the orchestrator layer,
- Integration with web interfaces (e.g., **xterm.js**) for browser-based SSH access,
- Efficient use of existing infrastructure — since WebSockets typically reuse port 443, no additional firewall rules are
  needed.

Using Chisel as a WebSocket transport layer provides a modern, cloud-native foundation for remote edge access — one that
remains compatible with HTTP-based load balancers, ingress controllers, and service meshes.

---

### Ephemeral Certificates vs SSH Keys

Traditional SSH authentication relies on **static key pairs** — long-lived public/private keys that must be distributed
and managed across devices. While simple to implement, static keys pose several operational and security challenges:

- Difficult key rotation and revocation,
- Risk of unauthorized reuse or leakage,
- Lack of auditability and time-bound access control.

To address these limitations, modern systems increasingly adopt **ephemeral SSH certificates**, typically issued by a *
*centralized certificate authority (CA)** managed by the orchestrator.  
Each time a session is initiated, the orchestrator dynamically issues a **short-lived certificate** (e.g., valid for a
few minutes), which authenticates the user or service to the target edge device.

This approach provides several advantages:

- **Strong security guarantees** — certificates automatically expire, reducing the attack surface,
- **Centralized access control** — orchestrator can define policies (roles, scopes, expiration times) for issued
  certificates,
- **Better traceability** — every access request is tied to a unique certificate identity, simplifying auditing and
  compliance,
- **Seamless integration** with reverse SSH — ephemeral credentials can be issued at tunnel setup time, avoiding
  persistent secrets on devices.
- **Rich contextual information** — certificates can embed metadata such as tenant ID, role, permissions, and session
  policy.

### Multitenancy and Context-Aware Access Control

In multi-tenant edge environments, where multiple organizations share the same orchestration platform, **tenant
isolation and contextual access control** are critical.  
Ephemeral certificates enable this by carrying additional identity metadata within their fields (e.g., as certificate
extensions or principals). This metadata may include:

- **Tenant ID or namespace** — allowing the proxy and orchestrator to enforce tenant-level isolation,
- **User role or privilege scope** — ensuring that certificates only grant the minimal required access,
- **Session-specific policy references** — binding each session to predefined authorization rules or resource quotas.

The orchestrator acts as a **central trust authority**, validating these fields before allowing session establishment
through the proxy manager. As a result, access control decisions become **context-aware**, combining certificate
identity with orchestration policies.

This design aligns with the **zero-trust security model**, where trust is not implicitly granted based on network
location but dynamically derived from verified identity, role, and session context. It provides a scalable mechanism to
manage access across many tenants, devices, and users — without relying on long-lived shared credentials.

## Scalability Considerations

Scalability is a critical aspect of enabling secure remote access in large-scale edge deployments.  
In environments where thousands of edge devices maintain reverse SSH tunnels, the system must efficiently manage
connections, sessions, and certificate lifecycles without becoming a bottleneck.

Several architectural mechanisms contribute to scalability:

- **Connection multiplexing** — multiple SSH sessions can share a single WebSocket connection through Chisel’s built-in
  multiplexing layer, significantly reducing the number of open sockets, TLS handshakes, and context switches per
  device.
  This allows the orchestrator to maintain connectivity with a large number of edge nodes using minimal system
  resources.

- **Ephemeral session model** — user-facing connections (e.g., from the UI or API gateway to the remote access proxy)
  are
  short-lived and authenticated using ephemeral SSH certificates. Each certificate is valid only for the duration of a
  single interactive session, typically a few minutes. This model minimizes resource retention and ensures that expired
  sessions cannot be reused.

- **Differentiated certificate lifetimes** — the system distinguishes between:
    - **Longer-lived certificates** used for the **edge-node ↔ remote access proxy** channel.  
      These certificates are tenant-scoped and can be shared among the tenant’s internal services or engineers
      responsible
      for maintaining devices. They enable the persistent reverse tunnel between the edge node and the proxy to stay
      open
      without frequent re-authentication, which is critical for network stability and scalability.
    - **Short-lived certificates** used for the **UI ↔ remote access proxy** channel.  
      These are ephemeral user session credentials that provide time-bound, auditable access to active tunnels. They are
      issued per session and automatically revoked upon timeout or logout, reducing the attack surface and supporting
      just-in-time access control.

- **Stateless orchestration layer** — session metadata, certificate mappings, and tunnel state are persisted in a
  distributed data store (e.g., etcd or PostgreSQL). This allows horizontal scaling of multiple `remote-access-proxy`
  replicas
  without maintaining shared in-memory state, enabling reliable load balancing and failover.

- **Load-aware routing** — the orchestrator can distribute tunnel creation and connection requests across multiple
  `remote-access-proxy` instances based on CPU load, memory usage, or active connection count. This avoids bottlenecks
  and
  supports linear scalability with the number of deployed proxies.

- **Tenant-based segmentation** — each tenant’s sessions and tunnels are separated by using different ports on
  remote-access-proxy site, ensuring isolation, dedicated resource quotas, and clear observability boundaries. This is
  critical in multi-tenant environments, where
  shared infrastructure must still enforce strict access isolation.

- **Graceful degradation** — when system capacity approaches defined thresholds, new session requests can be queued,
  rate-limited, or temporarily rejected with appropriate feedback to the user or orchestrator. This ensures that the
  system degrades predictably under heavy load rather than failing abruptly.

This design balances **security, stability, and scalability**. Persistent tunnels between edge nodes and proxies ensure
continuous reachability, while ephemeral user sessions provide secure, auditable, and resource-efficient access to those
connections through the orchestration layer.

### Multiplexing one ports for few sessions at once

## Rationale

[A discussion of alternate approaches that have been considered and the trade
offs, advantages, and disadvantages of the chosen approach.]

## Affected components and Teams

## Implementation plan

[A description of the implementation plan, who will do them, and when.
This should include a discussion of how the work fits into the product's
quarterly release cycle.]

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]
