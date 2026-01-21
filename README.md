# OpenEverest Provider SDK
A Go SDK for building database providers for the Everest platform. This SDK simplifies the creation of Kubernetes controllers that manage database lifecycle through the `DataStore` custom resource.

## 🎯 Purpose of this PoC

This repository contains a **proof-of-concept** implementation of the Provider SDK. The primary goals are:

1. **Evaluate SDK usability** - Ensure the SDK is easy to use for provider developers
2. **Validate design decisions** - Test the proposed architecture with a real implementation
3. **Gather team feedback** - Enable the team to review and help improve the SDK

## 📚 Documentation Guide

| Document | Audience | Description |
|----------|----------|-------------|
| [SDK Overview](docs/SDK_OVERVIEW.md) | All reviewers | Understand the problem and SDK architecture |
| [Provider CR Generation](docs/PROVIDER_CR_GENERATION.md) | Developers | How to generate the Provider CR manifest |
| [Examples Guide](examples/README.md) | Developers | Walk through the PSMDB reference implementation |
| [Metadata Helpers](docs/METADATA_HELPERS.md) | Developers | Working with provider metadata |

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- Access to a Kubernetes cluster (or use `kind`)
- `kubectl` configured

### Run the PSMDB Example

```bash
# Clone the repository
git clone https://github.com/openeverest/provider-sdk.git
cd provider-sdk

# Install SDK CRDs (in production: auto-installed with Everest)
kubectl apply -f config/crd/bases/

# Install PSMDB operator (in production: packaged in provider Helm chart)
kubectl apply --server-side -f https://raw.githubusercontent.com/percona/percona-server-mongodb-operator/v1.21.1/deploy/bundle.yaml

# Run the provider
cd examples/psmdb
go run cmd/provider/main.go
```

### Create a Test DataStore

```bash
kubectl apply -f examples/datastore-simple.yaml
```

## 📁 Repository Structure

```
provider-sdk/
├── README.md                    # This file
├── docs/
│   ├── SDK_OVERVIEW.md          # SDK architecture and concepts
│   ├── METADATA_HELPERS.md      # Working with metadata
│   └── PROVIDER_CR_GENERATION.md  # How to generate Provider manifests
├── pkg/
│   ├── apis/v2alpha1/           # CRD types (DataStore, Provider)
│   ├── controller/              # SDK core (Context handle, Status, etc.)
│   ├── reconciler/              # Reconciler implementations
│   └── server/                  # HTTP server for schemas
├── examples/
│   └── psmdb/                   # PSMDB provider example
│       ├── cmd/
│       │   └── provider/        # Provider entrypoint
│       ├── internal/            # PSMDB business logic
│       └── psmdbspec/           # PSMDB types and schemas
└── config/crd/bases/            # CRD manifests
```

## 🔍 How to Review This PoC

### For Decision Makers

1. **Read the [SDK Overview](docs/SDK_OVERVIEW.md)** to understand the problem and approach
2. **Review the decision documents** in `docs/decisions/`
3. **Look at the [examples](examples/)** to see both approaches in action

### For Developers

1. **Start with [examples/README.md](examples/README.md)** for a hands-on walkthrough
2. **Examine the SDK code** in `pkg/controller/` - especially:
   - [common.go](pkg/controller/common.go) - The `Context` handle abstraction
   - [interface.go](pkg/controller/interface.go) - Provider interface types
3. **Run the examples** and create test DataStore resources

### Questions to Consider

When reviewing, please consider:

1. **Usability**: Is the SDK easy to understand and use?
2. **API Design**: Is the interface design intuitive and idiomatic?
3. **Missing Features**: What's missing that would be needed for production?
4. **Naming**: Are the names (Context, Status, etc.) clear and appropriate?

## 📝 Providing Feedback

Please provide feedback through:
- GitHub Issues for specific problems or suggestions
- PR comments for code-level feedback
- Team discussions for design decisions

## 🔗 Related Links

- [Everest Platform](https://github.com/percona/everest) - Main Everest repository
- [PSMDB Operator](https://github.com/percona/percona-server-mongodb-operator) - Percona MongoDB operator

---

**Status**: Proof of Concept | **Version**: 0.1.0