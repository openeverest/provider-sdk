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

package generate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// BackupClassAssembled is the assembled form of a single
// definition/backupclasses/<name>/ directory before type-reference resolution.
type BackupClassAssembled struct {
	// Name is the directory name; it doubles as the BackupClass metadata.name.
	Name string
	// Class is the raw map parsed from class.yaml. Type-name references under
	// known string fields (parametersSchema.openAPIV3Schema,
	// restoreParametersSchema.openAPIV3Schema,
	// providerManaged.pitrParametersSchema.openAPIV3Schema) are collected into TypeRefs.
	Class map[string]any
	// UI is the raw map parsed from ui.yaml (or nil when absent). Inlined
	// verbatim under spec.uiSchema in the rendered manifest.
	UI map[string]any
}

// AssembleBackupClasses reads definition/backupclasses/*/{class.yaml,ui.yaml}
// and returns one entry per subdirectory. Type-name references (the
// openAPIV3Schema strings of parametersSchema, restoreParametersSchema, and
// providerManaged.pitrParametersSchema) are collected into typeRefs for later
// schema resolution.
//
// Missing definition/backupclasses/ directory is not an error; it just means
// the provider has no BackupClasses to emit.
func AssembleBackupClasses(defDir string, typeRefs map[string]bool) ([]BackupClassAssembled, error) {
	root := filepath.Join(defDir, "backupclasses")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading backupclasses directory: %w", err)
	}

	var out []BackupClassAssembled
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		classFile := filepath.Join(root, name, "class.yaml")
		if _, err := os.Stat(classFile); err != nil {
			continue // skip directories without a class.yaml
		}
		classData, err := readYAML(classFile)
		if err != nil {
			return nil, fmt.Errorf("reading backupclass %s: %w", name, err)
		}
		if err := validateBackupClass(classData); err != nil {
			return nil, fmt.Errorf("backupclass %s: %w", name, err)
		}
		collectBackupClassTypeRefs(classData, typeRefs)

		var uiData map[string]any
		uiFile := filepath.Join(root, name, "ui.yaml")
		if _, err := os.Stat(uiFile); err == nil {
			uiData, err = readYAML(uiFile)
			if err != nil {
				return nil, fmt.Errorf("reading backupclass %s ui.yaml: %w", name, err)
			}
		}

		out = append(out, BackupClassAssembled{
			Name:  name,
			Class: classData,
			UI:    uiData,
		})
	}

	// Sort for deterministic output across filesystems.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// validateBackupClass rejects a class.yaml generate cannot render: one
// without backupStorageClasses, or one that sets what generate writes.
func validateBackupClass(class map[string]any) error {
	if _, ok := class["supportedProviders"]; ok {
		return errors.New("class.yaml must not list supportedProviders: generate names the provider in providerManaged.providerRef")
	}
	if mode, ok := class["executionMode"]; ok && mode != "ProviderManaged" {
		return fmt.Errorf("executionMode %v is not supported: a provider's classes are ProviderManaged", mode)
	}
	if pm, ok := class["providerManaged"].(map[string]any); ok {
		if _, ok := pm["providerRef"]; ok {
			return errors.New("class.yaml must not set providerManaged.providerRef: generate sets it to the provider")
		}
	}
	storages, _ := class["backupStorageClasses"].([]any)
	if len(storages) == 0 {
		return errors.New("class.yaml must list the backupStorageClasses it can write to")
	}
	for i, s := range storages {
		entry, ok := s.(map[string]any)
		if !ok {
			return fmt.Errorf("backupStorageClasses[%d] must be a map", i)
		}
		tls, ok := entry["tls"]
		if !ok {
			continue
		}
		settings, ok := tls.(map[string]any)
		if !ok {
			return fmt.Errorf("backupStorageClasses[%d].tls must be a map", i)
		}
		for key, val := range settings {
			if key != "customCA" && key != "insecureSkipVerify" {
				return fmt.Errorf("backupStorageClasses[%d].tls.%s is not a TLS setting: write customCA or insecureSkipVerify", i, key)
			}
			if val != "Supported" && val != "None" {
				return fmt.Errorf("backupStorageClasses[%d].tls.%s must be Supported or None, not %v", i, key, val)
			}
		}
	}
	return nil
}

// collectBackupClassTypeRefs scans the known string fields that may carry a
// Go type-name reference and records them so ResolveSchemas can pick them up.
func collectBackupClassTypeRefs(class map[string]any, typeRefs map[string]bool) {
	if cfg, ok := class["parametersSchema"].(map[string]any); ok {
		if s, ok := cfg["openAPIV3Schema"].(string); ok && s != "" {
			typeRefs[s] = true
		}
	}
	if cfg, ok := class["restoreParametersSchema"].(map[string]any); ok {
		if s, ok := cfg["openAPIV3Schema"].(string); ok && s != "" {
			typeRefs[s] = true
		}
	}
	if pm, ok := class["providerManaged"].(map[string]any); ok {
		if cfg, ok := pm["pitrParametersSchema"].(map[string]any); ok {
			if s, ok := cfg["openAPIV3Schema"].(string); ok && s != "" {
				typeRefs[s] = true
			}
		}
	}
}

// buildBackupClassManifest renders one BackupClass of provider as a
// top-level Kubernetes manifest map ready to be marshaled to YAML. The class
// is ProviderManaged and names provider in providerManaged.providerRef.
// Type-name references are replaced by the resolved OpenAPI schemas from
// schemas (placeholder strings remain in place when the schema is missing so
// failures surface loudly), and each backupStorageClasses entry of a storage
// class whose contract provider-sdk knows gets that contract's schema,
// narrowed by the entry's parametersSchema.
func buildBackupClassManifest(bc BackupClassAssembled, schemas map[string]any, provider string) (map[string]any, error) {
	spec := resolveBackupClassRefs(bc.Class, schemas)
	spec["executionMode"] = "ProviderManaged"
	pm, _ := spec["providerManaged"].(map[string]any)
	if pm == nil {
		pm = map[string]any{}
	}
	pm["providerRef"] = map[string]any{"name": provider}
	spec["providerManaged"] = pm
	entries, err := narrowBackupStorageClasses(spec["backupStorageClasses"])
	if err != nil {
		return nil, fmt.Errorf("backupclass %s: %w", bc.Name, err)
	}
	spec["backupStorageClasses"] = entries
	if bc.UI != nil {
		spec["uiSchema"] = bc.UI
	}
	return map[string]any{
		"apiVersion": "backup.openeverest.io/v1alpha1",
		"kind":       "BackupClass",
		"metadata":   map[string]any{"name": bc.Name},
		"spec":       spec,
	}, nil
}

// narrowBackupStorageClasses returns the backupStorageClasses entries with
// the parametersSchema of each entry whose storage class provider-sdk knows
// replaced by the contract's schema narrowed by it. Entries of other storage
// classes keep the schema class.yaml writes, if any.
func narrowBackupStorageClasses(v any) ([]any, error) {
	entries, _ := v.([]any)
	out := make([]any, 0, len(entries))
	for _, e := range entries {
		entry := deepCopyMap(e.(map[string]any))
		name, _ := entry["name"].(string)
		contract, ok, err := storageClassContract(name)
		if err != nil {
			return nil, fmt.Errorf("storage class %s: %w", name, err)
		}
		if ok {
			var overlay map[string]any
			if ps, ok := entry["parametersSchema"].(map[string]any); ok {
				overlay, _ = ps["openAPIV3Schema"].(map[string]any)
			}
			schema, err := narrowSchema(contract, overlay)
			if err != nil {
				return nil, fmt.Errorf("storage class %s: parametersSchema.openAPIV3Schema.%w", name, err)
			}
			entry["parametersSchema"] = map[string]any{"openAPIV3Schema": schema}
		}
		out = append(out, entry)
	}
	return out, nil
}

// resolveBackupClassRefs returns a deep copy of the class spec with the known
// string-valued schema references replaced by their resolved OpenAPI schemas.
func resolveBackupClassRefs(class map[string]any, schemas map[string]any) map[string]any {
	out := make(map[string]any, len(class))
	for k, v := range class {
		switch k {
		case "parametersSchema", "restoreParametersSchema":
			out[k] = resolveOpenAPIRef(v, schemas)
		case "providerManaged":
			out[k] = resolveProviderManagedRefs(v, schemas)
		default:
			out[k] = v
		}
	}
	return out
}

func resolveOpenAPIRef(v any, schemas map[string]any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	resolved := make(map[string]any, len(m))
	for k, val := range m {
		if k == "openAPIV3Schema" {
			if typeName, ok := val.(string); ok && schemas != nil {
				if schema, ok := schemas[typeName]; ok {
					resolved[k] = schema
					continue
				}
			}
		}
		resolved[k] = val
	}
	return resolved
}

func resolveProviderManagedRefs(v any, schemas map[string]any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	resolved := make(map[string]any, len(m))
	for k, val := range m {
		if k == "pitrParametersSchema" {
			resolved[k] = resolveOpenAPIRef(val, schemas)
			continue
		}
		resolved[k] = val
	}
	return resolved
}

// writeBackupClassManifests writes each BackupClass of provider as its own
// YAML document under outputDir and returns the documents. Existing files in
// outputDir not corresponding to a current definition are removed so the
// chart directory mirrors the source of truth.
func writeBackupClassManifests(outputDir string, classes []BackupClassAssembled, schemas map[string]any, provider string) ([][]byte, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating backupclasses output directory: %w", err)
	}

	// Build the desired set.
	desired := make(map[string]bool, len(classes))
	docs := make([][]byte, 0, len(classes))
	for _, bc := range classes {
		manifest, err := buildBackupClassManifest(bc, schemas, provider)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(manifest); err != nil {
			return nil, fmt.Errorf("marshaling backupclass %s: %w", bc.Name, err)
		}
		enc.Close()
		docs = append(docs, buf.Bytes())

		header := "# This file is generated by `provider-sdk generate` from definition/ files.\n" +
			"# Do not edit manually. Run `make generate` to update.\n"
		outFile := filepath.Join(outputDir, bc.Name+".yaml")
		if err := os.WriteFile(outFile, []byte(header+buf.String()), 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", outFile, err)
		}
		desired[bc.Name+".yaml"] = true
		fmt.Fprintf(os.Stderr, "Generated: %s\n", outFile)
	}

	// Prune stale files so removing a backupclass directory removes its manifest.
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return docs, nil // best-effort; the writes above already succeeded
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		if !desired[e.Name()] {
			_ = os.Remove(filepath.Join(outputDir, e.Name()))
		}
	}
	return docs, nil
}
