# Decision: Interface-Based vs Builder-Based Provider API

**Status:** Pending Team Review  
**Decision Needed By:** Before SDK v1.0  
**Impact:** How provider developers will write their code

## Summary

The SDK offers two ways to create a provider. We need to decide which approach to recommend (or support both).

| Approach | Best For | Code Style |
|----------|----------|------------|
| **Interface-based** | Complex providers, testability | Traditional Go structs |
| **Builder-based** | Simple providers, rapid prototyping | Fluent/functional style |

## Quick Comparison

### Interface-Based Approach

```go
type PSMDBProvider struct {
    sdk.BaseProvider
}

func (p *PSMDBProvider) Validate(c *sdk.Cluster) error {
    return validatePSMDB(c)
}

func (p *PSMDBProvider) Sync(c *sdk.Cluster) error {
    if err := p.ensureMainCluster(c); err != nil {
        return err
    }
    return p.configureUsers(c)
}

func (p *PSMDBProvider) Status(c *sdk.Cluster) (sdk.Status, error) {
    return computeStatus(c)
}

func (p *PSMDBProvider) Cleanup(c *sdk.Cluster) error {
    return cleanup(c)
}
```

### Builder-Based Approach

```go
sdk.Build("psmdb").
    WithTypes(psmdbv1.SchemeBuilder.AddToScheme).
    Owns(&psmdbv1.PerconaServerMongoDB{}).
    Validate(validatePSMDB).
    Sync("Ensure main cluster", ensureMainCluster).
    Sync("Configure users", configureUsers).
    Status(computeStatus).
    Cleanup("Cleanup resources", cleanup).
    Done()
```

## Detailed Comparison

### Code Organization

| Aspect | Interface | Builder |
|--------|-----------|---------|
| Private methods | ✅ Easy with struct methods | ❌ Must use external functions |
| Shared state | ✅ Struct fields | ⚠️ Closures or global state |
| Code splitting | ✅ Multiple methods | ⚠️ Long chain or external functions |

**Interface example with private methods:**
```go
func (p *PSMDBProvider) Sync(c *sdk.Cluster) error {
    if err := p.ensureCluster(c); err != nil {
        return err
    }
    return p.configureReplicas(c, p.defaultReplicas)
}

func (p *PSMDBProvider) ensureCluster(c *sdk.Cluster) error {
    // Private helper method
}
```

**Builder requires external functions or closures:**
```go
Sync("Ensure cluster", ensureCluster).      // External function
Sync("Configure", func(c *sdk.Cluster) error {
    // Inline closure - can access outer scope
})
```

### Testability

| Aspect | Interface | Builder |
|--------|-----------|---------|
| Mock provider | ✅ Implement interface | ⚠️ Less common pattern |
| Test individual steps | ✅ Call methods directly | ⚠️ Need to extract functions |
| Dependency injection | ✅ Constructor injection | ⚠️ Closures |

**Interface testing:**
```go
func TestValidate(t *testing.T) {
    p := NewPSMDBProvider()
    err := p.Validate(mockCluster)
    assert.NoError(t, err)
}
```

**Builder testing (requires extracted functions):**
```go
func TestValidate(t *testing.T) {
    err := validatePSMDB(mockCluster)  // Must extract function
    assert.NoError(t, err)
}
```

### Flexibility

| Aspect | Interface | Builder |
|--------|-----------|---------|
| Conditional logic | ✅ Full control in Sync() | ⚠️ All steps always run |
| Step ordering | ✅ You control order | ✅ Defined order |
| Dynamic steps | ✅ Any logic in methods | ❌ Steps fixed at build time |

**Interface with conditional logic:**
```go
func (p *PSMDBProvider) Sync(c *sdk.Cluster) error {
    if err := p.ensurePrimary(c); err != nil {
        return err
    }
    
    // Conditional based on spec
    if c.Spec().Sharding.Enabled {
        if err := p.ensureSharding(c); err != nil {
            return err
        }
    }
    
    return p.ensureUsers(c)
}
```

**Builder - steps always run:**
```go
Sync("Ensure primary", ensurePrimary).
Sync("Ensure sharding", ensureSharding).  // Always runs
Sync("Ensure users", ensureUsers)
// Conditional must be inside the function
```

### Readability

| Aspect | Interface | Builder |
|--------|-----------|---------|
| At-a-glance overview | ⚠️ Look at method signatures | ✅ Entire config in one place |
| Named steps | ❌ No built-in naming | ✅ Each step has a name |
| Self-documenting | ⚠️ Depends on method names | ✅ Step names describe flow |

### Boilerplate

| Aspect | Interface | Builder |
|--------|-----------|---------|
| Lines of code | ~85 for PSMDB example | ~56 for PSMDB example |
| Required methods | 4 (Validate, Sync, Status, Cleanup) | 0 (all optional) |
| Type declarations | struct + methods | Just function calls |

## Decision Matrix

| Factor | Interface | Builder | Notes |
|--------|-----------|---------|-------|
| Simple providers | ⭐⭐ | ⭐⭐⭐ | Builder is more concise |
| Complex providers | ⭐⭐⭐ | ⭐⭐ | Interface handles complexity better |
| Testability | ⭐⭐⭐ | ⭐⭐ | Interface is easier to test |
| Readability | ⭐⭐ | ⭐⭐⭐ | Builder is self-documenting |
| Learning curve | ⭐⭐ | ⭐⭐⭐ | Builder is simpler to start |
| Shared state | ⭐⭐⭐ | ⭐ | Interface wins clearly |
| Conditional logic | ⭐⭐⭐ | ⭐ | Interface wins clearly |
| Named steps (logging) | ⭐ | ⭐⭐⭐ | Builder has this built-in |

## Options

### Option A: Recommend Interface-Based

**Pros:**
- Better for production providers
- Easier to test
- More flexible
- Traditional Go patterns

**Cons:**
- More boilerplate
- Steeper initial learning curve

### Option B: Recommend Builder-Based

**Pros:**
- Faster onboarding
- Less boilerplate
- Self-documenting
- Better for simple providers

**Cons:**
- Harder to test individual steps
- Doesn't scale well for complex providers
- Less flexible

### Option C: Support Both (Recommend Based on Complexity)

**Pros:**
- Developers can choose what fits
- Start with builder, migrate to interface

**Cons:**
- More documentation to maintain
- May confuse new users
- Two code paths to support

### Option D: Hybrid Approach

Extract functions for testability while using builder for configuration:

```go
sdk.Build("psmdb").
    Validate(validatePSMDB).        // External, testable function
    Sync("Sync cluster", syncPSMDB). // External, testable function
    Status(computeStatus).          // External, testable function
    Cleanup("Cleanup", cleanup).     // External, testable function
    Done()
```

**Pros:**
- Named steps from builder
- Testable functions
- Clean configuration

**Cons:**
- Doesn't help with shared state
- Still need to extract functions

## Current Implementation Status

Both approaches are implemented and working in this PoC:
- **Interface:** See `examples/psmdb_interface.go`
- **Builder:** See `examples/psmdb_builder.go`
- **Shared logic:** Both use `examples/psmdb_impl.go`

## Questions for Reviewers

1. **Which approach feels more natural to you?**
2. **Do we expect most providers to be simple or complex?**
3. **How important is unit testing individual steps?**
4. **Should we support both, or pick one to reduce maintenance burden?**
5. **Is the hybrid approach a good compromise?**

## Recommendation

*(To be filled after team discussion)*

---

**Related:**
- [SDK Overview](../SDK_OVERVIEW.md)
- [Examples](../../examples/README.md)
- [Interface implementation](../../pkg/controller/interface.go)
- [Builder implementation](../../pkg/controller/builder.go)

