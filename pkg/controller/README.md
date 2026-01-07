# SDK Controller Package

This package contains the core SDK abstractions for building Everest providers.

## Key Files

| File | Purpose |
|------|---------|
| `common.go` | The `Cluster` handle and resource operations |
| `interface.go` | Provider interface types (`ProviderInterface`, `BaseProvider`) |
| `metadata.go` | Provider metadata types and conversions |
| `generate.go` | CLI manifest generation utilities |

## Main Concepts

### The Cluster Handle (`common.go`)

The `Cluster` struct is the main interface for provider code:

```go
type Cluster struct {
    ctx      context.Context
    client   client.Client
    db       *v2alpha1.DataStore
    metadata *ProviderMetadata
}

// Key methods:
c.Name()           // Cluster name
c.Namespace()      // Cluster namespace
c.Spec()           // DataStore spec
c.Apply(obj)       // Create/update with owner reference
c.Get(obj, name)   // Read resource
c.Delete(obj)      // Delete resource
c.Metadata()       // Provider metadata
```

## Provider Interface

Implement the `ProviderInterface` to create a provider:

```go
type ProviderInterface interface {
    Name() string
    Types() func(*runtime.Scheme) error
    OwnedTypes() []client.Object
    Validate(cluster *Cluster) error
    Sync(cluster *Cluster) error
    Status(cluster *Cluster) (Status, error)
    Cleanup(cluster *Cluster) error
}
```

Use `BaseProvider` to inherit default implementations:

```go
type MyProvider struct {
    sdk.BaseProvider
}

func NewMyProvider() *MyProvider {
    return &MyProvider{
        BaseProvider: sdk.BaseProvider{
            ProviderName: "mydb",
            SchemeFuncs:  []func(*runtime.Scheme) error{mydbv1.AddToScheme},
            Owned:        []client.Object{&mydbv1.MyDB{}},
        },
    }
}

// Implement required methods
func (p *MyProvider) Validate(c *sdk.Cluster) error { ... }
func (p *MyProvider) Sync(c *sdk.Cluster) error { ... }
func (p *MyProvider) Status(c *sdk.Cluster) (sdk.Status, error) { ... }
func (p *MyProvider) Cleanup(c *sdk.Cluster) error { ... }
```

## See Also

- [SDK Overview](../../docs/SDK_OVERVIEW.md)
- [Examples](../../examples/README.md)

