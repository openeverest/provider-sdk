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
  --module github.com/my-org/provider-my-database
```

Or run interactively (you will be prompted for each value):

```bash
provider-sdk init
```

## Generated Structure

```
provider-my-database/
├── PROVIDER_DEVELOPMENT.md           # Complete development guide
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

Read [PROVIDER_DEVELOPMENT.md](PROVIDER_DEVELOPMENT.md).

```bash
cd provider-my-database

# 1. Add your operator Go dependency
go get your-operator-module@latest
go mod tidy

# 2. Add components and topologies
provider-sdk add component --name mydb --type mydb
provider-sdk add topology --name standalone

# 3. Configure versions in definition/versions.yaml
# 4. Implement provider logic in internal/provider/provider.go
# 5. Optionally add backup support (only if your operator supports backups)
provider-sdk add backup                      # Adds backup.go
provider-sdk add backup --include-mirror     # Adds backup.go + backup_mirror.go
provider-sdk add backupclass --name my-backup-class

# 6. If added: implement backup logic in internal/provider/backup.go + backup_mirror.go
# 7. Add RBAC markers in internal/provider/rbac.go

# 8. Generate all manifests
make generate

# 9. Run locally against a cluster
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
│   ├── add_topology.go             # add topology subcommand
│   ├── add_backup.go               # add backup subcommand
│   └── add_backupclass.go          # add backupclass subcommand
├── internal/
│   ├── scaffold/                   # Scaffolding engine + embedded template
│   │   ├── scaffold.go
│   │   ├── scaffold_test.go
│   │   ├── add_component.go
│   │   ├── add_topology.go
│   │   ├── add_backup.go
│   │   ├── add_backupclass.go
│   │   └── _template/             # Template files (embedded in binary)
│   ├── generate/                   # Provider CR spec generator
│   │   ├── generate.go
│   │   ├── assemble.go
│   │   ├── backupclasses.go
│   │   └── schema.go
│   └── tui/                        # Terminal UI helpers
│       ├── multiselect.go
│       └── prompt.go
├── pkg/util/
├── go.mod
└── go.sum
```

## Development

```bash
make build   # Build binary to ./bin/provider-sdk
make test    # Run unit tests
```
