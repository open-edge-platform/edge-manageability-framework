# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
#
# Makefile — repo-wide lint orchestration for the EMF helmfile flow.
#
# Quick start:
#   make help           # list targets
#   make lint           # run linters (best-effort: skips tools not found)
#   make lint-fix       # auto-fix what can be auto-fixed (shfmt)
#   make lint-tools     # report which linters are installed
#
# Each lint-* target is independent. Missing tools are warned about, not fatal,
# unless STRICT=1 is passed (CI mode):
#   make lint STRICT=1

SHELL              := /usr/bin/env bash
.SHELLFLAGS        := -e -o pipefail -c
.DEFAULT_GOAL      := help

# ─── Configuration ──────────────────────────────────────────────────────────
ROOT_DIR           := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
STRICT             ?= 0

# Exclude vendored / generated / license boilerplate.
EXCLUDE_PATHS      := -not -path '*/.git/*' \
                      -not -path '*/LICENSES/*' \
                      -not -path '*/node_modules/*' \
                      -not -path '*/vendor/*' \
                      -not -path '*/.cache/*' \
                      -not -path '*/logs/*'

# Helm chart templates contain gotmpl directives ({{- ... }}); yamllint cannot
# parse them. They are validated by lint-helmfile / helm lint instead.
YAML_EXCLUDE_PATHS := $(EXCLUDE_PATHS) \
                      -not -path '*/templates/*'

YAML_FILES         := $(shell find $(ROOT_DIR) -type f \( -name '*.yaml' -o -name '*.yml' \) $(YAML_EXCLUDE_PATHS) 2>/dev/null)
GOTMPL_FILES       := $(shell find $(ROOT_DIR) -type f -name '*.gotmpl' $(EXCLUDE_PATHS) 2>/dev/null)
SHELL_FILES        := $(shell find $(ROOT_DIR) -type f -name '*.sh' $(EXCLUDE_PATHS) 2>/dev/null)

# Tool overrides (set on command line: make lint YAMLLINT=/path/to/yamllint)
YAMLLINT           ?= yamllint
SHELLCHECK         ?= shellcheck
SHFMT              ?= shfmt
HELMFILE           ?= helmfile
TRIVY              ?= trivy

# Colors
C_RED              := \033[31m
C_GRN              := \033[32m
C_YEL              := \033[33m
C_BLU              := \033[34m
C_RST              := \033[0m

# Internal: run a checker if the binary exists, else warn (or fail if STRICT=1).
# Lint findings are non-fatal by default; pass STRICT=1 to make them fatal.
# Usage: $(call _run,<binary>,<command line>,<friendly name>)
define _run
	@if command -v $(1) >/dev/null 2>&1; then \
		echo -e "$(C_BLU)▶ $(3)$(C_RST)"; \
		if [[ "$(STRICT)" == "1" ]]; then \
			$(2); \
		else \
			$(2) || echo -e "$(C_YEL)⚠ $(3) reported issues (non-fatal; set STRICT=1 to fail)$(C_RST)"; \
		fi; \
	else \
		echo -e "$(C_YEL)⚠ skip $(3): '$(1)' not installed$(C_RST)"; \
		if [[ "$(STRICT)" == "1" ]]; then exit 1; fi; \
	fi
endef

# ─── Help ───────────────────────────────────────────────────────────────────
.PHONY: help
help: ## Show this help
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

# ─── Tooling ────────────────────────────────────────────────────────────────
.PHONY: lint-tools
lint-tools: ## Report which lint tools are available
	@for t in $(YAMLLINT) $(SHELLCHECK) $(SHFMT) $(HELMFILE) $(TRIVY); do \
	  if command -v $$t >/dev/null 2>&1; then \
	    printf "  $(C_GRN)✓$(C_RST) %-14s %s\n" "$$t" "$$(command -v $$t)"; \
	  else \
	    printf "  $(C_RED)✗$(C_RST) %-14s (not installed)\n" "$$t"; \
	  fi; \
	done

.PHONY: install-tools
install-tools: ## Hint how to install lint tools (Debian/Ubuntu)
	@echo "Suggested install commands (Debian/Ubuntu):"
	@echo "  sudo apt-get install -y yamllint shellcheck shfmt"
	@echo "  curl -sSfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sudo sh -s -- -b /usr/local/bin"
	@echo "  curl -sSfL https://github.com/helmfile/helmfile/releases/latest/download/helmfile_linux_amd64.tar.gz | tar -xz helmfile && sudo mv helmfile /usr/local/bin/"

# ─── Aggregates ─────────────────────────────────────────────────────────────
.PHONY: lint
lint: lint-yaml lint-gotmpl lint-shell ## Fast linters (no network). Run lint-helmfile / security separately.
	@echo -e "$(C_GRN)✔ lint complete$(C_RST)"

.PHONY: lint-fix
lint-fix: lint-shell-fix ## Auto-fix what we can
	@echo -e "$(C_GRN)✔ lint-fix complete$(C_RST)"

.PHONY: security
security: lint-trivy ## Run security scanners
	@echo -e "$(C_GRN)✔ security complete$(C_RST)"

# ─── YAML ───────────────────────────────────────────────────────────────────
# Relaxed default config: disable noisy rules. Override by dropping a
# .yamllint at the repo root, or pass YAMLLINT_CONFIG=path on the command line.
YAMLLINT_CONFIG    ?= {extends: default, rules: {line-length: disable, document-start: disable, truthy: {check-keys: false}, comments: {min-spaces-from-content: 1}, comments-indentation: disable, indentation: {indent-sequences: consistent}, braces: disable, brackets: disable}}

.PHONY: lint-yaml
lint-yaml: ## Lint *.yaml / *.yml with yamllint (warnings non-fatal unless STRICT=1)
	@if command -v $(YAMLLINT) >/dev/null 2>&1; then \
	  echo -e "$(C_BLU)▶ yamllint ($(words $(YAML_FILES)) files)$(C_RST)"; \
	  if [[ "$(STRICT)" == "1" ]]; then \
	    $(YAMLLINT) -s -d '$(YAMLLINT_CONFIG)' -f auto $(YAML_FILES); \
	  else \
	    $(YAMLLINT) -d '$(YAMLLINT_CONFIG)' -f auto $(YAML_FILES) || \
	      echo -e "$(C_YEL)⚠ yamllint reported issues (non-fatal; set STRICT=1 to fail)$(C_RST)"; \
	  fi; \
	else \
	  echo -e "$(C_YEL)⚠ skip yamllint: '$(YAMLLINT)' not installed$(C_RST)"; \
	  if [[ "$(STRICT)" == "1" ]]; then exit 1; fi; \
	fi

# ─── Helmfile / Helm gotmpl ─────────────────────────────────────────────────
.PHONY: lint-gotmpl
lint-gotmpl: ## Basic syntax check of *.gotmpl (delimiter balance)
	@echo -e "$(C_BLU)▶ gotmpl balance check ($(words $(GOTMPL_FILES)) files)$(C_RST)"
	@bad=0; for f in $(GOTMPL_FILES); do \
	  open=$$( { grep -o '{{' "$$f" 2>/dev/null || true; } | wc -l ); \
	  close=$$( { grep -o '}}' "$$f" 2>/dev/null || true; } | wc -l ); \
	  if [[ $$open -ne $$close ]]; then \
	    echo "  $(C_RED)✗$(C_RST) $$f  open=$$open close=$$close"; bad=$$((bad+1)); \
	  fi; \
	done; \
	if [[ $$bad -ne 0 ]]; then echo "$(C_RED)$$bad files unbalanced$(C_RST)"; exit 1; fi; \
	echo "  ok"

HELMFILE_ENVS      ?= onprem-eim onprem-vpro

.PHONY: lint-helmfile
lint-helmfile: ## helmfile lint (post-orch + pre-orch, all $(HELMFILE_ENVS))
	@for dir in post-orch pre-orch; do \
	  if [[ -f "$(ROOT_DIR)/$$dir/helmfile.yaml.gotmpl" ]]; then \
	    if command -v $(HELMFILE) >/dev/null 2>&1; then \
	      for env in $(HELMFILE_ENVS); do \
	        echo -e "$(C_BLU)▶ helmfile -e $$env lint  ($$dir)$(C_RST)"; \
	        (cd "$(ROOT_DIR)/$$dir" && $(HELMFILE) -e "$$env" lint --skip-deps) || exit 1; \
	      done; \
	    else \
	      echo -e "$(C_YEL)⚠ skip helmfile lint $$dir: helmfile not installed$(C_RST)"; \
	      if [[ "$(STRICT)" == "1" ]]; then exit 1; fi; \
	    fi; \
	  fi; \
	done

# ─── Shell ──────────────────────────────────────────────────────────────────
.PHONY: lint-shell
lint-shell: lint-shellcheck lint-shfmt ## Run shellcheck + shfmt (check only)

.PHONY: lint-shellcheck
lint-shellcheck: ## shellcheck on *.sh
	$(call _run,$(SHELLCHECK),$(SHELLCHECK) -x -S warning $(SHELL_FILES),shellcheck ($(words $(SHELL_FILES)) files))

.PHONY: lint-shfmt
lint-shfmt: ## shfmt formatting check
	$(call _run,$(SHFMT),$(SHFMT) -d -i 2 -ci -bn $(SHELL_FILES),shfmt -d ($(words $(SHELL_FILES)) files))

.PHONY: lint-shell-fix
lint-shell-fix: ## shfmt auto-format
	$(call _run,$(SHFMT),$(SHFMT) -w -i 2 -ci -bn $(SHELL_FILES),shfmt -w)

# ─── Security ───────────────────────────────────────────────────────────────
.PHONY: lint-trivy
lint-trivy: ## trivy fs scan (uses trivy.yaml if present)
	$(call _run,$(TRIVY),$(TRIVY) fs --config $(ROOT_DIR)/trivy.yaml $(ROOT_DIR),trivy fs)

# ─── Maintenance ────────────────────────────────────────────────────────────
.PHONY: clean
clean: ## Remove transient lint artefacts
	@rm -rf .cache 2>/dev/null || true
	@echo "cleaned"
