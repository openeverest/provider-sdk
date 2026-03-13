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
	"strings"

	"gopkg.in/yaml.v3"
)

// AddComponentConfig holds the configuration for adding a component.
type AddComponentConfig struct {
	ComponentName string // e.g., "backupAgent"
	ComponentType string // e.g., "backup"
}

// AddComponent adds a new component to an existing provider project.
// It updates definition/provider.yaml, definition/versions.yaml,
// definition/components/types.go, and internal/common/spec.go.
func AddComponent(cfg *AddComponentConfig) error {
	if cfg.ComponentName == "" {
		return fmt.Errorf("component name is required")
	}
	if cfg.ComponentType == "" {
		return fmt.Errorf("component type is required")
	}

	// Ensure we're in a provider project.
	if _, err := os.Stat("definition/provider.yaml"); err != nil {
		return fmt.Errorf("not in a provider project root (definition/provider.yaml not found)")
	}

	// 1. Update definition/provider.yaml
	if err := addComponentToProviderYAML(cfg); err != nil {
		return fmt.Errorf("updating provider.yaml: %w", err)
	}

	// 2. Update definition/versions.yaml (add type if new)
	if err := addTypeToVersionsYAML(cfg); err != nil {
		return fmt.Errorf("updating versions.yaml: %w", err)
	}

	// 3. Update definition/components/types.go
	if err := addComponentTypeStruct(cfg); err != nil {
		return fmt.Errorf("updating components/types.go: %w", err)
	}

	// 4. Update internal/common/spec.go
	if err := addComponentConstants(cfg); err != nil {
		return fmt.Errorf("updating common/spec.go: %w", err)
	}

	return nil
}

// addComponentToProviderYAML adds a component entry to definition/provider.yaml.
func addComponentToProviderYAML(cfg *AddComponentConfig) error {
	data, err := os.ReadFile("definition/provider.yaml")
	if err != nil {
		return err
	}

	var provider map[string]any
	if err := yaml.Unmarshal(data, &provider); err != nil {
		return fmt.Errorf("parsing provider.yaml: %w", err)
	}

	// Get or create components map.
	compsRaw, ok := provider["components"]
	if !ok {
		compsRaw = map[string]any{}
		provider["components"] = compsRaw
	}
	comps, ok := compsRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("components field is not a map")
	}

	// Check if component already exists.
	if _, exists := comps[cfg.ComponentName]; exists {
		return fmt.Errorf("component %q already exists in provider.yaml", cfg.ComponentName)
	}

	// Add the component.
	comps[cfg.ComponentName] = map[string]any{
		"type": cfg.ComponentType,
	}

	return writeYAMLWithHeader(
		"definition/provider.yaml",
		provider,
		"# Provider identity and component definitions.\n"+
			"# Edit this file to add or remove components.\n"+
			"# Run `make generate` after making changes.\n",
	)
}

// addTypeToVersionsYAML adds a component type to definition/versions.yaml if not already present.
func addTypeToVersionsYAML(cfg *AddComponentConfig) error {
	data, err := os.ReadFile("definition/versions.yaml")
	if err != nil {
		return err
	}

	var versions map[string]any
	if err := yaml.Unmarshal(data, &versions); err != nil {
		return fmt.Errorf("parsing versions.yaml: %w", err)
	}

	// Get or create componentTypes map.
	ctRaw, ok := versions["componentTypes"]
	if !ok {
		ctRaw = map[string]any{}
		versions["componentTypes"] = ctRaw
	}
	ct, ok := ctRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("componentTypes field is not a map")
	}

	// If type already exists, skip.
	if _, exists := ct[cfg.ComponentType]; exists {
		fmt.Printf("  Component type %q already exists in versions.yaml (skipped)\n", cfg.ComponentType)
		return nil
	}

	// Add placeholder version entry.
	ct[cfg.ComponentType] = map[string]any{
		"versions": []any{
			map[string]any{
				"version": "1.0.0",
				"image":   fmt.Sprintf("example/%s:1.0.0", cfg.ComponentType),
				"default": true,
			},
		},
	}

	return writeYAMLWithHeader(
		"definition/versions.yaml",
		versions,
		"# Component type version catalog.\n"+
			"# Add new versions here when upstream releases are available.\n"+
			"# Mark exactly one version per type as `default: true`.\n",
	)
}

// addComponentTypeStruct appends a CustomSpec struct to definition/components/types.go.
func addComponentTypeStruct(cfg *AddComponentConfig) error {
	path := "definition/components/types.go"
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	content := string(data)
	structName := toExportedName(cfg.ComponentType) + "CustomSpec"

	// Check if struct already exists.
	if strings.Contains(content, "type "+structName+" struct") {
		fmt.Printf("  Struct %s already exists in components/types.go (skipped)\n", structName)
		return nil
	}

	// Append the new struct.
	newStruct := fmt.Sprintf(
		"\n// %s defines custom configuration for %s components.\n"+
			"// Add fields here when the %s component type needs custom configuration\n"+
			"// beyond what the base Instance spec provides.\n"+
			"type %s struct{}\n",
		structName, cfg.ComponentType, cfg.ComponentType, structName,
	)

	return os.WriteFile(path, []byte(content+newStruct), 0o644)
}

// addComponentConstants adds component name and type constants to internal/common/spec.go.
func addComponentConstants(cfg *AddComponentConfig) error {
	path := "internal/common/spec.go"
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	content := string(data)

	// Build the constant names.
	nameConst := "Component" + toExportedName(cfg.ComponentName)
	typeConst := "ComponentType" + toExportedName(cfg.ComponentType)

	// Check if constants already exist.
	nameExists := strings.Contains(content, nameConst+" ")
	typeExists := strings.Contains(content, typeConst+" ")

	if nameExists && typeExists {
		fmt.Printf("  Constants already exist in common/spec.go (skipped)\n")
		return nil
	}

	// Find the closing paren of the const block.
	constClose := strings.LastIndex(content, ")")
	if constClose == -1 {
		return fmt.Errorf("could not find const block closing paren in spec.go")
	}

	var additions string
	if !nameExists {
		additions += fmt.Sprintf("\t%s = %q\n", nameConst, cfg.ComponentName)
	}
	if !typeExists {
		additions += fmt.Sprintf("\t%s = %q\n", typeConst, cfg.ComponentType)
	}

	// Insert before the closing paren.
	newContent := content[:constClose] + "\n" + additions + content[constClose:]
	return os.WriteFile(path, []byte(newContent), 0o644)
}

// writeYAMLWithHeader marshals data to YAML with a header comment and writes to path.
func writeYAMLWithHeader(path string, data any, header string) error {
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}
	return os.WriteFile(path, []byte(header+"\n"+string(out)), 0o644)
}

// toExportedName converts the first letter to uppercase.
// E.g., "backupAgent" → "BackupAgent", "mongod" → "Mongod".
func toExportedName(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
