# Provider Metadata Helpers

This document describes helper functions for working with provider metadata to look up component types, versions, and images.

## Overview

When implementing a provider, you often need to look up default images or versions for components. The SDK provides convenient helper functions through the `Context` handle.

**Key point:** When you register metadata with your provider (via `BaseProvider.Metadata` or `WithMetadata()`), it becomes available through `c.Metadata()` in your provider functions.

## Quick Reference

```go
func SyncPSMDB(c *sdk.Context) error {
    metadata := c.Metadata()
    
    // Get default image for a component type
    image := metadata.GetDefaultImage("mongod")
    // Returns: "percona/percona-server-mongodb:8.0.8-3"
    
    // Get full version info
    version := metadata.GetDefaultVersion("mongod")
    // Returns: &ComponentVersionMeta{Version: "8.0.8-3", Image: "...", Default: true}
    
    // Get component type for a logical component
    componentType := metadata.GetComponentType("engine")
    // Returns: "mongod"
    
    // Get default image for a logical component (combines above)
    engineImage := metadata.GetDefaultImageForComponent("engine")
    // Returns: "percona/percona-server-mongodb:8.0.8-3"
}
```

## Common Pattern: User Override with Default Fallback

The most common use case is allowing users to override images while providing sensible defaults:

```go
func SyncPSMDB(c *sdk.Context) error {
    engine := c.DB().Spec.Components["engine"]
    
    var image string
    if engine.Image != "" {
        // User explicitly specified an image
        image = engine.Image
    } else if metadata := c.Metadata(); metadata != nil {
        // Use the default from metadata
        image = metadata.GetDefaultImage(engine.Type)
    }
    
    psmdb := &psmdbv1.PerconaServerMongoDB{
        ObjectMeta: c.ObjectMeta(c.Name()),
        Spec: psmdbv1.PerconaServerMongoDBSpec{
            Image: image,
        },
    }
    
    return c.Apply(psmdb)
}
```

## Registering Metadata

When creating your provider, register metadata using the `BaseProvider` struct:

```go
func NewPSMDBProvider() *PSMDBProvider {
    return &PSMDBProvider{
        BaseProvider: sdk.BaseProvider{
            ProviderName: "psmdb",
            Metadata:     PSMDBMetadata(),  // Register metadata here
        },
    }
}
```

The reconciler automatically detects that your provider implements `MetadataProvider` and makes the metadata available through `c.Metadata()` in all your sync, validate, status, and cleanup functions.

## Metadata Structure

```go
func PSMDBMetadata() *sdk.ProviderMetadata {
    return &sdk.ProviderMetadata{
        // Component types define available versions and images
        ComponentTypes: map[string]sdk.ComponentTypeMeta{
            "mongod": {
                Versions: []sdk.ComponentVersionMeta{
                    {Version: "6.0.19-16", Image: "percona/percona-server-mongodb:6.0.19-16"},
                    {Version: "8.0.8-3", Image: "percona/percona-server-mongodb:8.0.8-3", Default: true},
                },
            },
        },
        
        // Components map logical names to component types
        Components: map[string]sdk.ComponentMeta{
            "engine":       {Type: "mongod"},
            "configServer": {Type: "mongod"},
        },
        
        // Topologies define valid deployment configurations
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

## Related Documentation

- [SDK Overview](SDK_OVERVIEW.md) - Architecture and concepts
- [Provider CR Generation](PROVIDER_CR_GENERATION.md) - How metadata is used for Provider CRs


