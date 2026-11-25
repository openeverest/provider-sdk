## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

HELM ?= helm
HELM_UNITTEST_VERSION ?= v1.0.3
HELM_PLUGINS_DIR ?= $(LOCALBIN)/helm-plugins

# ---- Build ----

.PHONY: build
build: ## Build the provider-sdk CLI binary.
	go build -o $(LOCALBIN)/provider-sdk .

.PHONY: install
install: ## Install the provider-sdk CLI binary to $GOPATH/bin.
	go install ./cmd/provider-sdk

.PHONY: gen
gen: ## Regenerate the committed copies of the chart templates generate writes, and their Helm test suites.
	go run ./hack/gen-compat-tests

.PHONY: test
test: ## Run unit tests.
	go test ./...

.PHONY: check-release
check-release: ## Check go.mod and the scaffold require a released OpenEverest, as a release must (run by the release workflow).
	go run ./hack/check-release-core

.PHONY: test-helm
test-helm: helm-unittest ## Run the helm-unittest suites of the chart templates generate writes.
	HELM_PLUGINS=$(HELM_PLUGINS_DIR) $(HELM) unittest test/helm/openeverest-compat

.PHONY: helm-unittest
helm-unittest: ## Install the helm-unittest plugin into bin/helm-plugins.
	@HELM_PLUGINS=$(HELM_PLUGINS_DIR) $(HELM) plugin list | grep -q '^unittest' || \
		HELM_PLUGINS=$(HELM_PLUGINS_DIR) $(HELM) plugin install https://github.com/helm-unittest/helm-unittest --version $(HELM_UNITTEST_VERSION)
