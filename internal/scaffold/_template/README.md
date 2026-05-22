# [[ .ProviderName ]]

An [OpenEverest](https://github.com/openeverest) provider.

> **New to provider development?** See `github.com/openeverest/provider-sdk/blob/main/PROVIDER_DEVELOPMENT.md` for a complete guide.

## Prerequisites

- Go 1.26+
- A Kubernetes cluster (k3d, kind, or remote)
- [OpenEverest CRDs](https://github.com/openeverest/openeverest) installed
- Your operator installed and running

## Quick Start

```bash
# Generate all manifests (RBAC, provider spec, Helm chart)
make generate

# Run the provider locally (for development)
make run

# Or deploy with Helm
make helm-install
```

## Development

### Project Structure

```
cmd/provider/              # Entry point
internal/
  provider/
    provider.go            # ProviderInterface implementation (Validate/Sync/Status/Cleanup)
    rbac.go                # Kubebuilder RBAC markers
  common/
    spec.go                # Component name constants
definition/
  provider.yaml            # Provider name + component→type mapping
  versions.yaml            # Component type version/image catalog
  types.go                 # Shared Go types
  components/
    types.go               # Component custom spec types
  topologies/
    <topology>/
      topology.yaml        # Topology config + UI schema
      types.go             # Topology-specific config types
config/
  rbac/
    role.yaml              # Generated ClusterRole (do not edit manually)
charts/[[ .ProviderName ]]/     # Helm chart for deployment
  generated/
    rbac-rules.yaml        # Generated RBAC rules (do not edit manually)
    provider-spec.yaml     # Generated Provider CR spec (do not edit manually)
  templates/               # Helm templates
examples/
  instance-example.yaml    # Example Instance CR
  instance-simple.yaml     # Minimal Instance CR
dev/
  k3d_config.yaml          # Local k3d cluster config
hack/                      # Helper scripts
gen.go                     # go:generate entry point
Makefile                   # Build, generate, and deploy targets
Dockerfile
```

### Make Targets

| Target                  | Description                                                |
|-------------------------|-------------------------------------------------------------|
| `make generate`         | Run all code generation (RBAC + Helm sync + provider spec) |
| `make run`              | Run the provider locally                                   |
| `make build`            | Build the provider binary                                  |
| `make docker-build`     | Build the container image                                  |
| `make helm-install`     | Deploy with Helm                                           |
| `make helm-template`    | Render Helm templates locally (dry-run)                    |
| `make test`             | Run unit tests                                             |
| `make test-integration` | Run kuttl integration tests                                |
| `make verify`           | Check generated files are up-to-date (CI)                  |
| `make lint`             | Run golangci-lint                                          |

> For development patterns (RBAC, watches, code generation), see [PROVIDER_DEVELOPMENT.md](https://github.com/openeverest/provider-sdk/blob/main/PROVIDER_DEVELOPMENT.md).

## Deployment

### Helm

```bash
# Install
helm install [[ .ProviderName ]] charts/[[ .ProviderName ]]/ --create-namespace

# Upgrade
helm upgrade [[ .ProviderName ]] charts/[[ .ProviderName ]]/

# Uninstall
helm uninstall [[ .ProviderName ]]
```

### Local Development

```bash
# Create a local k3d cluster
make k3d-cluster-up

# Run the provider locally against the cluster
make run

# Run integration tests
make test-integration

# Tear down the cluster
make k3d-cluster-down
```

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
