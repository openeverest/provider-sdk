# provider-sdk

CLI tool for scaffolding and extending [OpenEverest](https://github.com/openeverest) providers
— databases, caches, message queues, object storage, model-serving runtimes, and anything else
fronted by a Kubernetes operator.

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
  --name my-database \
  --module github.com/my-org/provider-my-database
```

Or run interactively (you will be prompted for each value):

```bash
provider-sdk init
```

## Documentation

| | |
|---|---|
| **[Tutorial](TUTORIAL.md)** | **Start here.** Build a working provider end to end, in about an hour. |
| [Provider development guide](PROVIDER_DEVELOPMENT.md) | Reference for everything: components, versions, topologies, the UI schema, the provider interface, RBAC, backups, releasing. |
| [provider-example](https://github.com/openeverest/provider-example) | The smallest complete provider, commented to be read. |
| [provider-percona-server-mongodb](https://github.com/openeverest/provider-percona-server-mongodb) | A production provider, for when you need to see the real thing. |

## Commands

| Command | Purpose |
|---|---|
| `provider-sdk init` | Scaffold a new provider project |
| `provider-sdk add component` | Add a component and its type |
| `provider-sdk add topology` | Add a deployment topology |
| `provider-sdk add backup` | Add backup and restore implementation stubs |
| `provider-sdk add backupclass` | Add a BackupClass definition |
| `provider-sdk generate` | Build the Provider CR spec from `definition/` |

Each command prints its own flags with `--help`. `generate` is normally invoked through
`go generate ./...` or `make generate` rather than by hand.

## Working on the SDK itself

```bash
make build   # build to ./bin/provider-sdk
make test    # run unit tests
```

The scaffolding engine lives in `internal/scaffold/`, with the project template it emits under
`internal/scaffold/_template/` (embedded into the binary; the leading underscore keeps the Go
toolchain from compiling it). The Provider CR generator lives in `internal/generate/`.

[`internal/scaffold/_template/README.md`](internal/scaffold/_template/README.md) is the
standard README for every OpenEverest provider, and the canonical copy: iterate on it here,
then apply it to the provider repositories. It is written for three audiences in order —
platform users, operators, contributors — and leads with the fact that a provider is not
standalone. The *Capabilities* tables deliberately reuse the same wording everywhere so
providers stay comparable; rows that make no sense for a technology are deleted rather than
marked unsupported.
