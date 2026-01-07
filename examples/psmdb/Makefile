## Tool Versions
OPENAPI_GEN_VERSION ?= v0.0.0-20250910181357-589584f1c912

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/../../bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

OPENAPI_GEN ?= $(LOCALBIN)/openapi-gen

.PHONY: run
run: ## Run the provider locally
	go run cmd/provider/main.go

.PHONY: generate-openapi
generate-openapi: openapi-gen ## Generate OpenAPI definitions for custom spec types
	$(OPENAPI_GEN) \
		--output-dir ./types/generated \
		--output-pkg github.com/openeverest/provider-sdk/examples/psmdb/types/generated \
		--output-file zz_generated.openapi.go \
		--report-filename /dev/null \
		--go-header-file ../../hack/boilerplate.go.txt \
		github.com/openeverest/provider-sdk/examples/psmdb/types

.PHONY: test-integration
test-integration: ## Run integration tests against K8S cluster
	. ./test/vars.sh && kubectl kuttl test --config ./test/integration/kuttl.yaml

.PHONY: openapi-gen
openapi-gen: $(OPENAPI_GEN) ## Download openapi-gen locally if necessary
$(OPENAPI_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/openapi-gen || \
	GOBIN=$(LOCALBIN) go install k8s.io/kube-openapi/cmd/openapi-gen@$(OPENAPI_GEN_VERSION)

