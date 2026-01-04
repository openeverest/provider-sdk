# PSMDB Provider Example

This directory contains a working implementation of a Percona Server MongoDB (PSMDB) provider using the SDK. It demonstrates both the interface-based and builder-based approaches.

## 📁 File Structure

```
examples/
├── psmdb_impl.go         # Shared business logic (ValidatePSMDB, SyncPSMDB, etc.)
├── psmdb_interface.go    # Interface-based provider implementation
├── psmdb_builder.go      # Builder-based provider implementation
├── psmdbspec/            # Custom spec types for PSMDB components
│   └── types.go
├── datastore-simple.yaml # Simple test DataStore manifest
├── datastore-example.yaml# Full DataStore manifest with all options
└── cmd/
    └── generate-manifest/
        └── main.go       # CLI tool to generate Provider CR manifest
```

## 🚀 Quick Start

### Prerequisites

1. A Kubernetes cluster (or `kind create cluster`)

2. Install the SDK CRDs:
   ```bash
   kubectl apply -f ../config/crd/bases/
   ```
   
   **Note:** In production, these CRDs are automatically installed when installing Everest.

3. Install the PSMDB operator:
   ```bash
   kubectl apply --server-side -f https://raw.githubusercontent.com/percona/percona-server-mongodb-operator/v1.21.1/deploy/bundle.yaml
   ```
   
   **Note:** This is a PoC requirement. In production, the underlying database operator (PSMDB in this case) should be packaged within the provider's Helm chart to ensure it installs automatically with the provider.

### Generate the Provider CR

Before running the provider, generate the Provider CR manifest:

```bash
# Generate the Provider CR from Go metadata
go run ./cmd/generate-manifest/main.go

# This creates charts/provider.yaml
# Install it in your cluster
kubectl apply -f charts/provider.yaml
```

**Important:** The Provider CR must be created before the provider starts. This tells Everest what component types and versions your provider supports.

See [Provider CR Generation Guide](../docs/PROVIDER_CR_GENERATION.md) for detailed instructions.

### Run the Provider

Choose one approach to run:

```bash
# Interface-based approach
go run psmdb_interface.go psmdb_impl.go

# OR Builder-based approach  
go run psmdb_builder.go psmdb_impl.go
```

### Create a Test DataStore

```bash
kubectl apply -f datastore-simple.yaml
```

Watch the provider logs and check the PSMDB resource:

```bash
kubectl get psmdb
kubectl get datastore
```

## 📖 Understanding the Code

### Shared Business Logic (`psmdb_impl.go`)

All provider logic is in `psmdb_impl.go`. Both approaches use these exact same functions:

```go
// Validate the DataStore spec
func ValidatePSMDB(c *sdk.Cluster) error { ... }

// Create/update PSMDB resources
func SyncPSMDB(c *sdk.Cluster) error { ... }

// Compute the current status
func StatusPSMDB(c *sdk.Cluster) (sdk.Status, error) { ... }

// Handle cleanup on deletion
func CleanupPSMDB(c *sdk.Cluster) error { ... }
```

This demonstrates that the SDK approach (interface vs builder) doesn't affect your business logic - only how you wire it up.

### Interface-Based Approach (`psmdb_interface.go`)

The interface approach uses a struct with methods:

```go
type PSMDBProvider struct {
    sdk.BaseProvider  // Provides default implementations
}

func NewPSMDBProviderInterface() *PSMDBProvider {
    return &PSMDBProvider{
        BaseProvider: sdk.BaseProvider{
            ProviderName: "psmdb",
            SchemeFuncs:  []func(*runtime.Scheme) error{psmdbv1.SchemeBuilder.AddToScheme},
            Owned:        []client.Object{&psmdbv1.PerconaServerMongoDB{}},
            Metadata:     PSMDBMetadata(),
        },
    }
}

// Implement the interface methods
func (p *PSMDBProvider) Validate(c *sdk.Cluster) error { return ValidatePSMDB(c) }
func (p *PSMDBProvider) Sync(c *sdk.Cluster) error { return SyncPSMDB(c) }
func (p *PSMDBProvider) Status(c *sdk.Cluster) (sdk.Status, error) { return StatusPSMDB(c) }
func (p *PSMDBProvider) Cleanup(c *sdk.Cluster) error { return CleanupPSMDB(c) }
```

**Key points:**
- Embed `sdk.BaseProvider` for defaults
- Implement `Validate`, `Sync`, `Status`, `Cleanup`
- Use `reconciler.NewFromInterface()` to create the reconciler

### Builder-Based Approach (`psmdb_builder.go`)

The builder approach uses a fluent API:

```go
func NewPSMDBProviderBuilder() *sdk.Provider {
    return sdk.Build("psmdb").
        WithTypes(psmdbv1.SchemeBuilder.AddToScheme).
        Owns(&psmdbv1.PerconaServerMongoDB{}).
        WithMetadata(PSMDBMetadata()).
        Validate(ValidatePSMDB).
        Sync("Sync PSMDB", SyncPSMDB).
        Status(StatusPSMDB).
        Cleanup("Cleanup PSMDB", CleanupPSMDB).
        Done()
}
```

**Key points:**
- Chain method calls to configure the provider
- Each sync/cleanup step has a name (for logging)
- Use `reconciler.NewFromBuilder()` to create the reconciler

## 🔧 Key SDK Concepts Demonstrated

### The Cluster Handle

The `*sdk.Cluster` is your main interface to everything:

```go
func SyncPSMDB(c *sdk.Cluster) error {
    // Get cluster info
    name := c.Name()
    namespace := c.Namespace()
    spec := c.Spec()
    
    // Access the underlying DataStore
    db := c.DB()
    
    // Get provider metadata
    metadata := c.Metadata()
    
    // Create resources (owner reference set automatically)
    psmdb := &psmdbv1.PerconaServerMongoDB{
        ObjectMeta: c.ObjectMeta(c.Name()),  // Helper for ObjectMeta
        Spec:       buildSpec(c),
    }
    return c.Apply(psmdb)  // Create or update
}
```

### Status Helpers

Instead of raw status structs:

```go
func StatusPSMDB(c *sdk.Cluster) (sdk.Status, error) {
    psmdb := &psmdbv1.PerconaServerMongoDB{}
    if err := c.Get(psmdb, c.Name()); err != nil {
        return sdk.Creating("Waiting for PSMDB"), nil
    }
    
    if psmdb.Status.State != "ready" {
        return sdk.Creating("PSMDB is starting"), nil
    }
    
    return sdk.RunningWithConnection(
        fmt.Sprintf("mongodb://%s:27017", c.Name()),
        c.Name() + "-credentials",
    ), nil
}
```

### Flow Control with WaitFor

```go
func CleanupPSMDB(c *sdk.Cluster) error {
    exists, _ := c.Exists(&psmdbv1.PerconaServerMongoDB{}, c.Name())
    if exists {
        return sdk.WaitFor("PSMDB deletion")  // Requeue and wait
    }
    return nil  // Done, continue cleanup
}
```

### Provider Metadata

Metadata describes what your provider supports. This is used to generate the Provider CR:

```go
func PSMDBMetadata() *sdk.ProviderMetadata {
    return &sdk.ProviderMetadata{
        ComponentTypes: map[string]sdk.ComponentTypeMeta{
            "mongod": {
                Versions: []sdk.ComponentVersionMeta{
                    {Version: "8.0.8-3", Image: "percona/percona-server-mongodb:8.0.8-3", Default: true},
                    {Version: "6.0.19-16", Image: "percona/percona-server-mongodb:6.0.19-16"},
                },
            },
        },
        Components: map[string]sdk.ComponentMeta{
            "engine": {Type: "mongod"},
        },
        Topologies: map[string]sdk.TopologyMeta{
            "replicaset": {
                Components: map[string]sdk.TopologyComponentMeta{
                    "engine": {Optional: false},
                },
            },
        },
    }
}
```

**Generating the Provider CR:**

```bash
# Run the generation tool
go run ./cmd/generate-manifest/main.go

# Output is written to charts/provider.yaml
# This file should be:
# 1. Committed to Git
# 2. Included in your Helm chart
# 3. Applied to the cluster before starting the provider
```

See [Provider CR Generation Guide](../docs/PROVIDER_CR_GENERATION.md) for more details.

## 🧪 Running Integration Tests

The `test/integration/` directory contains kuttl tests that verify the provider's behavior.

### Prerequisites for Tests

1. SDK CRDs installed (see Quick Start above)
2. PSMDB operator installed (see Quick Start above)
3. Provider running in the background:
   ```bash
   # In one terminal, start the provider:
   go run psmdb_interface.go psmdb_impl.go
   
   # Or use the builder approach:
   go run psmdb_builder.go psmdb_impl.go
   ```

### Running the Tests

```bash
# From the examples directory:
make test-integration

# Or run directly:
cd examples
. ./test/vars.sh && kubectl kuttl test --config ./test/integration/kuttl.yaml
```

**Note:** The tests assume the provider is already running and will create/update/delete DataStore resources to verify correct behavior.

## 📝 Creating Your Own Provider

To create a new provider:

1. **Copy the structure** from this example
2. **Replace PSMDB types** with your operator's types
3. **Define your metadata** with component types and versions
4. **Generate the Provider CR** using the CLI tool
5. **Implement the four functions**: Validate, Sync, Status, Cleanup
6. **Choose your approach** (interface or builder)

See the [SDK Overview](../docs/SDK_OVERVIEW.md) and [Provider CR Generation Guide](../docs/PROVIDER_CR_GENERATION.md) for detailed guidance.

## 🔗 Related Documentation

- [SDK Overview](../docs/SDK_OVERVIEW.md) - Architecture and concepts
- [Interface vs Builder Decision](../docs/decisions/INTERFACE_VS_BUILDER.md) - API style comparison
- [Provider CR Generation](../docs/PROVIDER_CR_GENERATION.md) - How to generate the Provider CR
- [Metadata Helpers](../docs/METADATA_HELPERS.md) - Working with metadata
