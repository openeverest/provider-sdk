// Copyright (C) 2026 The OpenEverest Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package instanceshape is a type universe for exercising UI schema path
// resolution.
//
// It is deliberately not a copy of core's Instance types, and carries no
// obligation to track them. It exists to cover each Go construct the resolver
// has to traverse: struct fields addressed by JSON tag, pointers, maps keyed by
// component or resource name, inlined embedded structs, and the opaque
// parameters boundary. Add to it when the resolver learns to handle a new
// construct — not when core changes shape.
//
// Coverage against the real core types comes from running generate against the
// providers, which resolve paths against whichever core version they pin.
package instanceshape

type InstanceSpec struct {
	Version    string                   `json:"version,omitempty"`
	Topology   *TopologySpec            `json:"topology,omitempty"`
	Parameters *RawExtension            `json:"parameters,omitempty"`
	Components map[string]ComponentSpec `json:"components,omitempty"`
}

type TopologySpec struct {
	Type       string        `json:"type,omitempty"`
	Parameters *RawExtension `json:"parameters,omitempty"`
}

type ComponentSpec struct {
	CommonComponentFields `json:",inline"`

	Replicas   *int32                `json:"replicas,omitempty"`
	Storage    *Storage              `json:"storage,omitempty"`
	Resources  *ResourceRequirements `json:"resources,omitempty"`
	Parameters *RawExtension         `json:"parameters,omitempty"`
}

type CommonComponentFields struct {
	Image string `json:"image,omitempty"`
}

type Storage struct {
	Size Quantity `json:"size,omitempty"`
}

type ResourceRequirements struct {
	Limits map[string]Quantity `json:"limits,omitempty"`
}

type Quantity struct {
	i int64
}

type RawExtension struct {
	Raw []byte `json:"-"`
}

// EngineParameters stands in for a provider-declared parametersSchema type.
type EngineParameters struct {
	Configuration string `json:"configuration,omitempty"`
}

// TopologyParameters stands in for a topology-level parametersSchema type.
type TopologyParameters struct {
	NumShards int32 `json:"numShards,omitempty"`
}
