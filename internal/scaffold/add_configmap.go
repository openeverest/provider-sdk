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
)

// AddConfigMapConfig holds the configuration for scaffolding a ConfigMap definition.
type AddConfigMapConfig struct {
	// Name is the configmap definition name and directory name under
	// definition/configmaps/.
	Name string
}

// AddConfigMap creates definition/configmaps/<name>/{definition.yaml, ui.yaml,
// types.go} in the current provider project.
func AddConfigMap(cfg *AddConfigMapConfig) error {
	if err := validateName(cfg.Name); err != nil {
		return fmt.Errorf("invalid configmap name: %w", err)
	}

	if _, err := os.Stat("definition/provider.yaml"); err != nil {
		return fmt.Errorf("not in a provider project root (definition/provider.yaml not found)")
	}

	cmDir := filepath.Join("definition", "configmaps", cfg.Name)
	if _, err := os.Stat(cmDir); err == nil {
		return fmt.Errorf("configmap %q already exists at %s", cfg.Name, cmDir)
	}
	if err := os.MkdirAll(cmDir, 0o755); err != nil {
		return fmt.Errorf("creating configmap directory: %w", err)
	}

	if err := createConfigMapDefinitionYAML(cmDir, cfg); err != nil {
		return fmt.Errorf("creating definition.yaml: %w", err)
	}
	if err := createConfigMapUIYAML(cmDir, cfg); err != nil {
		return fmt.Errorf("creating ui.yaml: %w", err)
	}
	if err := createConfigMapTypes(cmDir, cfg); err != nil {
		return fmt.Errorf("creating types.go: %w", err)
	}
	return nil
}

func createConfigMapDefinitionYAML(cmDir string, cfg *AddConfigMapConfig) error {
	typeName := toPascalCase(cfg.Name) + "ConfigMapData"

	def := map[string]any{
		"parametersSchema": map[string]any{
			"openAPIV3Schema": typeName,
		},
	}

	header := fmt.Sprintf(`# %s ConfigMap definition.
# This file is the source of truth for the provider's configmap type definition.
#
# Fields:
#   parametersSchema.openAPIV3Schema: Go type name (in this package) describing
#     the expected configmap data keys. Resolved to an OpenAPI schema at generation time.
#
# Co-located files:
#   ui.yaml  — UI rendering hints for the configmap creation form.
#   types.go — Go types referenced above; OpenAPI-extracted by provider-sdk generate.
`, toPascalCase(cfg.Name))

	return writeYAMLWithHeader(filepath.Join(cmDir, "definition.yaml"), def, header)
}

func createConfigMapUIYAML(cmDir string, cfg *AddConfigMapConfig) error {
	ui := map[string]any{
		"label":           toPascalCase(cfg.Name) + " ConfigMap",
		"componentsOrder": []any{},
		"components":      map[string]any{},
	}

	header := `# UI rendering hints for this configmap type.
# Inlined verbatim under spec.configMaps[name].uiSchema in the generated provider spec.
#
# Recommended shape:
#   label           — human-readable name for the configmap form.
#   componentsOrder — ordered list of field names for rendering.
#   components      — map of field names to UI hints (type, label, description, etc.).
`
	return writeYAMLWithHeader(filepath.Join(cmDir, "ui.yaml"), ui, header)
}

func createConfigMapTypes(cmDir string, cfg *AddConfigMapConfig) error {
	pkgName := toGoIdent(cfg.Name)
	typeName := toPascalCase(cfg.Name) + "ConfigMapData"

	content := fmt.Sprintf(`// Package %s contains the schema-bearing Go types for the
// %q configmap type. Each struct here is converted to an OpenAPI
// v3 schema by `+"`provider-sdk generate`"+` and inlined into the generated
// provider spec.
//
// +k8s:openapi-gen=true
package %s

// %s describes the expected data keys for this configmap type.
// Add fields that define what keys should be present in the configmap's data.
//
// Example:
//   type %s struct {
//       // ConfigFile is the main configuration file content.
//       ConfigFile string `+"`json:\"config.yaml\"`"+`
//       // Settings contains additional settings.
//       Settings string `+"`json:\"settings.json\"`"+`
//   }
type %s struct{}
`, pkgName, cfg.Name, pkgName,
		typeName, typeName, typeName,
	)
	return os.WriteFile(filepath.Join(cmDir, "types.go"), []byte(content), 0o644)
}
