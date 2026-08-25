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
)

const shapePackage = "./testdata/instanceshape"

// configWithPath builds a provider whose "simple" topology declares an engine
// component and renders a single field at the given path.
func configWithPath(path string) *AssembledConfig {
	return configWithUISchema(map[string]any{
		"sections": map[string]any{
			"main": map[string]any{
				"components": map[string]any{
					"field": map[string]any{"path": path, "uiType": "text"},
				},
			},
		},
	})
}

func configWithUISchema(schema map[string]any) *AssembledConfig {
	return &AssembledConfig{
		Components: map[string]any{
			"engine":     map[string]any{"type": "db", "parametersSchema": "EngineParameters"},
			"monitoring": map[string]any{"type": "pmm"},
		},
		Topologies: map[string]any{
			"simple": map[string]any{
				"parametersSchema": "TopologyParameters",
				"components":       map[string]any{"engine": map[string]any{}},
			},
		},
		UISchema: map[string]any{"simple": schema},
	}
}

func TestValidateUISchemaPathsResolvable(t *testing.T) {
	for _, path := range []string{
		"spec.version",
		"spec.components.engine.replicas",
		"spec.components.engine.image",
		"spec.components.engine.storage.size",
		"spec.components.engine.resources.limits.cpu",
		"spec.components.engine.parameters.configuration",
		"spec.topology.parameters.numShards",
		"status.somethingElse",
	} {
		t.Run(path, func(t *testing.T) {
			issues, err := validateUISchemaPaths(configWithPath(path), []string{shapePackage})
			require.NoError(t, err)
			assert.Empty(t, issues)
		})
	}
}

func TestValidateUISchemaPathsRejected(t *testing.T) {
	tests := map[string]struct {
		path   string
		reason string
	}{
		"field removed from the API": {
			path:   "spec.components.engine.config",
			reason: `no field "config" on ComponentSpec`,
		},
		"field removed from a nested struct": {
			path:   "spec.components.engine.storage.className",
			reason: `no field "className" on Storage`,
		},
		"component absent from this topology": {
			path:   "spec.components.monitoring.replicas",
			reason: `component "monitoring" is not part of topology "simple" (declared: engine)`,
		},
		"component that does not exist at all": {
			path:   "spec.components.typo.replicas",
			reason: `component "typo" is not part of topology "simple" (declared: engine)`,
		},
		"unknown key in a declared parameters schema": {
			path:   "spec.components.engine.parameters.nope",
			reason: `no field "nope" on EngineParameters`,
		},
		"addressing inside a scalar": {
			path:   "spec.version.major",
			reason: `"version" is a string, cannot address "major" inside it`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			issues, err := validateUISchemaPaths(configWithPath(tt.path), []string{shapePackage})
			require.NoError(t, err)
			require.Len(t, issues, 1)
			assert.Equal(t, "simple", issues[0].Topology)
			assert.Equal(t, "path", issues[0].Source)
			assert.Equal(t, tt.path, issues[0].Path)
			assert.Equal(t, tt.reason, issues[0].Reason)
		})
	}
}

// A parameters payload with no declared schema cannot be checked, and saying so
// is more useful than silently accepting whatever the form writes into it.
func TestValidateUISchemaPathsUndeclaredParametersSchema(t *testing.T) {
	cfg := configWithPath("spec.components.engine.parameters.configuration")
	cfg.Components["engine"] = map[string]any{"type": "db"}

	issues, err := validateUISchemaPaths(cfg, []string{shapePackage})
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t,
		`no parametersSchema declared for "components.engine.parameters", so "configuration" cannot be verified`,
		issues[0].Reason)
}

func TestValidateUISchemaPathsCELExpressions(t *testing.T) {
	cfg := configWithUISchema(map[string]any{
		"sections": map[string]any{
			"main": map[string]any{
				"components": map[string]any{
					"shards": map[string]any{
						"path":   "spec.topology.parameters.numShards",
						"uiType": "number",
						"validation": map[string]any{
							"celExpressions": []any{
								map[string]any{
									"celExpr": "spec.topology.config.numShards >= original.spec.topology.config.numShards",
									"message": "Number of shards cannot be decreased",
								},
							},
						},
					},
				},
			},
		},
	})

	issues, err := validateUISchemaPaths(cfg, []string{shapePackage})
	require.NoError(t, err)
	require.Len(t, issues, 2, "both the plain and the original.-prefixed reference are checked")
	for _, issue := range issues {
		assert.Equal(t, "celExpr", issue.Source)
		assert.Equal(t, "spec.topology.config.numShards", issue.Path)
		assert.Equal(t, `no field "config" on TopologySpec`, issue.Reason)
	}
}

func TestCollectPathRefs(t *testing.T) {
	refs := collectPathRefs(map[string]any{
		"sections": map[string]any{
			"resources": map[string]any{
				"uiType": "group",
				"components": map[string]any{
					"cpu":  map[string]any{"path": "spec.components.engine.resources.limits.cpu"},
					"disk": map[string]any{"path": "spec.components.engine.storage.size"},
				},
			},
			"nodes": map[string]any{
				"path": "spec.components.engine.replicas",
				"validation": map[string]any{
					"celExpressions": []any{
						map[string]any{"celExpr": "spec.components.engine.replicas % 2 == 1"},
					},
				},
			},
		},
		// Not a field reference: labelPath addresses the options payload.
		"labelPath": "name",
	})

	assert.Equal(t, []pathRef{
		{source: "path", path: "spec.components.engine.replicas"},
		{source: "celExpr", path: "spec.components.engine.replicas"},
		{source: "path", path: "spec.components.engine.resources.limits.cpu"},
		{source: "path", path: "spec.components.engine.storage.size"},
	}, refs)
}
