.PHONY: run-interface
run-interface:
	go run psmdb_interface.go psmdb_impl.go

.PHONY: run-builder
run-builder:
	go run psmdb_builder.go psmdb_impl.go

.PHONY: test-integration
test-integration: ## Run integration tests against K8S cluster
	. ./test/vars.sh && kubectl kuttl test --config ./test/integration/kuttl.yaml

