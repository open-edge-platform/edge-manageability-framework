# Buidall Makefile

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

SHELL	:= bash -eu -o pipefail

# default goal to show help
.DEFAULT_GOAL := help

# project name
PROJECT_NAME := buildall

# github organization
GITHUB_ORG  ?= open-edge-platform

# list of repos, regeneratable, but usually static
REPO_LIST   := repo_list

# directories
SCRIPTS_DIR := scripts

# temp dirs
REPO_DIR    := repos
SCRATCH_DIR := scratch
CHARTS_DIR  := charts

$(REPO_DIR):
	@mkdir -p $@

$(SCRATCH_DIR):
	@mkdir -p $@

$(CHARTS_DIR):
	@mkdir -p $@

check: ## check for prereqs (currently noOp)

lint: license shellcheck pylint black markdownlint ## Lint everything

SH_FILES := $(shell find $(SCRIPTS_DIR) -type f \( -name '*.sh' \) -print )
shellcheck: ## lint shell scripts with shellcheck
	shellcheck --version
	shellcheck -x -S style $(SH_FILES)

# Python virtualenv, for python-based tools
VENV_DIR     := venv_$(PROJECT_NAME)

# virtualenv activate script has undefined variables, disable then re-enable in bash
$(VENV_DIR): requirements.txt
	@echo "## Building Python Virtualenv ##"
	@python3 -m venv $@ ;\
  set +u; . ./$@/bin/activate; set -u ;\
  python -m pip install --upgrade pip ;\
  python -m pip install -r requirements.txt

pylint: $(VENV_DIR) ## lint python files
	set +u; . ./$</bin/activate; set -u ;\
	pylint --version ;\
	pylint --ignore-paths "$(VENV_DIR),$(REPO_DIR),$(SCRIPTS_DIR)/ba_lib/build" .

black: $(VENV_DIR) ## format check python files
	set +u; . ./$</bin/activate; set -u ;\
	black --version ;\
	black --check .

markdownlint: ## lint markdown files
	markdownlint --version ;\
	markdownlint README.md $(SCRIPTS_DIR)/*.md

license: $(VENV_DIR) ## Check licensing with the reuse tool
	set +u; . ./$</bin/activate; set -u ;\
  reuse --version ;\
  reuse --root . lint

## targets to help create buildall ##

# rebuild a repo list - not needed most of the time
$(REPO_LIST):
	$(SCRIPTS_DIR)/list_repos.sh $(GITHUB_ORG) > $@

# build list of full paths from repo list
REPOS := $(addprefix $(REPO_DIR)/, $(shell tr '\n' ' ' < $(REPO_LIST)))

# clone all repos
$(REPO_DIR)/%: | $(REPO_LIST)
	git clone https://github.com/$(GITHUB_ORG)/$*.git $(REPO_DIR)/$*

checkout-repos: $(REPOS) | $(REPO_LIST) ## checkout all repos
	@echo "## All repos have been checked out ##"

GIT_BRANCH ?= main
branch-checkout: checkout-repos ## refresh and checkout all repos at branch GIT_BRANCH (default: main)
	@for repo in $(REPOS); do \
		pushd $$repo ;\
		git checkout $(GIT_BRANCH) ; git pull --ff-only ;\
	  popd ;\
	done

# Check if there are the right make targets needed in the repo
MAKEFILE_CHECKS := $(addprefix $(SCRATCH_DIR)/make_check_, $(shell tr '\n' ' ' < $(REPO_LIST)))
check-makefiles: $(REPOS) | $(MAKEFILE_CHECKS)

$(SCRATCH_DIR)/make_check_%:
	$(SCRIPTS_DIR)/check_makefile.sh repos/$* |tee $@

## targets used to run buildall ##
buildall: $(SCRATCH_DIR) ## perform the entire build
	@$(SCRIPTS_DIR)/buildall.sh

timed-buildall: $(SCRATCH_DIR) ## perform the entire build with build times and verbose output
	$(SCRIPTS_DIR)/timed_buildall.sh

# list all artifacts used in each repo
ARTIFACT_LIST := $(addprefix $(SCRATCH_DIR)/artifacts_, $(addsuffix .yaml, $(shell tr '\n' ' ' < $(REPO_LIST))))
list-artifacts: $(REPOS) | $(SCRATCH_DIR) $(ARTIFACT_LIST) ## list charts/images provided by each repo
	@echo "## All artifact lists have been created ##"

$(SCRATCH_DIR)/artifacts_%.yaml:
	@echo "### Generating artifact list in 'repos/$*' ###" ;\
	$(SCRIPTS_DIR)/list_artifacts.sh repos/$* > $@

# determine work needed with orch-deploy manifest
chart-manifest: $(SCRATCH_DIR)/manifest_charts.yaml ## build chart manifest

$(SCRATCH_DIR)/manifest_charts.yaml: $(SCRATCH_DIR)
	@pushd .. > /dev/null ;\
	echo "## Generating releaseManifest of required helm charts ##" ;\
	mage gen:releaseManifest $(PROJECT_NAME)/$@ ;\
	popd > /dev/null

sort-charts: $(VENV_DIR)  ## determine which repos can build charts required by manifest, and write to htags_
	@set +u; . ./$</bin/activate; set -u ;\
	python scripts/sort_charts.py

image-manifest: $(SCRATCH_DIR)/manifest_images.yaml  ## build image manifest
$(SCRATCH_DIR)/manifest_images.yaml: $(SCRATCH_DIR)
	@pushd .. > /dev/null ;\
	echo "## Generating localReleaseImageManifest of docker images from charts ###" ;\
	mage gen:localReleaseImageManifest $(PROJECT_NAME)/$@ > $(PROJECT_NAME)/$@.log ;\
	popd > /dev/null

sort-images: $(VENV_DIR)  ## determine which repos can build images required by manifest, and write to itags_
	@set +u; . ./$</bin/activate; set -u ;\
	python scripts/sort_images.py

# build charts
HELM_BUILDS := $(addprefix $(SCRATCH_DIR)/hbuild_, $(shell tr '\n' ' ' < $(REPO_LIST)))
helm-build: $(HELM_BUILDS) $(CHARTS_DIR) ## build all helm charts
	@echo "## All helm charts have been created ##"

$(SCRATCH_DIR)/hbuild_%: $(CHARTS_DIR)
	@$(SCRIPTS_DIR)/helm_build.sh $* $@

clean-helm:  ## clean up helm chart scratchfiles to force rebuild
	rm -rf $(SCRATCH_DIR)/hbuild_*

# build images
IMAGE_BUILDS := $(addprefix $(SCRATCH_DIR)/ibuild_, $(shell tr '\n' ' ' < $(REPO_LIST)))
image-build: $(IMAGE_BUILDS) ## build all container images
	@echo "## All docker images have been created ##"

$(SCRATCH_DIR)/ibuild_%:
	@$(SCRIPTS_DIR)/image_build.sh $* $@

clean-image: ## clean up docker image scratchfiles to force rebuild
	rm -rf $(SCRATCH_DIR)/ibuild_*

# codecount
ALL_REPO_LIST := all_repo_list  # list of all repos that need to be counted
ALL_REPOS := $(addprefix $(REPO_DIR)/, $(shell tr '\n' ' ' < $(ALL_REPO_LIST)))
SCC_LIST := $(addprefix $(SCRATCH_DIR)/scc_, $(addsuffix .json, $(shell tr '\n' ' ' < $(ALL_REPO_LIST))))

codecount: $(VENV_DIR) $(SCC_LIST) $(ALL_REPOS) ## Create summarized CSV codecounts and totals from scc json
	set +u; . ./$</bin/activate; set -u ;\
	$(SCRIPTS_DIR)/codecount_summarize.py $(SCRATCH_DIR)

$(SCRATCH_DIR)/scc_%.json: $(ALL_REPOS) | $(SCRATCH_DIR)
	@pushd ${REPO_DIR}/$* ;\
  scc --min-gen -f json2 --output "../../$@";\
	popd

clean-codecount: ## clean up codecounts
	rm -rf $(SCRATCH_DIR)/scc_*
	rm -rf $(SCRATCH_DIR)/codecount_*

# cleanup
clean: clean-helm clean-image clean-codecount ## clean up scratch files
	rm -rf $(SCRATCH_DIR)/manifest_*
	rm -rf $(SCRATCH_DIR)/artifacts_*
	rm -rf $(SCRATCH_DIR)/htags_*
	rm -rf $(SCRATCH_DIR)/itags_*
	rm -rf $(CHARTS_DIR)

clean-all: clean # clean all downloaded files and venv
	rm -rf $(SCRATCH_DIR) $(VENV_DIR)
	rm -rf $(SCRIPTS_DIR)/ba_lib/build
	rm -rf $(SCRIPTS_DIR)/ba_lib/ba_lib.egg-info
#	rm -rf $(SCRATCH_DIR) $(REPOS_DIR) $(VENV_DIR)

help: ## Print help for each target
	@echo buildall make targets
	@echo "Target               Makefile:Line    Description"
	@echo "-------------------- ---------------- -----------------------------------------"
	@grep -H -n '^[[:alnum:]_-]*:.* ##' $(MAKEFILE_LIST) \
    | sort -t ":" -k 3 \
    | awk 'BEGIN  {FS=":"}; {sub(".* ## ", "", $$4)}; {printf "%-20s %-16s %s\n", $$3, $$1 ":" $$2, $$4};'
