## Tool Versions
KUSTOMIZE_VERSION ?= v5.7.0
CONTROLLER_TOOLS_VERSION ?= v0.18.0
OPENAPI_GEN_VERSION ?= v0.0.0-20250910181357-589584f1c912

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
OPENAPI_GEN ?= $(LOCALBIN)/openapi-gen

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object paths="./..."

.PHONY: generate-openapi
generate-openapi: openapi-gen ## Generate OpenAPI definitions for custom spec types.
	$(OPENAPI_GEN) \
		--output-dir ./examples/psmdbspec/generated \
		--output-pkg github.com/openeverest/provider-sdk/examples/psmdbspec/generated \
		--output-file zz_generated.openapi.go \
		--report-filename /dev/null \
		--go-header-file ./hack/boilerplate.go.txt \
		github.com/openeverest/provider-sdk/examples/psmdbspec

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:allowDangerousTypes=true webhook paths="./..." output:crd:artifacts:config=config/crd/bases


.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: openapi-gen
openapi-gen: $(OPENAPI_GEN) ## Download openapi-gen locally if necessary.
$(OPENAPI_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/openapi-gen || \
	GOBIN=$(LOCALBIN) go install k8s.io/kube-openapi/cmd/openapi-gen@$(OPENAPI_GEN_VERSION)
