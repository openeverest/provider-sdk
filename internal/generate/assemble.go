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

// Package generate implements the provider definition assembly and Provider CR
// spec generation, reading from the definition/ directory structure and producing
// the Helm chart's generated/provider-spec.yaml intermediate file.
package generate

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AssembledConfig holds the assembled provider configuration from the definition directory.
type AssembledConfig struct {
	// Name is the provider name from definition/provider.yaml.
	Name string

	// ComponentTypes maps component type names to their version catalogs.
	ComponentTypes map[string]any

	// Components maps logical component names to their spec (type, optional customSpecSchema).
	Components map[string]any

	// Topologies maps topology names to their Provider CR spec representation.
	Topologies map[string]any

	// UISchema maps topology names to their UI rendering hints.
	UISchema map[string]any

	// GlobalConfigSchema is the Go type name reference for global config, if any.
	GlobalConfigSchema string

	// TypeRefs collects all Go type name references found during assembly
	// (e.g., configSchema, customSpecSchema, globalConfigSchema values).
	TypeRefs map[string]bool
}

// Assemble reads the definition directory and builds an assembled provider configuration.
//
// It reads:
//   - definition/provider.yaml    — provider name and component→type mapping
//   - definition/versions.yaml    — component type version catalogs
//   - definition/topologies/*/topology.yaml — topology configs and UI schemas
//
// Type name references (configSchema, customSpecSchema, globalConfigSchema) are
// collected in TypeRefs for later schema resolution.
func Assemble(defDir string) (*AssembledConfig, error) {
	provider, err := readYAML(filepath.Join(defDir, "provider.yaml"))
	if err != nil {
		return nil, fmt.Errorf("reading provider.yaml: %w", err)
	}

	versions, err := readYAML(filepath.Join(defDir, "versions.yaml"))
	if err != nil {
		return nil, fmt.Errorf("reading versions.yaml: %w", err)
	}

	cfg := &AssembledConfig{
		TypeRefs: make(map[string]bool),
	}

	// Provider name.
	if name, ok := provider["name"].(string); ok {
		cfg.Name = name
	}

	// Component types from versions.yaml.
	if ct, ok := versions["componentTypes"]; ok {
		if m, ok := ct.(map[string]any); ok {
			cfg.ComponentTypes = m
		}
	}

	// Components from provider.yaml.
	if comps, ok := provider["components"]; ok {
		if m, ok := comps.(map[string]any); ok {
			cfg.Components = buildComponentsSpec(m, cfg.TypeRefs)
		}
	}

	// Global config schema reference.
	if gcs, ok := provider["globalConfigSchema"].(string); ok && gcs != "" {
		cfg.GlobalConfigSchema = gcs
		cfg.TypeRefs[gcs] = true
	}

	// Topologies and UI schema from topology directories.
	topologiesDir := filepath.Join(defDir, "topologies")
	entries, err := os.ReadDir(topologiesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading topologies directory: %w", err)
	}

	topologies := make(map[string]any)
	uiSchema := make(map[string]any)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		topoName := entry.Name()
		topoFile := filepath.Join(topologiesDir, topoName, "topology.yaml")
		if _, err := os.Stat(topoFile); err != nil {
			continue // skip directories without a topology.yaml
		}
		topoData, err := readYAML(topoFile)
		if err != nil {
			return nil, fmt.Errorf("reading topology %s: %w", topoName, err)
		}
		if configRaw, ok := topoData["config"]; ok {
			if configMap, ok := configRaw.(map[string]any); ok {
				topologies[topoName] = buildTopologySpec(configMap, cfg.TypeRefs)
			}
		}
		if ui, ok := topoData["ui"]; ok {
			uiSchema[topoName] = ui
		}
	}

	if len(topologies) > 0 {
		cfg.Topologies = topologies
	}
	if len(uiSchema) > 0 {
		cfg.UISchema = uiSchema
	}

	return cfg, nil
}

// buildComponentsSpec builds the components section of the Provider CR spec.
// It also collects type references from customSpecSchema fields.
func buildComponentsSpec(comps map[string]any, typeRefs map[string]bool) map[string]any {
	spec := make(map[string]any)
	for name, compRaw := range comps {
		comp := make(map[string]any)
		if compMap, ok := compRaw.(map[string]any); ok {
			if t, ok := compMap["type"].(string); ok {
				comp["type"] = t
			}
			if schema, ok := compMap["customSpecSchema"].(string); ok && schema != "" {
				typeRefs[schema] = true
				comp["customSpecSchema"] = schema // placeholder, resolved later
			}
		}
		spec[name] = comp
	}
	return spec
}

// buildTopologySpec builds a single topology entry for the Provider CR spec.
// It extracts only CR-valid fields (optional, configSchema) from the topology config,
// intentionally dropping non-CR fields like "defaults".
func buildTopologySpec(configMap map[string]any, typeRefs map[string]bool) map[string]any {
	result := make(map[string]any)

	// Extract configSchema reference (will be resolved to OpenAPI schema later).
	if cs, ok := configMap["configSchema"].(string); ok && cs != "" {
		typeRefs[cs] = true
		result["configSchema"] = cs // placeholder, resolved later
	}

	// Extract components, filtering to only CR-valid fields.
	if compsRaw, ok := configMap["components"]; ok {
		if comps, ok := compsRaw.(map[string]any); ok {
			specComps := make(map[string]any)
			for compName, compRaw := range comps {
				specComp := make(map[string]any)
				if compMap, ok := compRaw.(map[string]any); ok {
					if opt, ok := compMap["optional"]; ok {
						specComp["optional"] = opt
					}
					// Intentionally skip "defaults" and other non-CR fields.
				}
				specComps[compName] = specComp
			}
			result["components"] = specComps
		}
	}

	return result
}

func readYAML(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return m, nil
}
