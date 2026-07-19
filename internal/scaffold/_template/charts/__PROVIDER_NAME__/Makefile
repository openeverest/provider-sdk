HELM ?= helm

# values to override
IMAGE_PREFIX ?= ghcr.io/openeverest
PROVIDER_IMAGE_NAME ?= [[ .ProviderName ]]
VERSION ?= 0.1.0
CHART_VERSION ?= $(VERSION)

.PHONY: prepare-chart
prepare-chart:
	yq eval -i '.version = "$(CHART_VERSION)"' Chart.yaml
	yq eval -i '.appVersion = "$(VERSION)"' Chart.yaml
	yq eval -i '.image.repository = "$(IMAGE_PREFIX)/$(PROVIDER_IMAGE_NAME)"' values.yaml
	yq eval -i '.image.tag = "$(VERSION)"' values.yaml
	$(MAKE) deps

.PHONY: release
release: prepare-chart

.PHONY: release-dev
release-dev: PROVIDER_IMAGE_NAME := [[ .ProviderName ]]-dev
release-dev: prepare-chart

.PHONY: deps
deps:
	$(HELM) version
	# TODO: If your provider bundles an operator subchart, add its Helm repo and
	# resolve dependencies here so `helm dependency build`/`update` can find it.
	# For example:
	#   $(HELM) repo add my-operator https://example.com/helm-charts/
	#   $(HELM) dependency update .
