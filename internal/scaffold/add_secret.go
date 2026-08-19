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

// AddSecretConfig holds the configuration for scaffolding a Secret definition.
type AddSecretConfig struct {
	// Name is the secret definition name and directory name under
	// definition/secrets/.
	Name string
}

// AddSecret creates definition/secrets/<name>/{definition.yaml, ui.yaml,
// types.go} in the current provider project.
func AddSecret(cfg *AddSecretConfig) error {
	if err := validateName(cfg.Name); err != nil {
		return fmt.Errorf("invalid secret name: %w", err)
	}

	if _, err := os.Stat("definition/provider.yaml"); err != nil {
		return fmt.Errorf("not in a provider project root (definition/provider.yaml not found)")
	}

	secretDir := filepath.Join("definition", "secrets", cfg.Name)
	if _, err := os.Stat(secretDir); err == nil {
		return fmt.Errorf("secret %q already exists at %s", cfg.Name, secretDir)
	}
	if err := os.MkdirAll(secretDir, 0o755); err != nil {
		return fmt.Errorf("creating secret directory: %w", err)
	}

	if err := createSecretDefinitionYAML(secretDir, cfg); err != nil {
		return fmt.Errorf("creating definition.yaml: %w", err)
	}
	if err := createSecretUIYAML(secretDir, cfg); err != nil {
		return fmt.Errorf("creating ui.yaml: %w", err)
	}
	if err := createSecretTypes(secretDir, cfg); err != nil {
		return fmt.Errorf("creating types.go: %w", err)
	}
	return nil
}

func createSecretDefinitionYAML(secretDir string, cfg *AddSecretConfig) error {
	typeName := toPascalCase(cfg.Name) + "SecretData"

	def := map[string]any{
		"parametersSchema": map[string]any{
			"openAPIV3Schema": typeName,
		},
	}

	header := fmt.Sprintf(`# %s Secret definition.
# This file is the source of truth for the provider's secret type definition.
#
# Fields:
#   parametersSchema.openAPIV3Schema: Go type name (in this package) describing
#     the expected secret data keys. Resolved to an OpenAPI schema at generation time.
#
# Co-located files:
#   ui.yaml  — UI rendering hints for the secret creation form.
#   types.go — Go types referenced above; OpenAPI-extracted by provider-sdk generate.
`, toPascalCase(cfg.Name))

	return writeYAMLWithHeader(filepath.Join(secretDir, "definition.yaml"), def, header)
}

func createSecretUIYAML(secretDir string, cfg *AddSecretConfig) error {
	ui := map[string]any{
		"label":           toPascalCase(cfg.Name) + " Secret",
		"componentsOrder": []any{},
		"components":      map[string]any{},
	}

	header := `# UI rendering hints for this secret type.
# Inlined verbatim under spec.secrets[name].uiSchema in the generated provider spec.
#
# Recommended shape:
#   label           — human-readable name for the secret form.
#   componentsOrder — ordered list of field names for rendering.
#   components      — map of field names to UI hints (type, label, description, etc.).
`
	return writeYAMLWithHeader(filepath.Join(secretDir, "ui.yaml"), ui, header)
}

func createSecretTypes(secretDir string, cfg *AddSecretConfig) error {
	pkgName := toGoIdent(cfg.Name)
	typeName := toPascalCase(cfg.Name) + "SecretData"

	content := fmt.Sprintf(`// Package %s contains the schema-bearing Go types for the
// %q secret type. Each struct here is converted to an OpenAPI
// v3 schema by `+"`provider-sdk generate`"+` and inlined into the generated
// provider spec.
//
// +k8s:openapi-gen=true
package %s

// %s describes the expected data keys for this secret type.
// Add fields that define what keys should be present in the secret's data.
//
// Example:
//   type %s struct {
//       // Username is the database username.
//       Username string `+"`json:\"username\"`"+`
//       // Password is the database password.
//       Password string `+"`json:\"password\"`"+`
//   }
type %s struct{}
`, pkgName, cfg.Name, pkgName,
		typeName, typeName, typeName,
	)
	return os.WriteFile(filepath.Join(secretDir, "types.go"), []byte(content), 0o644)
}
