# On-Premise Orchestrator

This folder stores the code for the On-premise Orchestrator.

It includes:

1. Install/uninstall scripts
1. OS distribution packages
1. Automated tests
1. Documentation
1. Automation that builds and publishes release artifacts for distribution
1. ...and more!

## Development

### Running ⚡

The On-premise Orchestrator can be deployed to a VM for development purposes using [Libvirt](orchestrator).

At the end of this guide, you will have a full Orchestrator ready to provision Edge Nodes.

#### Setup tools

The [asdf](https://asdf-vm.com/) is used to manage development dependencies for this repository.
It is assumed that your development machine is POSIX compatible and has `git` installed.

1. Install [asdf](https://asdf-vm.com/guide/getting-started-legacy.html)

1. Clone the repository the orchestrator repository.

1. Install asdf plugins

    ```shell
    ./installer/asdf-install-plugins
    ```

1. Install development dependencies

    ```shell
    asdf install
    ```

1. You're on fire 🔥, keep going! 👇

#### Setup variables

We use Terraform to provision the infrastructure. Before we deploy Orchestrator's underlying infrastructure, we need
to supply some required values to the Terraform configuration. Passing variable values to Terraform is typically done
using a `.tfvars` file.

A [terraform/terraform.tfvars](terraform/terraform.tfvars) file has been created, and some default values may have
been supplied. This is the file you're going to use to pass into the Terraform configuration that will bring up
Orchestrator in the next section.

You'll eventually want to update this file with your own values depending on what you're trying to do. If you would
like to use your own `.tfvars` file, please export the TF_VARS_FILE variable

```shell
export TF_VAR_FILE="<path-to-file>"
```

1. Re-review this section; it actually is important for the 🧠

#### Create Orchestrator

You're going to execute the Terraform configuration that will bring up the infrastructure and kick off the
installation of Orchestrator. Let's break down what Terraform will do:

- A VM with minimum baseline resources will be created to deploy the Orchestrator
- Ubuntu Server 22.04 will be imaged to the first hard disk aka the boot disk
- The VM will be booted and `cloud-init` will configure the OS
- `cloud-init` will start running the Orchestrator install script in a [screen session](https://www.gnu.org/software/screen/)

Execute the following instructions on your development machine.

1. Use mage command to deploy On Premise Deployment.

    ```shell
   mage deploy:onPrem
    ```

1. Get the SSH IP address of the VM and save for later

    ```shell
    cat << EOF >> .env
    export MY_ORCH_SSH_IP=$(terraform -chdir=terraform/ output -json | jq -r .ssh_host.value)
    EOF

    source .env

    echo "My Orchestrator's SSH IP is ${MY_ORCH_SSH_IP}"
    ```

1. Copy your SSH key to the VM for password-less login

    ```shell
    # When prompted, type password `ubuntu`
    ssh-copy-id "ubuntu@${MY_ORCH_SSH_IP}"
    ```

#### Setup local DNS and proxy overrides

On your development machine, you're going to need to be able to resolve the DNS name `*.cluster.onprem` to the IP
address that Orchestrator's Traefik is serving on.

Execute the following instructions on your development machine.

1. Add to your `/etc/hosts` file so that DNS lookups resolve to Orchestrator's Traefik IP address

    ```shell
    cat << EOF
    ${TRAEFIK_IP} "alerting-monitor.cluster.onprem"
    ${TRAEFIK_IP} "api.cluster.onprem"
    ${TRAEFIK_IP} "app-orch.cluster.onprem"
    ${TRAEFIK_IP} "app-service-proxy.cluster.onprem"
    ${ARGO_IP} "argocd.cluster.onprem"
    ${TRAEFIK_IP} "cluster-orch-edge-node.cluster.onprem"
    ${TRAEFIK_IP} "cluster-orch-node.cluster.onprem"
    ${TRAEFIK_IP} "cluster-orch.cluster.onprem"
    ${TRAEFIK_IP} "cluster.onprem"
    ${TRAEFIK_IP} "connect-gateway.cluster.onprem"
    ${TRAEFIK_IP} "fleet.cluster.onprem"
    ${TRAEFIK_IP} "infra-node.cluster.onprem"
    ${TRAEFIK_IP} "keycloak.cluster.onprem"
    ${TRAEFIK_IP} "license-node.cluster.onprem"
    ${TRAEFIK_IP} "log-query.cluster.onprem"
    ${TRAEFIK_IP} "logs-node.cluster.onprem"
    ${TRAEFIK_IP} "metadata.cluster.onprem"
    ${TRAEFIK_IP} "metrics-node.cluster.onprem"
    ${TRAEFIK_IP} "observability-admin.cluster.onprem"
    ${TRAEFIK_IP} "observability-ui.cluster.onprem"
    ${TRAEFIK_IP} "onboarding-node.cluster.onprem"
    ${TRAEFIK_IP} "onboarding-stream.cluster.onprem"
    ${TRAEFIK_IP} "onboarding.cluster.onprem"
    ${TRAEFIK_IP} "orchestrator-license.cluster.onprem"
    ${TRAEFIK_IP} "registry-oci.cluster.onprem"
    ${TRAEFIK_IP} "registry.cluster.onprem"
    ${TRAEFIK_IP} "release.cluster.onprem"
    ${TRAEFIK_IP} "telemetry-node.cluster.onprem"
    ${NGINX_IP} "tinkerbell-nginx.cluster.onprem"
    ${NGINX_IP} "tinkerbell-server.cluster.onprem"
    ${TRAEFIK_IP} "update-node.cluster.onprem"
    ${TRAEFIK_IP} "vault.cluster.onprem"
    ${TRAEFIK_IP} "vnc.cluster.onprem"
    ${TRAEFIK_IP} "web-ui.cluster.onprem"
    ${TRAEFIK_IP} "ws-app-service-proxy.cluster.onprem"
    EOF
    ```

1. Ensure you add `.cluster.onprem` to any `no_proxy` environment variables and OS settings.

#### Connecting to the Orchestrator

1. Setup another proxy within the current proxy (#TunnelInception) 🤯

    ```shell
    # In your dev machine in a dedicated terminal. Keep this running!
    sshuttle -r "ubuntu@${MY_ORCH_SSH_IP}" 192.168.1.0/24
    ```

1. SSH to the Orchestrator VM to establish a session

    ```shell
    # In your development machine in another terminal
    ssh -v "ubuntu@${MY_ORCH_SSH_IP}"
    ```

1. Attach the screen session running the Orchestrator installer

    ```shell
    # In your Orchestrator VM once you establish an SSH session in the previous step
    screen -d -r

    # To detach from the screen session, press Ctrl-A then d

    # View logs to see progress with journalctl
    sudo journalctl -f
    ```

1. Wait 10 minutes. Eventually, you will be able to log in to the [Argo CD UI](https://argo.cluster.onprem) using user
   `admin` with the password from the secret

    ```shell
    # In your development machine
    ssh "ubuntu@${MY_ORCH_SSH_IP}" kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 --decode && echo
    ```

1. Wait patiently ⌛. Installation of the full Orchestrator takes about 25 minutes. Afterwards, you'll be able to reach the
Orchestrator UI via the URL:

<https://web-ui.cluster.onprem>

Congrats, you should now have a fully functional on-premise Orchestrator! 🎉🎉🎉

#### Destroying Orchestrator

From your development machine, bringing down the Orchestrator is as simple as:

```shell
mage undeployOnPrem
```

It is a good practice to destroy your Orchestrator when you are not using it.
