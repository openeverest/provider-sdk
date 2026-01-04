# SDK Controller Package

This package contains the core SDK abstractions for building Everest providers.

## Key Files

| File | Purpose |
|------|---------|
| `common.go` | The `Cluster` handle and resource operations |
| `interface.go` | Interface-based provider types (`ProviderIface`, `BaseProvider`) |
| `builder.go` | Builder-based provider types (`ProviderBuilder`, `Provider`) |
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

### Status Helpers (`common.go`)

```go
Creating("message")           // Creating phase
Running()                     // Running phase
RunningWithConnection(url, secret)
Failed(err)                   // Failed phase
```

### Flow Control (`common.go`)

```go
WaitFor("reason")             // Requeue reconciliation
WaitForDuration(duration, "reason")
```

## Provider Approaches

### Interface-Based (`interface.go`)

```go
type ProviderIface interface {
    Name() string
    Types() func(*runtime.Scheme) error
    OwnedTypes() []client.Object
    Validate(cluster *Cluster) error
    Sync(cluster *Cluster) error
    Status(cluster *Cluster) (Status, error)
    Cleanup(cluster *Cluster) error
}
```

### Builder-Based (`builder.go`)

```go
sdk.Build("name").
    WithTypes(fn).
    Owns(obj).
    Validate(fn).
    Sync("step", fn).
    Status(fn).
    Cleanup("step", fn).
    Done()
```

## See Also

- [SDK Overview](../../docs/SDK_OVERVIEW.md)
- [Interface vs Builder Decision](../../docs/decisions/INTERFACE_VS_BUILDER.md)
- [Examples](../../examples/README.md)

