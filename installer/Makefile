# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Import environment variables from .env file if .env file exists
ifeq (,$(wildcard .env))
$(info No local .env cache present)
else
# Import environment variables from .env file
include .env
$(info Loading local .env cache)
endif

SHELL            := /bin/bash
MAKEDIR          := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
M                := $(MAKEDIR)/milestones

# Install within the installer container using the source pushed to CodeCommit
EDGE_MANAGEABILITY_FRAMEWORK_DIR  ?= ${HOME}/src/edge-manageability-framework
###

CLUSTER_CONFIG=${HOME}/src/edge-manageability-framework/orch-configs/clusters/${CLUSTER_NAME}.yaml

INTEL_HARBOR_CACERT := ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/mage/intel-harbor-ca.crt

ORCH_ISTIO_NAMESPACES := orch-infra orch-iam orch-app orch-cluster orch-ui orch-platform orch-harbor orch-gateway orch-sre orch-database

ORCH_NAMESPACES  := cattle-system
ARGO_NAMESPACES  := dev

# Default to blank cluster, raise an error if not specified. Future UI options?
CLUSTER_NAME   ?=
TARGET_ENV     ?= ${CLUSTER_NAME}

ARGO_IP   ?= 127.0.0.1
ARGO_PORT ?= 32443
ARGO_VERSION ?= 7.4.4

HARBOR_PASSWORD := $(shell head -c 512 /dev/urandom | tr -dc A-Za-z0-9 | cut -c1-100)
KEYCLOAK_PASSWORD := $(shell  head -c 512 /dev/urandom | tr -dc 'A-Za-z0-9!@#$%^&*()_+|:<>?=' | cut -c1-14)

ifeq (${USE_REPO_PROXY},true)
    # Commands to run if USE_REPO_PROXY is true
    REPO_PROXY := --proxy ${https_proxy}
endif

## TBD: This shouldn't be a flag. It should be a parameter containing a reference to a proxy configuration that can be used
##      to render this bootstrap file from a template.
ifeq (${USE_PROXY},true)
    # Commands to run if USE_REPO_PROXY is true
    ## ARGO_PROXY := --values ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/bootstrap/internal-proxy.yamln/main
endif

ifeq (${USE_TEST_ADMIN},true)
    # Commands to run if USE_REPO_PROXY is true
    ORCH_DEFAULT_PASSWORD ?= $(shell openssl rand -base64 32 | tr -dc 'A-Za-z0-9' | head -c 16)
    ARGO_AUTH_BCRYPT := $(shell argocd account bcrypt --password ${ORCH_DEFAULT_PASSWORD} )
    ARGO_TEST_SESSION := --set configs.cm.users.session.duration=24h
endif

## TBD: This shouldn't be a flag. It should be a parameter containing a reference to the Registry and Cert files
ifeq (${USE_REGISTRY_CERTS},true)
    ## Commands to run if USE_REGISTRY_CERTS is true
	## TBD: This needs to be passed in as a generalized custom Registry and optional Registry Cert parameter
    ## ARGO_REGISTRY_CERTS := --values ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/bootstrap/amr-harbor.yaml
endif

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

${M}:
	mkdir -p ${M}

##@ Installation process
${M}/create-namespaces: | ${M}
	for ns in ${ORCH_ISTIO_NAMESPACES}; do \
		kubectl create ns $$ns --dry-run=client -o yaml | kubectl apply -f -; \
		kubectl label namespace $$ns istio-injection=enabled --overwrite; \
	done
	for ns in ${ORCH_NAMESPACES} ${ARGO_NAMESPACES}; do \
		kubectl create ns $$ns --dry-run=client -o yaml | kubectl apply -f -; \
	done
	touch $@

# These should now be handled by ASM and the harbor-bootstrap chart. Keeping this for potential dev/onprem flow
# NOTE: Currently the harbor-admin-credential does not seem to be created anywhere so it is enabled.
${M}/release-secrets: | ${M}/create-namespaces
	@if ! kubectl get secret harbor-admin-password -n orch-harbor > /dev/null 2>&1; then \
		kubectl create secret generic -n orch-harbor harbor-admin-password --from-literal=HARBOR_ADMIN_PASSWORD="$(HARBOR_PASSWORD)" --dry-run=client -o yaml | kubectl apply -f - ;\
	fi
	@if ! kubectl get secret harbor-admin-credential -n orch-harbor > /dev/null 2>&1; then \
		kubectl create secret generic -n orch-harbor harbor-admin-credential --from-literal=credential=admin:"$(HARBOR_PASSWORD)" --dry-run=client -o yaml | kubectl apply -f - ;\
	fi
	touch $@

${M}/keycloak-secret: | ${M}/create-namespaces
	@if ! kubectl get secret platform-keycloak -n orch-platform > /dev/null 2>&1; then \
		kubectl create secret generic -n orch-platform platform-keycloak --from-literal=admin-password="$(KEYCLOAK_PASSWORD)" --dry-run=client -o yaml | kubectl apply -f - ;\
	fi
	touch $@

${M}/argocd-release: | ${M}/create-namespaces ${M}/release-secrets ${M}/keycloak-secret
	@helm repo add argo-helm https://argoproj.github.io/argo-helm --force-update
	@helm -n argocd upgrade --install argocd argo-helm/argo-cd --version ${ARGO_VERSION} \
		--values ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/bootstrap/argocd.yaml --values ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/bootstrap/nodeport.yaml \
		${ARGO_PROXY} ${ARGO_REGISTRY_CERTS} ${ARGO_TEST_SESSION} \
		--create-namespace --wait
ifeq (${USE_TEST_ADMIN},true)
	@kubectl -n argocd patch secret argocd-secret --type='merge' -p '{"stringData": {"admin.password": "${ARGO_AUTH_BCRYPT}"}}'
endif
	touch $@

${M}/argocd-ready: | ${M}/argocd-release
	${HOME}/await-argo.sh
	touch $@

${M}/deploy-orch: | ${M}/argocd-ready
	@[ -z "${TARGET_ENV}" ] && echo "*** TARGET_ENV not set" && exit 1 || true
	helm upgrade --install root-app ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/argocd/root-app -n ${TARGET_ENV} -f ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/orch-configs/clusters/${TARGET_ENV}.yaml --create-namespace
	touch $@

install: ${M}/deploy-orch ## Install the Edge Orchestrator software

update:
	@[ -z "${TARGET_ENV}" ] && echo "*** TARGET_ENV not set" && exit 1 || true
	helm upgrade --install root-app ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/argocd/root-app -n ${TARGET_ENV} -f ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/orch-configs/clusters/${TARGET_ENV}.yaml --create-namespace

upgrade: | ${M}/release-secrets ${M}/argocd-ready
	helm repo add argo-helm https://argoproj.github.io/argo-helm --force-update
	helm -n argocd upgrade --install argocd argo-helm/argo-cd --version ${ARGO_VERSION} \
		--values ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/bootstrap/argocd.yaml \
		--values ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/bootstrap/nodeport.yaml ${ARGO_PROXY} ${ARGO_REGISTRY_CERTS} ${ARGO_TEST_SESSION} \
		--create-namespace --wait
	@[ "${USE_TEST_ADMIN}" = "true" ] && kubectl -n argocd patch secret argocd-secret --type='merge' -p '{"stringData": {"admin.password": "${ARGO_AUTH_BCRYPT}"}}';
	@[ -z "${TARGET_ENV}" ] && echo "*** TARGET_ENV not set" && exit 1 || true
	helm upgrade --install root-app ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/argocd/root-app -n ${TARGET_ENV} -f ${EDGE_MANAGEABILITY_FRAMEWORK_DIR}/orch-configs/clusters/${TARGET_ENV}.yaml --create-namespace

clean: ## Remove the milestones directory
	rm -rf ${M}
