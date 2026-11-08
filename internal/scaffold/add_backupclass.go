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
)

// AddBackupClassConfig holds the configuration for scaffolding a BackupClass.
type AddBackupClassConfig struct {
	// Name is the BackupClass metadata.name and directory name under
	// definition/backupclasses/.
	Name string
	// ExecutionMode is the BackupClass execution mode. "ProviderManaged" (the
	// default) is the only mode OpenEverest supports; ProviderManaged classes
	// carry providerManaged.limits / pitrParametersSchema stubs in the
	// generated class.yaml.
	ExecutionMode string
}

// AddBackupClass creates definition/backupclasses/<name>/{class.yaml, ui.yaml,
// types.go} in the current provider project.
func AddBackupClass(cfg *AddBackupClassConfig) error {
	if err := validateResourceName(cfg.Name); err != nil {
		return fmt.Errorf("invalid backup class name for Kubernetes resource: %w", err)
	}

	if err := validateIdentifier(cfg.Name); err != nil {
		return fmt.Errorf("invalid backup class name for Go identifier: %w", err)
	}

	if cfg.ExecutionMode == "" {
		cfg.ExecutionMode = "ProviderManaged"
	}
	if cfg.ExecutionMode != "ProviderManaged" {
		return fmt.Errorf("invalid execution mode %q: must be ProviderManaged", cfg.ExecutionMode)
	}

	if _, err := os.Stat("definition/provider.yaml"); err != nil {
		return fmt.Errorf("not in a provider project root (definition/provider.yaml not found)")
	}

	bcDir := filepath.Join("definition", "backupclasses", cfg.Name)
	if _, err := os.Stat(bcDir); err == nil {
		return fmt.Errorf("backup class %q already exists at %s", cfg.Name, bcDir)
	}
	if err := os.MkdirAll(bcDir, 0o755); err != nil {
		return fmt.Errorf("creating backupclass directory: %w", err)
	}

	if err := createBackupClassYAML(bcDir, cfg); err != nil {
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

func createBackupClassYAML(bcDir string, cfg *AddBackupClassConfig) error {
	configTypeName := toPascalCase(cfg.Name) + "BackupParameters"
	restoreTypeName := toPascalCase(cfg.Name) + "RestoreParameters"
	pitrTypeName := toPascalCase(cfg.Name) + "PITRParameters"

	class := map[string]any{
		"displayName": toPascalCase(cfg.Name) + " Backup Class",
		"description": "TODO: describe what this BackupClass does.",
		"backupStorageClasses": []any{
			map[string]any{
				"name":      "s3",
				"authModes": []any{"AccessKey"},
				"tls":       map[string]any{"customCA": "None", "insecureSkipVerify": "None"},
			},
		},
		"parametersSchema": map[string]any{
			"openAPIV3Schema": configTypeName,
		},
		"restoreParametersSchema": map[string]any{
			"openAPIV3Schema": restoreTypeName,
		},
	}

	if cfg.ExecutionMode == "ProviderManaged" {
		class["providerManaged"] = map[string]any{
			"supportsPITR": false,
			// Limits are commented hints in the YAML header; leave the map
			// minimal so developers opt into specific caps by uncommenting.
			"limits": map[string]any{},
			"pitrParametersSchema": map[string]any{
				"openAPIV3Schema": pitrTypeName,
			},
		}
	}

	header := fmt.Sprintf(`# %s BackupClass definition.
# This file is the source of truth for the generated BackupClass manifest.
#
# provider-sdk generate makes the class ProviderManaged and names this
# provider in providerManaged.providerRef: the provider claims and executes it.
#
# Fields:
#   displayName/description: human-readable metadata.
#   backupStorageClasses: the BackupStorageClasses this class can write to. Required.
#     name, authModes: a storage class and the authentication modes the engine supports.
#     parametersSchema.openAPIV3Schema: what the engine supports of the storage
#       class's parameters, when less than all of them. Generate starts from the
#       schema of the storage class in the openeverest version this project
#       pins and applies it: properties narrow parameters (an enum to a subset,
#       a stricter pattern), required adds required ones. A parameter newer than
#       that version is unsupported.
#     tls.customCA / tls.insecureSkipVerify: Supported when the engine honours
#       that setting of BackupStorage.spec.tls, None (the default) otherwise.
#       Pass backupclasses.TLSSupport(<class>, <storage class>) to
#       c.BackupStorageTLS, so the engine and the class agree.
#   providerManaged.supportsPITR: advertise PITR capability to Restore validation.
#   providerManaged.limits: caps the runtime enforces against Instance.spec.backup.
#     maxStorages, maxPITREnabledStorages, maxSchedulesPerStorage — unset means unlimited.
#   providerManaged.pitrParametersSchema.openAPIV3Schema: Go type name (in this
#     package) describing per-storage PITR parameters. Resolved to an OpenAPI
#     schema at generation time.
#   parametersSchema.openAPIV3Schema / restoreParametersSchema.openAPIV3Schema:
#     Go type names describing backup-time and restore-time parameters respectively.
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
#   backup  — on-demand backup modal (fields backed by parametersSchema.openAPIV3Schema).
#   pitr    — per-storage PITR sub-form (fields backed by providerManaged.pitrParametersSchema).
#   restore — restore modal (fields backed by restoreParametersSchema.openAPIV3Schema).
`
	return writeYAMLWithHeader(filepath.Join(bcDir, "ui.yaml"), ui, header)
}

func createBackupClassTypes(bcDir string, cfg *AddBackupClassConfig) error {
	pkgName := toGoIdent(cfg.Name)
	cfgType := toPascalCase(cfg.Name) + "BackupParameters"
	restoreType := toPascalCase(cfg.Name) + "RestoreParameters"
	pitrType := toPascalCase(cfg.Name) + "PITRParameters"

	content := fmt.Sprintf(`// Package %s contains the schema-bearing Go types for the
// %q BackupClass. Each struct here is converted to an OpenAPI
// v3 schema by `+"`provider-sdk generate`"+` and inlined into the generated
// BackupClass manifest.
//
// +k8s:openapi-gen=true
package %s

// %s describes the parameters accepted by Backup CRs that
// target this class (spec.parameters). Add fields the user can set per backup.
type %s struct{}

// %s describes the parameters accepted by Restore CRs that
// target this class (spec.parameters). Add fields the user can set per restore.
type %s struct{}

// %s describes the per-storage PITR parameters exposed to
// Instance.spec.backup.storages[].pitr.parameters. Add fields a provider needs
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
