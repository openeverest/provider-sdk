# provider-sdk

CLI tool for scaffolding and extending [OpenEverest](https://github.com/openeverest) database providers.

## Installation

```bash
go install github.com/openeverest/provider-sdk@latest
```

Or without installing globally:

```bash
go run github.com/openeverest/provider-sdk@latest init
```

## Quick Start

```bash
provider-sdk init \
  --name provider-my-database \
  --module github.com/my-org/provider-my-database \
  --component-type mydb
```

Or run interactively (you will be prompted for each value):

```bash
provider-sdk init
```

## Commands

### `init` — Scaffold a New Provider

| Flag | Description | Default |
|------|-------------|---------|
| `--name` | Provider name (e.g., `provider-my-database`) | — (required) |
| `--module` | Go module path (e.g., `github.com/my-org/provider-my-database`) | — (required) |
| `--component-type` | Primary component type name (e.g., `mydb`) | — (required) |
| `--topology` | Initial topology name | `standalone` |
| `--api-group` | Upstream operator API group (optional, used as RBAC hint) | — |
| `--resource` | Upstream operator resource, plural (optional, used as RBAC hint) | — |
| `--output-dir`, `-o` | Output directory | `./<name>` |
| `--non-interactive` | Fail instead of prompting for missing values | `false` |

## Generated Structure

```
provider-my-database/
├── cmd/provider/main.go              # Entry point
├── internal/
│   ├── provider/
│   │   ├── provider.go               # ProviderInterface implementation stubs
│   │   └── rbac.go                   # Kubebuilder RBAC markers
│   └── common/
│       └── spec.go                   # Component name/type constants
├── definition/
│   ├── provider.yaml                 # Provider name + component→type mapping
│   ├── versions.yaml                 # Component type version/image catalog
│   ├── types.go                      # Shared Go types
│   ├── PROVIDER_DEVELOPMENT.md       # Complete development guide
│   ├── components/
│   │   └── types.go                  # Component custom spec types
│   └── topologies/
│       └── <topology>/
│           ├── topology.yaml         # Topology config + UI schema
│           └── types.go              # Topology config types
├── gen.go                            # go:generate directive
├── config/rbac/role.yaml             # Generated ClusterRole
├── charts/provider-my-database/      # Helm chart
│   ├── Chart.yaml
│   ├── values.yaml
│   ├── generated/
│   │   ├── provider-spec.yaml        # Generated from definition/
│   │   └── rbac-rules.yaml           # Generated from RBAC markers
│   └── templates/
├── test/integration/                 # kuttl integration tests
├── dev/k3d_config.yaml               # Local k3d cluster config
├── examples/                         # Example Instance CRs
├── .github/workflows/                # CI pipelines
├── Makefile
├── Dockerfile
└── go.mod
```

## After Scaffolding

```bash
cd provider-my-database

# 1. Read the development guide
cat definition/PROVIDER_DEVELOPMENT.md

# 2. Add your upstream operator Go dependency
go get your-operator-module@latest
go mod tidy

# 3. Add components and topologies
provider-sdk add component --name mydb --type mydb
provider-sdk add topology --name standalone

# 4. Configure versions in definition/versions.yaml
# 5. Implement provider logic in internal/provider/provider.go
# 6. Add RBAC markers in internal/provider/rbac.go

# 7. Generate all manifests
make generate

# 8. Run locally against a cluster
make run
```

## Repository Structure

```
provider-sdk/
├── main.go                         # Entry point
├── cmd/
│   ├── root.go                     # Root command
│   ├── init.go                     # init subcommand
│   ├── generate.go                 # generate subcommand
│   ├── add.go                      # add parent command
│   ├── add_component.go            # add component subcommand
│   └── add_topology.go             # add topology subcommand
├── internal/
│   ├── scaffold/                   # Scaffolding engine + embedded template
│   │   ├── scaffold.go
│   │   ├── scaffold_test.go
│   │   ├── add_component.go
│   │   ├── add_topology.go
│   │   └── _template/             # Template files (embedded in binary)
│   └── generate/                   # Provider CR spec generator
│       ├── generate.go
│       ├── assemble.go
│       └── schema.go
├── go.mod
└── go.sum
```

## Development

```bash
make build   # Build binary to ./bin/provider-sdk
make test    # Run unit tests
```
