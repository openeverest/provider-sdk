package controller

// Builder-Based Provider SDK
//
// Use the fluent builder API to configure a provider:
//
//   sdk.Build("name").
//       WithTypes(fn).
//       Owns(obj).
//       Validate(fn).
//       Sync("step", fn).
//       Status(fn).
//       Cleanup("step", fn).
//       Done()
//
// See examples/psmdb_builder.go for a complete example.
// See docs/decisions/INTERFACE_VS_BUILDER.md for comparison with interface approach.

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncFunc is the signature for sync/cleanup step functions.
type SyncFunc func(c *Cluster) error

// ValidateFunc is the signature for validation functions.
type ValidateFunc func(c *Cluster) error

// StatusFunc is the signature for status computation functions.
type StatusFunc func(c *Cluster) (Status, error)

// Build creates a new provider builder with the given name.
func Build(name string) *ProviderBuilder {
	return &ProviderBuilder{
		name:             name,
		componentSchemas: make(map[string]interface{}),
		topologies:       make(map[string]TopologyDefinition),
	}
}

// ProviderBuilder creates a provider using a fluent API.
type ProviderBuilder struct {
	name         string
	types        []func(*runtime.Scheme) error
	ownedTypes   []client.Object
	validateFn   ValidateFunc
	syncSteps    []Step
	statusFn     StatusFunc
	cleanupSteps []Step

	metadata         *ProviderMetadata
	componentSchemas map[string]interface{}
	topologies       map[string]TopologyDefinition
	globalSchema     interface{}
}

// Step represents a named unit of work.
type Step struct {
	Name string
	Fn   SyncFunc
}

// WithTypes registers scheme builder functions for provider-specific CRDs.
// Can be called multiple times to register multiple schemes.
func (b *ProviderBuilder) WithTypes(fns ...func(*runtime.Scheme) error) *ProviderBuilder {
	b.types = append(b.types, fns...)
	return b
}

// Owns registers types that this provider creates.
// Changes to these resources will trigger reconciliation.
func (b *ProviderBuilder) Owns(objs ...client.Object) *ProviderBuilder {
	b.ownedTypes = append(b.ownedTypes, objs...)
	return b
}

// Validate sets the validation function.
func (b *ProviderBuilder) Validate(fn ValidateFunc) *ProviderBuilder {
	b.validateFn = fn
	return b
}

// Sync adds a named sync step. Steps run in the order they are added.
func (b *ProviderBuilder) Sync(name string, fn SyncFunc) *ProviderBuilder {
	b.syncSteps = append(b.syncSteps, Step{Name: name, Fn: fn})
	return b
}

// Status sets the status computation function.
func (b *ProviderBuilder) Status(fn StatusFunc) *ProviderBuilder {
	b.statusFn = fn
	return b
}

// Cleanup adds a named cleanup step. Steps run in the order they are added.
func (b *ProviderBuilder) Cleanup(name string, fn SyncFunc) *ProviderBuilder {
	b.cleanupSteps = append(b.cleanupSteps, Step{Name: name, Fn: fn})
	return b
}

// =============================================================================
// METADATA METHODS (for Provider CR generation)
// =============================================================================

// WithMetadata sets the provider metadata for Provider CR generation.
// This enables CLI manifest generation.
//
// Example:
//
//	Build("psmdb").
//	    WithMetadata(&sdk.ProviderMetadata{
//	        ComponentTypes: map[string]sdk.ComponentTypeMeta{
//	            "mongod": {Versions: []sdk.ComponentVersionMeta{{Version: "8.0.8-3", Default: true}}},
//	        },
//	    })
func (b *ProviderBuilder) WithMetadata(m *ProviderMetadata) *ProviderBuilder {
	b.metadata = m
	return b
}

// =============================================================================
// SCHEMA METHODS (for OpenAPI schema generation)
// =============================================================================

// WithComponentSchema registers a schema type for a component.
// The type should be a struct that represents the component's custom spec.
// Use struct tags for schema customization:
//   - description: Schema description
//   - enum: Comma-separated allowed values
//   - default: Default value
//   - minimum/maximum: Numeric bounds
//
// Example:
//
//	Build("psmdb").
//	    WithComponentSchema("engine", &MongodCustomSpec{}).
//	    WithComponentSchema("proxy", &MongosCustomSpec{})
func (b *ProviderBuilder) WithComponentSchema(name string, typ interface{}) *ProviderBuilder {
	b.componentSchemas[name] = typ
	return b
}

// WithTopology registers a topology with its configuration schema and supported components.
// This combines schema definition and component list in a single call.
//
// Example:
//
//	Build("psmdb").
//	    WithTopology("replicaset", TopologyDefinition{
//	        Schema:     &ReplicaSetTopologyConfig{},
//	        Components: []string{"engine", "backupAgent", "monitoring"},
//	    }).
//	    WithTopology("sharded", TopologyDefinition{
//	        Schema:     &ShardedTopologyConfig{},
//	        Components: []string{"engine", "proxy", "configServer", "backupAgent", "monitoring"},
//	    })
func (b *ProviderBuilder) WithTopology(name string, def TopologyDefinition) *ProviderBuilder {
	b.topologies[name] = def
	return b
}

// WithGlobalSchema registers a schema type for global configuration.
//
// Example:
//
//	Build("psmdb").
//	    WithGlobalSchema(&GlobalConfig{})
func (b *ProviderBuilder) WithGlobalSchema(typ interface{}) *ProviderBuilder {
	b.globalSchema = typ
	return b
}

// Done finalizes the provider configuration.
func (b *ProviderBuilder) Done() *Provider {
	return &Provider{builder: b}
}

// =============================================================================
// BUILT PROVIDER
// =============================================================================

// Provider is a configured provider ready for use.
type Provider struct {
	builder *ProviderBuilder
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return p.builder.name
}

// Types returns a function that applies all registered scheme builders.
func (p *Provider) Types() func(*runtime.Scheme) error {
	if len(p.builder.types) == 0 {
		return nil
	}
	return func(s *runtime.Scheme) error {
		for _, fn := range p.builder.types {
			if err := fn(s); err != nil {
				return err
			}
		}
		return nil
	}
}

// OwnedTypes returns the types owned by this provider.
func (p *Provider) OwnedTypes() []client.Object {
	return p.builder.ownedTypes
}

// Validate runs the validation function.
func (p *Provider) Validate(c *Cluster) error {
	if p.builder.validateFn == nil {
		return nil
	}
	return p.builder.validateFn(c)
}

// Sync runs all sync steps in order.
func (p *Provider) Sync(c *Cluster) error {
	for _, step := range p.builder.syncSteps {
		if err := step.Fn(c); err != nil {
			return fmt.Errorf("%s: %w", step.Name, err)
		}
	}
	return nil
}

// Status computes the cluster status.
func (p *Provider) Status(c *Cluster) (Status, error) {
	if p.builder.statusFn == nil {
		return Running(), nil
	}
	return p.builder.statusFn(c)
}

// Cleanup runs all cleanup steps in order.
func (p *Provider) Cleanup(c *Cluster) error {
	for _, step := range p.builder.cleanupSteps {
		if err := step.Fn(c); err != nil {
			return fmt.Errorf("%s: %w", step.Name, err)
		}
	}
	return nil
}

// GetSyncSteps returns the sync steps (for logging/debugging).
func (p *Provider) GetSyncSteps() []Step {
	return p.builder.syncSteps
}

// GetCleanupSteps returns the cleanup steps (for logging/debugging).
func (p *Provider) GetCleanupSteps() []Step {
	return p.builder.cleanupSteps
}

// =============================================================================
// METADATA & SCHEMA ACCESS (for reconciler integration)
// =============================================================================

// GetMetadata returns the provider metadata for Provider CR generation.
// Returns nil if no metadata was configured.
func (p *Provider) GetMetadata() *ProviderMetadata {
	return p.builder.metadata
}

// ComponentSchemas returns the custom spec schemas for each component.
// Implements the SchemaProvider interface pattern for builder-based providers.
func (p *Provider) ComponentSchemas() map[string]interface{} {
	return p.builder.componentSchemas
}

// Topologies returns the topology definitions (schema + components).
// Implements the SchemaProvider interface pattern for builder-based providers.
func (p *Provider) Topologies() map[string]TopologyDefinition {
	return p.builder.topologies
}

// GlobalSchema returns the global configuration schema.
// Implements the SchemaProvider interface pattern for builder-based providers.
func (p *Provider) GlobalSchema() interface{} {
	return p.builder.globalSchema
}

// HasSchemas returns true if any schemas are configured.
func (p *Provider) HasSchemas() bool {
	return len(p.builder.componentSchemas) > 0 ||
		len(p.builder.topologies) > 0 ||
		p.builder.globalSchema != nil
}

