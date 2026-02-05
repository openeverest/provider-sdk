/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package psmdbspec contains custom spec types for the PSMDB (Percona Server MongoDB) provider.
// These types are annotated with k8s:validation markers for OpenAPI schema generation.
//
// To regenerate OpenAPI schemas after modifying these types:
//
//	make generate-openapi
//
// The generated code will be placed in examples/psmdbspec/generated/zz_generated.openapi.go
//
// +k8s:openapi-gen=true
package types

// =============================================================================
// MONGOD COMPONENT SPEC
// =============================================================================

// MongodCustomSpec defines custom configuration for mongod components.
// This struct is converted to OpenAPI schema and served via the /schema endpoint.
// Provider users can specify these fields in the DataStore's component CustomSpec.
type MongodCustomSpec struct{}

// =============================================================================
// MONGOS COMPONENT SPEC
// =============================================================================

// MongosCustomSpec defines custom configuration for mongos (proxy) components.
type MongosCustomSpec struct{}

// =============================================================================
// PMM (MONITORING) COMPONENT SPEC
// =============================================================================

// PMMCustomSpec defines custom configuration for PMM monitoring.
// This allows users to configure PMM as a monitoring solution for PSMDB.
// +k8s:openapi-gen=true
type PMMCustomSpec struct {
	// ServerHost specifies the hostname/IP of the PMM server.
	ServerHost string `json:"serverHost,omitempty"`

	// SecretRef references a Kubernetes Secret containing PMM credentials.
	SecretRef *PMMSecretRef `json:"secretRef,omitempty"`
}

// PMMSecretRef references a Kubernetes Secret for PMM authentication.
// +k8s:openapi-gen=true
type PMMSecretRef struct {
	// Name is the name of the Kubernetes Secret.
	Name string `json:"name,omitempty"`
}

// =============================================================================
// BACKUP COMPONENT SPEC
// =============================================================================

// BackupCustomSpec defines custom configuration for backup agents.
type BackupCustomSpec struct{}

// =============================================================================
// TOPOLOGY SPECS
// =============================================================================

// TopologyType defines the type of deployment topology.
type TopologyType string

const (
	// TopologyTypeReplicaSet represents a replica set topology.
	TopologyTypeReplicaSet TopologyType = "replicaSet"
	// TopologyTypeSharded represents a sharded cluster topology.
	TopologyTypeSharded TopologyType = "sharded"
)

// ReplicaSetTopologyConfig defines configuration for replica set topology.
type ReplicaSetTopologyConfig struct {
}

// ShardedTopologyConfig defines configuration for sharded cluster topology.
type ShardedTopologyConfig struct {
	// NumShards specifies the initial number of shards.
	// +k8s:validation:minimum=1
	// +default=2
	// +optional
	NumShards int32 `json:"numShards,omitempty"`
}

// =============================================================================
// GLOBAL CONFIG
// =============================================================================

// GlobalConfig defines global configuration that applies to the entire cluster.
type GlobalConfig struct{}
