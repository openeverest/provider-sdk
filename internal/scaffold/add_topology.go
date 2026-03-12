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

package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// AddTopologyConfig holds the configuration for adding a topology.
type AddTopologyConfig struct {
	TopologyName       string   // e.g., "replicaSet", "sharded"
	SelectedComponents []string // subset of components to include; nil = all from provider.yaml
}

// AddTopology adds a new topology to an existing provider project.
// It creates definition/topologies/<name>/topology.yaml and types.go.
func AddTopology(cfg *AddTopologyConfig) error {
	if cfg.TopologyName == "" {
		return fmt.Errorf("topology name is required")
	}

	// Ensure we're in a provider project.
	if _, err := os.Stat("definition/provider.yaml"); err != nil {
		return fmt.Errorf("not in a provider project root (definition/provider.yaml not found)")
	}

	topoDir := filepath.Join("definition", "topologies", cfg.TopologyName)

	// Check if topology already exists.
	if _, err := os.Stat(topoDir); err == nil {
		return fmt.Errorf("topology %q already exists at %s", cfg.TopologyName, topoDir)
	}

	// Read provider.yaml to get component list.
	components := cfg.SelectedComponents
	if len(components) == 0 {
		var err error
		components, err = ReadProviderComponents()
		if err != nil {
			return fmt.Errorf("reading components from provider.yaml: %w", err)
		}
	}

	// Create the topology directory.
	if err := os.MkdirAll(topoDir, 0o755); err != nil {
		return fmt.Errorf("creating topology directory: %w", err)
	}

	// Create topology.yaml.
	if err := createTopologyYAML(topoDir, cfg.TopologyName, components); err != nil {
		return fmt.Errorf("creating topology.yaml: %w", err)
	}

	// Create types.go.
	if err := createTopologyTypes(topoDir, cfg.TopologyName); err != nil {
		return fmt.Errorf("creating types.go: %w", err)
	}

	return nil
}

// ReadProviderComponents reads component names from definition/provider.yaml.
func ReadProviderComponents() ([]string, error) {
	data, err := os.ReadFile("definition/provider.yaml")
	if err != nil {
		return nil, err
	}

	var provider map[string]any
	if err := yaml.Unmarshal(data, &provider); err != nil {
		return nil, err
	}

	compsRaw, ok := provider["components"]
	if !ok {
		return nil, nil
	}
	comps, ok := compsRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("components field is not a map")
	}

	names := make([]string, 0, len(comps))
	for name := range comps {
		names = append(names, name)
	}
	return names, nil
}

// createTopologyYAML generates a topology.yaml with all components listed.
func createTopologyYAML(topoDir, topoName string, components []string) error {
	// Build the components section.
	// First component (usually "engine") gets defaults, others are optional.
	compsConfig := make(map[string]any)
	for i, name := range components {
		if i == 0 && name == "engine" {
			compsConfig[name] = map[string]any{
				"defaults": map[string]any{
					"replicas": 3,
				},
			}
		} else {
			compsConfig[name] = map[string]any{
				"optional": true,
			}
		}
	}
	if len(compsConfig) == 0 {
		compsConfig["engine"] = map[string]any{
			"defaults": map[string]any{
				"replicas": 3,
			},
		}
	}

	// Build the full topology structure.
	topology := map[string]any{
		"config": map[string]any{
			"components": compsConfig,
		},
		"ui": map[string]any{
			"sections": map[string]any{
				"basicInfo": map[string]any{
					"label":       "Basic Information",
					"description": "Provide the basic information for your new database.",
					"components": map[string]any{
						"version": map[string]any{
							"uiType": "select",
							"path":   "spec.components.engine.version",
							"fieldParams": map[string]any{
								"label": "Database Version",
							},
						},
					},
				},
				"resources": map[string]any{
					"label":       "Resources",
					"description": "Configure the resources your database will have access to.",
					"components": map[string]any{
						"numberOfNodes": map[string]any{
							"path":   "spec.components.engine.replicas",
							"uiType": "number",
							"fieldParams": map[string]any{
								"label": "Number of nodes",
							},
							"validation": map[string]any{
								"min": 1,
								"max": 7,
							},
						},
						"resources": map[string]any{
							"uiType":    "group",
							"groupType": "line",
							"components": map[string]any{
								"cpu": map[string]any{
									"path":   "spec.components.engine.resources.requests.cpu",
									"uiType": "number",
									"fieldParams": map[string]any{
										"label": "CPU",
									},
									"validation": map[string]any{
										"min": 1,
										"max": 10,
									},
								},
								"memory": map[string]any{
									"path":   "spec.components.engine.resources.requests.memory",
									"uiType": "number",
									"fieldParams": map[string]any{
										"label": "Memory",
									},
									"validation": map[string]any{
										"min": 1,
										"max": 10,
									},
								},
								"disk": map[string]any{
									"path":   "spec.components.engine.storage.size",
									"uiType": "number",
									"fieldParams": map[string]any{
										"label": "Disk",
									},
									"validation": map[string]any{
										"min": 10,
										"max": 100,
									},
								},
							},
						},
					},
				},
			},
			"sectionsOrder": []any{"basicInfo", "resources"},
		},
	}

	header := fmt.Sprintf("# %s topology definition.\n"+
		"# This file defines both the structural topology and its UI rendering.\n"+
		"#\n"+
		"# config.components: Which components are used and their defaults.\n"+
		"# config.configSchema: Reference a Go type for custom topology config fields.\n"+
		"# ui: Rendering hints for the frontend form.\n"+
		"#\n"+
		"# See definition/PROVIDER_DEVELOPMENT.md for detailed guidance.\n",
		toPascalCase(topoName),
	)

	return writeYAMLWithHeader(
		filepath.Join(topoDir, "topology.yaml"),
		topology,
		header,
	)
}

// createTopologyTypes generates a types.go for the topology.
func createTopologyTypes(topoDir, topoName string) error {
	pkgName := strings.ToLower(topoName)
	typeName := toPascalCase(topoName) + "TopologyConfig"

	content := fmt.Sprintf(`// Package %s contains custom spec types for the %s topology.
//
// Add fields to %s and reference it via configSchema in
// topology.yaml when this topology needs custom configuration.
//
// +k8s:openapi-gen=true
package %s

// %s defines configuration for the %s topology.
// Add fields here when the %s topology needs custom configuration
// beyond what the base Instance spec provides.
//
// Example:
//   type %s struct {
//       NumShards int32 `+"`"+`json:"numShards,omitempty"`+"`"+`
//   }
//
// Then reference it in topology.yaml:
//   config:
//     configSchema: %s
type %s struct{}
`, pkgName, topoName, typeName, pkgName, typeName, topoName, topoName, typeName, typeName, typeName)

	return os.WriteFile(filepath.Join(topoDir, "types.go"), []byte(content), 0o644)
}
