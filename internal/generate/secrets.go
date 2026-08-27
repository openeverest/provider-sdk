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
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// SecretDefinitionAssembled is the assembled form of a single
// definition/secrets/<name>/ directory before type-reference resolution.
type SecretDefinitionAssembled struct {
	// Name is the directory name; it becomes the key in provider.spec.secrets.
	Name string
	// Definition is the raw map parsed from definition.yaml. Type-name references
	// under parametersSchema.openAPIV3Schema are collected into TypeRefs.
	Definition map[string]any
	// UI is the raw map parsed from ui.yaml (or nil when absent). Inlined
	// verbatim under spec.secrets[name].uiSchema in the rendered provider spec.
	UI map[string]any
}

// AssembleSecrets reads definition/secrets/*/{definition.yaml,ui.yaml}
// and returns one entry per subdirectory. Type-name references (the
// openAPIV3Schema string of parametersSchema) are collected into typeRefs for
// later schema resolution.
//
// Missing definition/secrets/ directory is not an error; it just means
// the provider has no Secrets to emit.
func AssembleSecrets(defDir string, typeRefs map[string]bool) ([]SecretDefinitionAssembled, error) {
	root := filepath.Join(defDir, "secrets")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading secrets directory: %w", err)
	}

	var out []SecretDefinitionAssembled
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		defFile := filepath.Join(root, name, "definition.yaml")
		if _, err := os.Stat(defFile); err != nil {
			continue // skip directories without a definition.yaml
		}
		defData, err := readYAML(defFile)
		if err != nil {
			return nil, fmt.Errorf("reading secret %s: %w", name, err)
		}
		collectSecretTypeRefs(defData, typeRefs)

		var uiData map[string]any
		uiFile := filepath.Join(root, name, "ui.yaml")
		if _, err := os.Stat(uiFile); err == nil {
			uiData, err = readYAML(uiFile)
			if err != nil {
				return nil, fmt.Errorf("reading secret %s ui.yaml: %w", name, err)
			}
		}

		out = append(out, SecretDefinitionAssembled{
			Name:       name,
			Definition: defData,
			UI:         uiData,
		})
	}

	// Sort for deterministic output across filesystems.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// collectSecretTypeRefs scans the known string fields that may carry a
// Go type-name reference and records them so ResolveSchemas can pick them up.
func collectSecretTypeRefs(def map[string]any, typeRefs map[string]bool) {
	if cfg, ok := def["parametersSchema"].(map[string]any); ok {
		if s, ok := cfg["openAPIV3Schema"].(string); ok && s != "" {
			typeRefs[s] = true
		}
	}
}

// buildSecretsSpec builds the secrets section of the Provider CR spec.
// It returns a map keyed by secret name, with each value containing the
// resolved parametersSchema and optional uiSchema.
func buildSecretsSpec(secrets []SecretDefinitionAssembled, schemas map[string]any) map[string]any {
	if len(secrets) == 0 {
		return nil
	}

	result := make(map[string]any, len(secrets))
	for _, s := range secrets {
		entry := make(map[string]any)

		// Resolve parametersSchema reference.
		if cfg, ok := s.Definition["parametersSchema"].(map[string]any); ok {
			if typeName, ok := cfg["openAPIV3Schema"].(string); ok && schemas != nil {
				if schema, ok := schemas[typeName]; ok {
					entry["parametersSchema"] = map[string]any{"openAPIV3Schema": schema}
				}
			}
		}

		// Include UI schema if present.
		if s.UI != nil {
			entry["uiSchema"] = s.UI
		}

		result[s.Name] = entry
	}

	return result
}
