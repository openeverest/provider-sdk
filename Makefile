## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# ---- Build ----

.PHONY: build
build: ## Build the provider-sdk CLI binary.
	go build -o $(LOCALBIN)/provider-sdk .

.PHONY: install
install: ## Install the provider-sdk CLI binary to $GOPATH/bin.
	go install ./cmd/provider-sdk

.PHONY: test
test: ## Run unit tests.
	go test ./...
