// Package [[ .TopologyPackage ]] contains parameter types for the [[ .TopologyName ]] topology.
//
// Add fields to [[ .TopologyTypeName ]]TopologyParameters and reference it via parametersSchema in
// topology.yaml when this topology needs parameters.
//
// +k8s:openapi-gen=true
package [[ .TopologyPackage ]]

// [[ .TopologyTypeName ]]TopologyParameters defines the parameters for the [[ .TopologyName ]] topology.
// Currently empty — add fields here when the [[ .TopologyName ]] topology needs
// parameters beyond what the base Instance spec provides.
type [[ .TopologyTypeName ]]TopologyParameters struct{}
