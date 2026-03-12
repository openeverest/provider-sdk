# [[ .ProviderName ]]

An [OpenEverest](https://github.com/openeverest) provider.

> **New to provider development?** See `definition/PROVIDER_DEVELOPMENT.md` for a complete guide.

## Prerequisites

- Go 1.25+
- A Kubernetes cluster (k3d, kind, or remote)
- [OpenEverest CRDs](https://github.com/openeverest/openeverest) installed
- Your upstream operator installed and running

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
    standalone/
      topology.yaml        # Topology config + UI schema
      types.go             # Topology-specific config types
config/
  rbac/
    role.yaml              # Generated ClusterRole (do not edit manually)
charts/[[ .ProviderName ]]/     # Helm chart for deployment
  generated/
    rbac-rules.yaml        # Generated RBAC rules (do not edit manually)
    provider-spec.yaml     # Generated Provider CR spec (do not edit manually)
Makefile                   # Build, generate, and deploy targets
```

### Key Make Targets

| Target             | Description                                              |
|--------------------|----------------------------------------------------------|
| `make run`         | Run the provider locally                                 |
| `make generate`    | Run all code generation (RBAC + Helm sync + provider.yaml) |
| `make manifests`   | Generate RBAC from kubebuilder markers                   |
| `make generate` | Run all code generation (RBAC + Helm sync + provider spec) |
| `make build`       | Build the provider binary                                |
| `make docker-build`| Build the container image                                |
| `make helm-install`| Deploy with Helm                                         |
| `make helm-template`| Render Helm templates locally (dry-run)                 |
| `make test`        | Run unit tests                                           |
| `make test-integration` | Run kuttl integration tests                         |
| `make verify`      | Check generated files are up-to-date (CI)                |
| `make lint`        | Run golangci-lint                                        |

### RBAC Workflow

RBAC permissions are declared using [kubebuilder markers](https://book.kubebuilder.io/reference/markers/rbac) in Go source files (primarily `internal/provider/rbac.go`).

1. Add `+kubebuilder:rbac` markers for any new resources your provider needs
2. Run `make generate`
3. This regenerates `config/rbac/role.yaml` and syncs rules into the Helm chart
4. Commit the changes

Base RBAC for the provider-runtime (Instances, Providers, leases, events) is pre-configured in `rbac.go`.

### Adding Watches

When you add a new resource to `WatchConfigs` in your provider, add the corresponding RBAC markers:

```go
// In provider.go
WatchConfigs: []controller.WatchConfig{
    controller.WatchOwned(&operatorv1.MyDatabase{}),
    controller.WatchOwned(&operatorv1.MyBackup{}),  // new watch
},

// In rbac.go (or provider.go)
// +kubebuilder:rbac:groups=mydb.example.com,resources=mybackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mydb.example.com,resources=mybackups/status,verbs=get
```

Then run `make generate` to update the RBAC manifests.

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
