// Package [[ .TopologyPackage ]] contains custom spec types for the [[ .TopologyName ]] topology.
//
// Add fields to [[ .TopologyTypeName ]]TopologyConfig and reference it via configSchema in
// topology.yaml when this topology needs custom configuration.
//
// +k8s:openapi-gen=true
package [[ .TopologyPackage ]]

// [[ .TopologyTypeName ]]TopologyConfig defines configuration for the [[ .TopologyName ]] topology.
// Currently empty — add fields here when the [[ .TopologyName ]] topology needs
// custom configuration beyond what the base Instance spec provides.
type [[ .TopologyTypeName ]]TopologyConfig struct{}
