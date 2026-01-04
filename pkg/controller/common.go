package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openeverest/provider-sdk/pkg/apis/v2alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// =============================================================================
// CORE ABSTRACTION: The Cluster handle
// =============================================================================

// Cluster is the main handle for working with a DataStore.
// It provides a simplified interface that hides Kubernetes complexity.
type Cluster struct {
	ctx      context.Context
	client   client.Client
	db       *v2alpha1.DataStore
	metadata *ProviderMetadata
}

// NewCluster creates a new Cluster handle (used internally by the reconciler).
func NewCluster(ctx context.Context, c client.Client, db *v2alpha1.DataStore) *Cluster {
	return &Cluster{ctx: ctx, client: c, db: db}
}

// NewClusterWithMetadata creates a new Cluster handle with provider metadata.
// This is preferred over NewCluster as it makes metadata available to provider implementations.
func NewClusterWithMetadata(ctx context.Context, c client.Client, db *v2alpha1.DataStore, metadata *ProviderMetadata) *Cluster {
	return &Cluster{ctx: ctx, client: c, db: db, metadata: metadata}
}

// Spec returns the cluster specification.
func (c *Cluster) Spec() *v2alpha1.DataStoreSpec {
	return &c.db.Spec
}

// Name returns the cluster name.
func (c *Cluster) Name() string {
	return c.db.Name
}

// Namespace returns the cluster namespace.
func (c *Cluster) Namespace() string {
	return c.db.Namespace
}

// Labels returns the cluster labels.
func (c *Cluster) Labels() map[string]string {
	return c.db.Labels
}

// Annotations returns the cluster annotations.
func (c *Cluster) Annotations() map[string]string {
	return c.db.Annotations
}

// ComponentsOfType returns all components of a given type.
func (c *Cluster) ComponentsOfType(componentType string) []v2alpha1.ComponentSpec {
	return c.db.GetComponentsOfType(componentType)
}

// DB returns the underlying DataStore for direct access.
func (c *Cluster) DB() *v2alpha1.DataStore {
	return c.db
}

// Metadata returns the provider metadata, if available.
// Returns nil if metadata was not provided when creating the Cluster handle.
// The metadata is automatically populated by the reconciler if the provider
// implements the MetadataProvider interface.
func (c *Cluster) Metadata() *ProviderMetadata {
	return c.metadata
}

// Raw returns the underlying DataStore (escape hatch for advanced use).
// Deprecated: Use DB() instead.
func (c *Cluster) Raw() *v2alpha1.DataStore {
	return c.db
}

// =============================================================================
// RESOURCE OPERATIONS
// =============================================================================

// Apply creates or updates a resource, setting ownership automatically.
// This is the primary way to manage resources - just describe what you want.
func (c *Cluster) Apply(obj client.Object) error {
	// Set the owner reference automatically
	if err := controllerutil.SetControllerReference(c.db, obj, c.client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner: %w", err)
	}

	// Use create-or-update semantics
	existing := obj.DeepCopyObject().(client.Object)
	err := c.client.Get(c.ctx, client.ObjectKeyFromObject(obj), existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		// Doesn't exist, create it
		return c.client.Create(c.ctx, obj)
	}
	// Exists, update it
	obj.SetResourceVersion(existing.GetResourceVersion())
	return c.client.Update(c.ctx, obj)
}

// Get retrieves a resource by name (in the cluster's namespace).
func (c *Cluster) Get(obj client.Object, name string) error {
	return c.client.Get(c.ctx, client.ObjectKey{
		Namespace: c.db.Namespace,
		Name:      name,
	}, obj)
}

// Exists checks if a resource exists.
func (c *Cluster) Exists(obj client.Object, name string) (bool, error) {
	err := c.Get(obj, name)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

// Delete removes a resource.
func (c *Cluster) Delete(obj client.Object) error {
	err := c.client.Delete(c.ctx, obj)
	return client.IgnoreNotFound(err)
}

// List retrieves resources matching optional filters.
func (c *Cluster) List(list client.ObjectList, opts ...client.ListOption) error {
	allOpts := append([]client.ListOption{client.InNamespace(c.db.Namespace)}, opts...)
	return c.client.List(c.ctx, list, allOpts...)
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// ObjectMeta returns a pre-configured ObjectMeta for creating resources.
func (c *Cluster) ObjectMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: c.Namespace(),
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "everest",
			"app.kubernetes.io/instance":   c.Name(),
		},
	}
}

// DecodeTopologyConfig unmarshals the topology configuration into the provided struct.
// The target should be a pointer to the expected config type.
// Returns an error if the config is nil, empty, or unmarshaling fails.
//
// Example:
//
//	var config psmdbspec.ShardedTopologyConfig
//	if err := c.DecodeTopologyConfig(&config); err != nil {
//	    // handle error or use defaults
//	}
func (c *Cluster) DecodeTopologyConfig(target interface{}) error {
	topologyConfig := c.db.GetTopologyConfig()
	if topologyConfig == nil || topologyConfig.Raw == nil {
		return fmt.Errorf("topology config not set")
	}
	return json.Unmarshal(topologyConfig.Raw, target)
}

// DecodeGlobalConfig unmarshals the global configuration into the provided struct.
// The target should be a pointer to the expected config type.
// Returns an error if the config is nil, empty, or unmarshaling fails.
//
// Example:
//
//	var config psmdbspec.GlobalConfig
//	if err := c.DecodeGlobalConfig(&config); err != nil {
//	    // handle error or use defaults
//	}
func (c *Cluster) DecodeGlobalConfig(target interface{}) error {
	globalConfig := c.db.Spec.Global
	if globalConfig == nil || globalConfig.Raw == nil {
		return fmt.Errorf("global config not set")
	}
	return json.Unmarshal(globalConfig.Raw, target)
}

// DecodeComponentCustomSpec unmarshals a component's custom spec into the provided struct.
// The target should be a pointer to the expected custom spec type.
// Returns an error if the custom spec is nil, empty, or unmarshaling fails.
//
// Example:
//
//	engine := c.db.Spec.Components["engine"]
//	var customSpec psmdbspec.MongodCustomSpec
//	if err := c.DecodeComponentCustomSpec(engine, &customSpec); err != nil {
//	    // handle error or use defaults
//	}
func (c *Cluster) DecodeComponentCustomSpec(component v2alpha1.ComponentSpec, target interface{}) error {
	if component.CustomSpec == nil || component.CustomSpec.Raw == nil {
		return fmt.Errorf("component custom spec not set")
	}
	return json.Unmarshal(component.CustomSpec.Raw, target)
}

// TryDecodeTopologyConfig attempts to decode topology config, returning false if not set.
// This is a convenience method that doesn't return an error for missing configs.
//
// Example:
//
//	var config psmdbspec.ShardedTopologyConfig
//	if c.TryDecodeTopologyConfig(&config) {
//	    numShards = config.NumShards
//	} else {
//	    numShards = 2 // default
//	}
func (c *Cluster) TryDecodeTopologyConfig(target interface{}) bool {
	err := c.DecodeTopologyConfig(target)
	return err == nil
}

// TryDecodeGlobalConfig attempts to decode global config, returning false if not set.
func (c *Cluster) TryDecodeGlobalConfig(target interface{}) bool {
	err := c.DecodeGlobalConfig(target)
	return err == nil
}

// TryDecodeComponentCustomSpec attempts to decode component custom spec, returning false if not set.
func (c *Cluster) TryDecodeComponentCustomSpec(component v2alpha1.ComponentSpec, target interface{}) bool {
	err := c.DecodeComponentCustomSpec(component, target)
	return err == nil
}

// =============================================================================
// STATUS TYPES
// =============================================================================

// Status represents the current state of the database cluster.
type Status struct {
	Phase         v2alpha1.DataStorePhase
	Message       string
	ConnectionURL string
	Credentials   string // Secret name containing credentials
	Components    []ComponentStatus
}

// ComponentStatus represents the status of a single component.
type ComponentStatus struct {
	Name  string
	Ready int32
	Total int32
	State string // "Ready", "InProgress", "Error"
}

// ToV2Alpha1 converts Status to the API type.
func (s Status) ToV2Alpha1() v2alpha1.DataStoreStatus {
	status := v2alpha1.DataStoreStatus{
		Phase:         s.Phase,
		ConnectionURL: s.ConnectionURL,
	}
	if s.Credentials != "" {
		status.CredentialSecretRef.Name = s.Credentials
	}
	return status
}

// Status helper functions

// Creating returns a status indicating the cluster is being created.
func Creating(message string) Status {
	return Status{Phase: v2alpha1.DataStorePhaseCreating, Message: message}
}

// Running returns a status indicating the cluster is running.
func Running() Status {
	return Status{Phase: v2alpha1.DataStorePhaseRunning}
}

// RunningWithConnection returns a running status with connection details.
func RunningWithConnection(url, credentialsSecret string) Status {
	return Status{
		Phase:         v2alpha1.DataStorePhaseRunning,
		ConnectionURL: url,
		Credentials:   credentialsSecret,
	}
}

// Failed returns a status indicating the cluster has failed.
func Failed(message string) Status {
	return Status{Phase: v2alpha1.DataStorePhaseFailed, Message: message}
}

// =============================================================================
// WAIT HELPERS
// =============================================================================

// WaitError signals that a step is waiting for something.
type WaitError struct {
	Reason   string
	Duration time.Duration
}

func (e *WaitError) Error() string {
	return fmt.Sprintf("waiting: %s", e.Reason)
}

// IsWaitError checks if an error is a WaitError.
func IsWaitError(err error) bool {
	_, ok := err.(*WaitError)
	return ok
}

// GetWaitDuration returns the wait duration from a WaitError.
func GetWaitDuration(err error) time.Duration {
	if we, ok := err.(*WaitError); ok {
		return we.Duration
	}
	return 10 * time.Second
}

// WaitFor returns an error indicating the step should be retried.
func WaitFor(reason string) error {
	return &WaitError{Reason: reason, Duration: 10 * time.Second}
}

// WaitForDuration returns an error indicating retry after a specific duration.
func WaitForDuration(reason string, d time.Duration) error {
	return &WaitError{Reason: reason, Duration: d}
}

