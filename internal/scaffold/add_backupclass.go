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

// AddBackupClassConfig holds the configuration for scaffolding a BackupClass.
type AddBackupClassConfig struct {
	// Name is the BackupClass metadata.name and directory name under
	// definition/backupclasses/.
	Name string
	// ExecutionMode selects between "ProviderManaged" (default) and "Job".
	// Only ProviderManaged classes carry providerManaged.limits / pitrConfigSchema
	// stubs in the generated class.yaml.
	ExecutionMode string
}

// AddBackupClass creates definition/backupclasses/<name>/{class.yaml, ui.yaml,
// types.go} in the current provider project.
func AddBackupClass(cfg *AddBackupClassConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("backup class name is required")
	}
	if cfg.ExecutionMode == "" {
		cfg.ExecutionMode = "ProviderManaged"
	}
	if cfg.ExecutionMode != "ProviderManaged" && cfg.ExecutionMode != "Job" {
		return fmt.Errorf("invalid execution mode %q: must be ProviderManaged or Job", cfg.ExecutionMode)
	}

	if _, err := os.Stat("definition/provider.yaml"); err != nil {
		return fmt.Errorf("not in a provider project root (definition/provider.yaml not found)")
	}

	providerName, err := readProviderName()
	if err != nil {
		return fmt.Errorf("reading provider name: %w", err)
	}

	bcDir := filepath.Join("definition", "backupclasses", cfg.Name)
	if _, err := os.Stat(bcDir); err == nil {
		return fmt.Errorf("backup class %q already exists at %s", cfg.Name, bcDir)
	}
	if err := os.MkdirAll(bcDir, 0o755); err != nil {
		return fmt.Errorf("creating backupclass directory: %w", err)
	}

	if err := createBackupClassYAML(bcDir, cfg, providerName); err != nil {
		return fmt.Errorf("creating class.yaml: %w", err)
	}
	if err := createBackupClassUIYAML(bcDir, cfg); err != nil {
		return fmt.Errorf("creating ui.yaml: %w", err)
	}
	if err := createBackupClassTypes(bcDir, cfg); err != nil {
		return fmt.Errorf("creating types.go: %w", err)
	}
	return nil
}

func readProviderName() (string, error) {
	data, err := os.ReadFile("definition/provider.yaml")
	if err != nil {
		return "", err
	}
	var provider map[string]any
	if err := yaml.Unmarshal(data, &provider); err != nil {
		return "", err
	}
	if name, ok := provider["name"].(string); ok {
		return name, nil
	}
	return "", fmt.Errorf("definition/provider.yaml missing 'name' field")
}

func createBackupClassYAML(bcDir string, cfg *AddBackupClassConfig, providerName string) error {
	configTypeName := toPascalCase(cfg.Name) + "BackupConfig"
	restoreTypeName := toPascalCase(cfg.Name) + "RestoreConfig"
	pitrTypeName := toPascalCase(cfg.Name) + "PITRConfig"

	class := map[string]any{
		"displayName":        toPascalCase(cfg.Name) + " Backup Class",
		"description":        "TODO: describe what this BackupClass does.",
		"supportedProviders": []any{providerName},
		"executionMode":      cfg.ExecutionMode,
		"config": map[string]any{
			"openAPIV3Schema": configTypeName,
		},
		"restoreConfig": map[string]any{
			"openAPIV3Schema": restoreTypeName,
		},
	}

	if cfg.ExecutionMode == "ProviderManaged" {
		class["providerManaged"] = map[string]any{
			"supportsPITR": false,
			// Limits are commented hints in the YAML header; leave the map
			// minimal so developers opt into specific caps by uncommenting.
			"limits":           map[string]any{},
			"pitrConfigSchema": pitrTypeName,
		}
	}

	header := fmt.Sprintf(`# %s BackupClass definition.
# This file is the source of truth for the generated BackupClass manifest.
#
# Fields:
#   displayName/description: human-readable metadata.
#   supportedProviders: providers this class can be used with.
#   executionMode: "ProviderManaged" (operator-native) or "Job".
#   providerManaged.supportsPITR: advertise PITR capability to Restore validation.
#   providerManaged.limits: caps the runtime enforces against Instance.spec.backup.
#     maxStorages, maxPITREnabledStorages, maxSchedulesPerStorage — unset means unlimited.
#   providerManaged.pitrConfigSchema: Go type name (in this package) describing
#     per-storage PITR custom config. Resolved to an OpenAPI schema at generation time.
#   config.openAPIV3Schema / restoreConfig.openAPIV3Schema: Go type names
#     describing backup-time and restore-time custom config respectively.
#
# Co-located files:
#   ui.yaml  — free-form UI rendering hints, inlined verbatim under spec.uiSchema.
#   types.go — Go types referenced above; OpenAPI-extracted by provider-sdk generate.
`, toPascalCase(cfg.Name))

	return writeYAMLWithHeader(filepath.Join(bcDir, "class.yaml"), class, header)
}

func createBackupClassUIYAML(bcDir string, _ *AddBackupClassConfig) error {
	ui := map[string]any{
		"backup": map[string]any{
			"label":           "Backup Configuration",
			"componentsOrder": []any{},
			"components":      map[string]any{},
		},
		"pitr": map[string]any{
			"label":           "PITR Configuration",
			"componentsOrder": []any{},
			"components":      map[string]any{},
		},
		"restore": map[string]any{
			"label":           "Restore Configuration",
			"componentsOrder": []any{},
			"components":      map[string]any{},
		},
	}

	header := `# UI rendering hints for this BackupClass.
# Inlined verbatim under spec.uiSchema in the generated manifest; treated as
# opaque by the runtime. The recommended shape groups fields by the modal
# that renders them:
#   backup  — on-demand backup modal (fields backed by config.openAPIV3Schema).
#   pitr    — per-storage PITR sub-form (fields backed by providerManaged.pitrConfigSchema).
#   restore — restore modal (fields backed by restoreConfig.openAPIV3Schema).
`
	return writeYAMLWithHeader(filepath.Join(bcDir, "ui.yaml"), ui, header)
}

func createBackupClassTypes(bcDir string, cfg *AddBackupClassConfig) error {
	pkgName := toGoIdent(cfg.Name)
	cfgType := toPascalCase(cfg.Name) + "BackupConfig"
	restoreType := toPascalCase(cfg.Name) + "RestoreConfig"
	pitrType := toPascalCase(cfg.Name) + "PITRConfig"

	content := fmt.Sprintf(`// Package %s contains the schema-bearing Go types for the
// %q BackupClass. Each struct here is converted to an OpenAPI
// v3 schema by `+"`provider-sdk generate`"+` and inlined into the generated
// BackupClass manifest.
//
// +k8s:openapi-gen=true
package %s

// %s describes the configuration accepted by Backup CRs that
// target this class (spec.config). Add fields the user can set per backup.
type %s struct{}

// %s describes the configuration accepted by Restore CRs that
// target this class (spec.config). Add fields the user can set per restore.
type %s struct{}

// %s describes the per-storage PITR custom config exposed to
// Instance.spec.backup.storages[].pitr.config. Add fields a provider needs
// to fine-tune its PITR pipeline (oplog span, compression, retention, etc.).
type %s struct{}
`, pkgName, cfg.Name, pkgName,
		cfgType, cfgType,
		restoreType, restoreType,
		pitrType, pitrType,
	)
	return os.WriteFile(filepath.Join(bcDir, "types.go"), []byte(content), 0o644)
}

// toGoIdent normalizes a backup class name into a valid Go identifier
// (lower-case, no separators). e.g. "everest-percona-psmdb-operator" → "everestperconapsmdboperator".
func toGoIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		}
	}
	out := b.String()
	if out == "" {
		out = "backupclass"
	}
	return out
}
