# Testing

## Local

When testing platform deployment in the Coder environment, a default mage command is executed:

```sh
export ORCH_DEFAULT_PASSWORD="<add-value-here>"
mage test:e2e
```

## sc-dev

In order to execute the tests in `sc-dev` environment, the command should be modified to specify a domain name using an environment variable:

```sh
rm -f jwt.txt
export ORCH_DEFAULT_PASSWORD="<add-value-here>"
export E2E_SVC_DOMAIN="sc-dev.orch.intel.com"
mage test:e2e
unset E2E_SVC_DOMAIN
```

Please note that subsequent executions in the same environment, do not need to remove the `jwt.txt` file.

## integration

In `integration` environment, execute this command:

```sh
rm -f jwt.txt
export ORCH_DEFAULT_PASSWORD="<add-value-here>"
export E2E_SVC_DOMAIN="integration.orch.intel.com"
mage test:e2e
unset E2E_SVC_DOMAIN
```

## offline environment

These tests can be executed from the Coder connecting to the Offline server.

1. Add `/etc/hosts` entries for `offline.lab` targets in Coder (can use 127.0.0.1 when using jumphost tunnel shown below)
1. Setup tunneling via jumphost assuming that 21.41 is a jumphost and 21.164 is the Orchestrator VM
(e.g. ssh -CT -L 8006:10.23.21.41:8006 -L 3022:10.23.21.164:22 -L 3044:10.23.21.164:443 smartedge@10.23.21.49)
1. Extract CA cert from the offline server after each deployment.

```sh
kubectl get secret -n orch-platform orch-ca -o json | jq '.data."orch-ca"' | tr -d '"' | base64 -d
```

or copy contents of this file:

```sh
cat /home/root/offline-ca.crt
```

1. Save the CA cert on Coder in file `offline-ca.crt`

1. Run this mage command on Coder to add the offline CA cert to the local trust store:

```sh
mage offline:addCA
```

1. Execute the tests:

```sh
rm -f jwt.txt

export ORCH_DEFAULT_PASSWORD="<add-value-here>"
export E2E_SVC_DOMAIN="offline.lab"
export E2E_SVC_PORT="3044"
export no_proxy=$no_proxy,offline.lab
export NO_PROXY=$NO_PROXY,offline.lab
mage test:e2e

unset E2E_SVC_DOMAIN
unset E2E_SVC_PORT
unset ORCH_DEFAULT_PASSWORD
unset E2E_VAULT_TOKEN
```

## all environment

For all other environments including `demo` environment, keycloak password and vault token should be provided in addition to the custom domain value.

```sh
rm -f jwt.txt
export E2E_VAULT_TOKEN="<add-value-here>"
export ORCH_DEFAULT_PASSWORD="<add-value-here>"
export E2E_SVC_DOMAIN="demo.orch.intel.com"
export E2E_SVC_PORT="30443"
mage test:e2e
unset E2E_SVC_DOMAIN
unset ORCH_DEFAULT_PASSWORD
unset E2E_VAULT_TOKEN
````
