# SDK Overview

This document explains the problem the SDK solves, its architecture, and key concepts.

## The Problem

Everest aims to provide a unified, cloud-native database management experience across multiple database engines (PostgreSQL, MongoDB, MySQL, ClickHouse, etc.). Each database engine has its own Kubernetes operator with unique:

- Custom Resource Definitions (CRDs)
- Reconciliation patterns
- Status reporting mechanisms
- Configuration schemas

**The core challenge:** How do we enable database engine maintainers to integrate their operators with Everest without requiring deep Kubernetes controller expertise?

### Without an SDK

Without a proper SDK, provider authors face several challenges:

| Pain Point | Description |
|------------|-------------|
| **Kubernetes complexity** | Authors must understand `context.Context`, `client.Client`, `reconcile.Request`, owner references, finalizers, and more |
| **Boilerplate code** | Each provider reimplements the same patterns: create-or-update, status mapping, cleanup logic |
| **Inconsistent implementations** | Without guidance, providers handle errors, retries, and status differently |
| **Steep learning curve** | New contributors need weeks to understand controller-runtime before writing provider logic |
| **Testing difficulty** | Tight coupling to Kubernetes makes unit testing painful |

### The Gap

```
┌─────────────────────────────────────────────────────────────────┐
│                    What Provider Authors Know                   │
│  • How their database operator works                            │
│  • What CRs their operator needs                                │
│  • How to map DataStore spec to operator-specific config        │
│  • What status fields indicate healthy/unhealthy state          │
└─────────────────────────────────────────────────────────────────┘
                              ▼
                         ╔═══════╗
                         ║  GAP  ║  ← This is what the SDK bridges
                         ╚═══════╝
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│               What Kubernetes Controllers Require               │
│  • context.Context propagation                                  │
│  • client.Client for API operations                             │
│  • Reconcile loops with proper requeue logic                    │
│  • Owner references for garbage collection                      │
│  • Finalizers for cleanup                                       │
│  • Watch configuration with predicates                          │
│  • Status subresource updates                                   │
│  • Error handling and retry backoff                             │
└─────────────────────────────────────────────────────────────────┘
```

## The Solution: Provider SDK

The SDK bridges this gap by providing:

1. **A simplified `Cluster` handle** - One object that provides everything a provider needs
2. **Automatic Kubernetes plumbing** - Finalizers, owner references, requeue logic handled automatically
3. **Semantic status helpers** - `Creating()`, `Running()`, `Failed()` instead of raw status structs
4. **Error-based flow control** - Use Go's idiomatic error handling, not custom result types

### Before (Raw controller-runtime) - ~100+ lines

```go
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
    var db v2alpha1.DataStore
    if err := r.Client.Get(ctx, req.NamespacedName, &db); err != nil {
        return reconcile.Result{}, client.IgnoreNotFound(err)
    }

    if db.DeletionTimestamp != nil {
        if controllerutil.ContainsFinalizer(&db, finalizerName) {
            // Complex cleanup logic with multiple API calls...
            // Handle finalizer removal...
            // Check for dependent resources...
        }
        return reconcile.Result{}, nil
    }

    if !controllerutil.ContainsFinalizer(&db, finalizerName) {
        controllerutil.AddFinalizer(&db, finalizerName)
        if err := r.Client.Update(ctx, &db); err != nil {
            return reconcile.Result{}, err
        }
    }

    // Create the operator CR with proper owner references...
    // Update if exists, create if not...
    // Check status and requeue if not ready...
    // Update DataStore status...
    // ... many more lines
}
```

### After (With SDK) - ~50 lines

```go
type PSMDBProvider struct {
    sdk.BaseProvider
}

func (p *PSMDBProvider) Validate(c *sdk.Cluster) error {
    // Just validation logic, nothing else
    return nil
}

func (p *PSMDBProvider) Sync(c *sdk.Cluster) error {
    psmdb := &psmdbv1.PerconaServerMongoDB{
        ObjectMeta: c.ObjectMeta(c.Name()),
        Spec:       buildSpec(c),
    }
    return c.Apply(psmdb)  // Owner ref set automatically
}

func (p *PSMDBProvider) Status(c *sdk.Cluster) (sdk.Status, error) {
    psmdb := &psmdbv1.PerconaServerMongoDB{}
    if err := c.Get(psmdb, c.Name()); err != nil {
        return sdk.Creating("Initializing"), nil
    }
    if psmdb.Status.State != "ready" {
        return sdk.Creating("Starting"), nil
    }
    return sdk.Running(), nil
}

func (p *PSMDBProvider) Cleanup(c *sdk.Cluster) error {
    exists, _ := c.Exists(&psmdbv1.PerconaServerMongoDB{}, c.Name())
    if exists {
        return sdk.WaitFor("PSMDB deletion")
    }
    return nil
}

// Create reconciler
provider := &PSMDBProvider{
    BaseProvider: sdk.BaseProvider{
        ProviderName: "psmdb",
        SchemeFuncs:  []func(*runtime.Scheme) error{psmdbv1.AddToScheme},
        Owned:        []client.Object{&psmdbv1.PerconaServerMongoDB{}},
    },
}
reconciler, _ := reconciler.New(provider)
```

## SDK Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                             Provider Code                                   │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Validate() → Sync() → Status() → Cleanup()                         │    │
│  │  (Your business logic - no Kubernetes complexity)                   │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                               SDK Layer                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │   Cluster    │  │    Status    │  │   WaitFor    │  │  ObjectMeta  │     │
│  │   Handle     │  │   Helpers    │  │   Helpers    │  │   Helpers    │     │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘     │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            Reconciler Layer                                 │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │  • Finalizer management                                              │   │
│  │  • Owner reference handling                                          │   │
│  │  • Requeue logic                                                     │   │
│  │  • Status updates                                                    │   │
│  │  • Watch configuration                                               │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         controller-runtime                                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Key Concepts

### The Cluster Handle

The `Cluster` struct is the main interface between your provider code and the SDK. It wraps:
- The Kubernetes client
- The current DataStore being reconciled
- Context for API operations
- Provider metadata (if configured)

```go
func MySync(c *sdk.Cluster) error {
    // Identity
    c.Name()        // Cluster name
    c.Namespace()   // Cluster namespace
    
    // Spec access
    c.Spec()        // Full spec
    c.DB()          // Underlying DataStore
    c.Metadata()    // Provider metadata
    
    // Resource operations (owner ref set automatically)
    c.Apply(obj)    // Create or update
    c.Get(obj, name)// Read
    c.Delete(obj)   // Delete
    c.Exists(obj, name) // Check existence
    c.List(list)    // List resources
    
    // Helpers
    c.ObjectMeta(name) // Create ObjectMeta with owner ref
}
```

### Status Helpers

Instead of manually constructing status structs, use semantic helpers:

```go
// Creating state
return sdk.Creating("Waiting for primary node")

// Running state
return sdk.Running()
return sdk.RunningWithConnection("mongodb://...", "secret-name")

// Failed state
return sdk.Failed(fmt.Errorf("replication failed"))
```

### Flow Control

Use errors for flow control - it's idiomatic Go:

```go
func MySync(c *sdk.Cluster) error {
    // Success - continue to next step
    return nil
    
    // Wait and requeue
    return sdk.WaitFor("resource to be ready")
    
    // Error - will be logged and reconciliation retried
    return fmt.Errorf("failed to create resource: %w", err)
}
```

## What the SDK Handles Automatically

| Concern | How SDK Handles It |
|---------|-------------------|
| **Finalizers** | Added automatically, removed after cleanup completes |
| **Owner references** | Set automatically by `Apply()` |
| **Requeue logic** | `WaitFor()` errors trigger requeue with backoff |
| **Error handling** | Errors are logged and trigger requeue |
| **Status updates** | Called after sync, updates status subresource |
| **Deletion handling** | Cleanup steps run when deletion timestamp is set |
| **Watch setup** | Configured from `Owns()` |
| **Scheme registration** | Types registered from `WithTypes()` |

## Provider Lifecycle

When a DataStore is created, modified, or deleted, the reconciler follows this flow:

```
┌─────────────────────────────────────────────────────────────┐
│                    DataStore Event                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
              ┌───────────────────────────────┐
              │   Is DeletionTimestamp set?   │
              └───────────────────────────────┘
                    │                    │
                   Yes                   No
                    │                    │
                    ▼                    ▼
        ┌───────────────────┐   ┌───────────────────┐
        │    Cleanup()      │   │   Add Finalizer   │
        │    Remove         │   │   Validate()      │
        │    Finalizer      │   │   Sync()          │
        └───────────────────┘   │   Status()        │
                                └───────────────────┘
```

## Next Steps

- **[Provider CR Generation Guide](PROVIDER_CR_GENERATION.md)** - How to generate Provider manifests
- **[Examples Guide](../examples/README.md)** - See a working implementation

