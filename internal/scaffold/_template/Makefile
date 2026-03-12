## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# CONTAINER_TOOL defines the container tool to be used for building images.
CONTAINER_TOOL ?= docker

# Image URL to use for building/pushing image targets
IMG ?= ghcr.io/openeverest/[[ .ProviderName ]]-dev:latest

# controller-gen version
CONTROLLER_TOOLS_VERSION ?= v0.18.0
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)

# yq version for YAML processing
YQ_VERSION ?= v4.44.6
YQ ?= $(LOCALBIN)/yq-$(YQ_VERSION)

# golangci-lint version
GOLANGCI_LINT_VERSION ?= v1.63.4
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)

# Helm chart directory
CHART_DIR ?= charts/[[ .ProviderName ]]

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: run
run: generate ## Run the provider locally.
	go run cmd/provider/main.go

.PHONY: lint
lint: golangci-lint ## Run golangci-lint.
	$(GOLANGCI_LINT) run

.PHONY: test
test: ## Run unit tests.
	go test ./... -coverprofile cover.out

##@ Code Generation

.PHONY: manifests
manifests: controller-gen ## Generate RBAC manifests using controller-gen from kubebuilder markers.
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./..." output:rbac:dir=config/rbac

.PHONY: helm-sync-rbac
helm-sync-rbac: yq ## Sync generated RBAC rules into the Helm chart.
	@echo "Syncing RBAC rules from config/rbac/role.yaml to Helm chart..."
	@$(YQ) '.rules' config/rbac/role.yaml > $(CHART_DIR)/generated/rbac-rules.yaml
	@echo "Done."

.PHONY: generate
generate: manifests helm-sync-rbac ## Run all code generation (RBAC + Helm sync + provider spec from definition/).
	go generate ./...
	@echo "All generation complete."

.PHONY: verify
verify: ## Verify that generated files are up-to-date (for CI).
	@$(MAKE) generate
	@if git diff --quiet -- config/ $(CHART_DIR)/generated/; then \
		echo "Generated files are up-to-date."; \
	else \
		echo "ERROR: Generated files are out of date. Run 'make generate' and commit the changes."; \
		git diff -- config/ $(CHART_DIR)/generated/; \
		exit 1; \
	fi

##@ Build

.PHONY: build
build: generate ## Build provider binary.
	go build -o bin/provider cmd/provider/main.go

.PHONY: docker-build
docker-build: ## Build docker image.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image.
	$(CONTAINER_TOOL) push ${IMG}

##@ Helm

.PHONY: helm-install
helm-install: ## Install the provider using Helm.
	helm install [[ .ProviderName ]] $(CHART_DIR) --create-namespace

.PHONY: helm-upgrade
helm-upgrade: ## Upgrade the provider using Helm.
	helm upgrade [[ .ProviderName ]] $(CHART_DIR)

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall the provider using Helm.
	helm uninstall [[ .ProviderName ]]

.PHONY: helm-template
helm-template: ## Render Helm chart templates locally (dry-run).
	helm template [[ .ProviderName ]] $(CHART_DIR)

##@ Testing

.PHONY: test-integration
test-integration: ## Run integration tests (kuttl) against a running cluster.
	. ./test/vars.sh && kubectl kuttl test --config ./test/integration/kuttl.yaml

##@ Local Development Cluster

.PHONY: k3d-cluster-up
k3d-cluster-up: ## Create a local k3d cluster for development.
	$(info Creating k3d cluster for testing)
	k3d cluster create --config ./dev/k3d_config.yaml

.PHONY: k3d-cluster-down
k3d-cluster-down: ## Delete the local k3d cluster.
	$(info Destroying k3d test cluster)
	k3d cluster delete --config ./dev/k3d_config.yaml

.PHONY: k3d-cluster-reset
k3d-cluster-reset: k3d-cluster-down k3d-cluster-up ## Reset the local k3d cluster.

##@ Tool Dependencies

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Install controller-gen.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: yq
yq: $(YQ) ## Install yq.
$(YQ): $(LOCALBIN)
	@echo "Installing yq $(YQ_VERSION)..."
	@GOBIN=$(LOCALBIN) go install github.com/mikefarah/yq/v4@$(YQ_VERSION) && mv $(LOCALBIN)/yq $(YQ)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Install golangci-lint.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# go-install-tool will 'go install' any package with custom target and target name. Usage:
# $(call go-install-tool,<target>,<package>,<version>)
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3); \
echo "Installing $${package}"; \
GOBIN=$(LOCALBIN) go install $${package}; \
mv -f $$(echo "$(1)" | sed "s/-$(3)$$//") $(1); \
}
endef
