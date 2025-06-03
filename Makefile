ORG=sample-org
PROJECT=sample-project
PREFIX=${PROJECT}
PROJECTID = $(shell kubectl get projects -o yaml|yq '.items[0].status.projectStatus.uID')
NODEGUID = $(shell kubectl -n enic exec enic-0 -c edge-node -- dmidecode -s system-uuid|tr -d ' \n')

all: help

.PHONY: deploy
deploy: deploy-orch deploy-edge ## Deploy orchestrator and cluster

.PHONY: deploy-orch
deploy-orch: ## Deploy only orchestrator
	mage -v deploy:kindPreset ../scorch/presets/dev-internal-coder-minimal.yaml
	mage deploy:waitUntilComplete
	mage gen:orchCa deploy:orchCa router:stop router:start
	mage tenantUtils:createDefaultMtSetup

.PHONY: deploy-edge
deploy-edge: ## Deploy ENiC and create a cluster
	mage deploy:edgeCluster dev-internal-coder-minimal

.PHONY: undeploy-edge
undeploy-edge: ## Delete cluster and remove ENiC
	mage undeploy:EdgeCluster ${ORG} ${PROJECT}
	@echo "Deleting PVC"
	@kubectl -n enic delete pvc rancher-vol-enic-0

.PHONY: cluster-create
cluster-create: ## Create a cluster
	PROJECT=${PROJECT} mage coUtils:createCluster demo-cluster ${NODEGUID}

.PHONY: cluster-delete
cluster-delete: ## Delete a cluster
	PROJECT=${PROJECT} mage coUtils:deleteCluster demo-cluster

.PHONY: cluster-status
cluster-status: ## Show the cluster status
	clusterctl describe cluster demo-cluster -n ${PROJECTID} --show-conditions all

.PHONY: agent-logs
agent-logs: ## Tail -f the cluster agent logs
	kubectl -n enic exec enic-0 -c edge-node -- journalctl -xefu cluster-agent

.PHONY: kubeconfig
kubeconfig: ## Get kubeconfig.yam file for the cluster
	@clusterctl get kubeconfig demo-cluster -n ${PROJECTID} > kubeconfig.yaml
	@sed -i 's|http://[[:alnum:]\.-]*:8080/|https://connect-gateway.kind.internal:443/|' kubeconfig.yaml
	@sed -i "s|certificate-authority-data: [[:alnum:]=]*|certificate-authority-data: $(shell base64 -w 0 orch-ca.crt)|" kubeconfig.yaml

.PHONY: smoke-test
smoke-test: ## Run CO smoke test from orch-deploy repo
	ORCH_DEFAULT_PASSWORD="ChangeMeOn1stLogin!" \
                PROJECT=${PROJECT} \
                NODE_UUID=${NODEGUID} \
                EDGE_MGR_USER=${PROJECT}-edge-mgr \
                EDGE_INFRA_USER=${PROJECT}-api-user \
                CLUSTER_NAME=demo-cluster \
                mage test:clusterOrchSmokeTest

.PHONY: deploy-enic
deploy-enic: ## Deploy just ENIC
	mage devUtils:deployEnic 1 dev-internal-coder-minimal
	mage devUtils:registerEnic enic-0
	#mage devUtils:provisionEnic enic-0
	sleep 5
	mage devUtils:WaitForEnic

.PHONY: delete-enic
delete-enic: ## Delete the ENIC
	mage devUtils:deployEnic 0 dev-internal-coder-minimal
	kubectl -n enic delete pvc rancher-vol-enic-0

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
