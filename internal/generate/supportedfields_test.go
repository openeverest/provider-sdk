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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// configWithDeclaration builds a provider whose "simple" topology declares an
// engine component with the given supportedFields schema.
func configWithDeclaration(t *testing.T, schema string) *AssembledConfig {
	t.Helper()

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(schema), &parsed))

	return &AssembledConfig{
		Components: map[string]any{"engine": map[string]any{"type": "db"}},
		Topologies: map[string]any{
			"simple": map[string]any{
				"components": map[string]any{
					"engine": map[string]any{
						"supportedFields": map[string]any{"openAPIV3Schema": parsed},
					},
				},
			},
		},
	}
}

func TestValidateSupportedFieldsAcceptsASelection(t *testing.T) {
	// image is promoted from an embedded struct, and size is a Quantity:
	// a struct in Go, a string on the wire.
	issues, err := validateSupportedFields(configWithDeclaration(t, `
required: [storage]
properties:
  storage:
    properties:
      size: {type: string}
  resources: {}
  replicas: {type: integer, minimum: 1}
  image: {type: string}
`), []string{shapePackage})

	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestValidateSupportedFieldsRejects(t *testing.T) {
	for name, tc := range map[string]struct{ schema, wantReason string }{
		"a field that does not exist": {
			schema:     "properties:\n  storidge: {}",
			wantReason: "no such field on ComponentSpec",
		},
		"a field that does not exist inside a group": {
			schema:     "properties:\n  storage:\n    properties:\n      sighs: {}",
			wantReason: "no such field on Storage",
		},
		"re-typing a struct as a scalar": {
			schema:     "properties:\n  storage: {type: string}",
			wantReason: `declared as "string" but the field is "object"`,
		},
		"re-typing a scalar": {
			schema:     "properties:\n  replicas: {type: string}",
			wantReason: `declared as "string" but the field is "integer"`,
		},
		"selecting inside a scalar": {
			schema:     "properties:\n  replicas:\n    properties:\n      anything: {}",
			wantReason: "has none to select",
		},
		"requiring something undeclared": {
			schema:     "required: [storage]\nproperties:\n  resources: {}",
			wantReason: `required lists "storage", which the schema does not declare`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			issues, err := validateSupportedFields(configWithDeclaration(t, tc.schema), []string{shapePackage})

			require.NoError(t, err)
			require.Len(t, issues, 1)
			assert.Contains(t, issues[0].Reason, tc.wantReason)
			assert.Equal(t, "engine", issues[0].Component)
		})
	}
}

func TestUIPathsMustBeDeclared(t *testing.T) {
	declaration := map[string]any{
		"openAPIV3Schema": map[string]any{
			"properties": map[string]any{
				"storage":   map[string]any{},
				"resources": map[string]any{"properties": map[string]any{"limits": map[string]any{}}},
			},
		},
	}

	for name, tc := range map[string]struct {
		path     string
		declares bool
		wantErr  bool
	}{
		"a declared field":               {path: "spec.components.engine.storage.size", declares: true},
		"deeper than the declaration":    {path: "spec.components.engine.resources.limits.cpu", declares: true},
		"parameters, governed elsewhere": {path: "spec.components.engine.parameters.configuration", declares: true},
		"not a component path":           {path: "spec.version", declares: true},
		"a component declaring nothing":  {path: "spec.components.engine.replicas", declares: false},
		"an undeclared field":            {path: "spec.components.engine.replicas", declares: true, wantErr: true},
		"undeclared inside a declared group": {
			path: "spec.components.engine.resources.requests.cpu", declares: true, wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			engine := map[string]any{}
			if tc.declares {
				engine["supportedFields"] = declaration
			}
			cfg := configWithPath(tc.path)
			cfg.Topologies = map[string]any{
				"simple": map[string]any{"components": map[string]any{"engine": engine}},
			}

			issues := ValidateUIPathsAreDeclared(cfg)
			if !tc.wantErr {
				assert.Empty(t, issues)
				return
			}
			require.Len(t, issues, 1)
			assert.Equal(t, tc.path, issues[0].Path)
			assert.Contains(t, issues[0].Reason, "not declared in supportedFields")
		})
	}
}
