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

func TestAssembleParsesDefaultVersion(t *testing.T) {
	t.Parallel()

	cfg, err := Assemble(writeDefinition(t, `
componentTypes:
  mongod:
    defaultVersion: "8.0.12-4"
    versions:
      - version: "8.0.12-4"
        image: percona/psmdb:8.0.12-4
defaultVersion: "8.0"
versions:
  - name: "8.0"
    components:
      engine: "8.0.12-4"
`))

	require.NoError(t, err)
	assert.Equal(t, "8.0", cfg.DefaultVersion)

	spec := buildSpecMap(cfg, nil, nil, nil)
	assert.Equal(t, "8.0", spec["defaultVersion"])
}

func TestAssembleRejectsUnknownDefaultVersionBundle(t *testing.T) {
	t.Parallel()

	_, err := Assemble(writeDefinition(t, `
componentTypes:
  mongod:
    versions:
      - version: "8.0.12-4"
        image: percona/psmdb:8.0.12-4
defaultVersion: "9.0"
versions:
  - name: "8.0"
    components:
      engine: "8.0.12-4"
`))

	require.ErrorContains(t, err, `defaultVersion "9.0" not found in versions`)
}

func TestAssembleRejectsUnknownComponentTypeDefaultVersion(t *testing.T) {
	t.Parallel()

	_, err := Assemble(writeDefinition(t, `
componentTypes:
  mongod:
    defaultVersion: "9.9.9"
    versions:
      - version: "8.0.12-4"
        image: percona/psmdb:8.0.12-4
`))

	require.ErrorContains(t, err, `componentTypes["mongod"]: defaultVersion "9.9.9" not found in versions`)
}

func TestAssembleRejectsRemovedPerEntryDefaultFlag(t *testing.T) {
	t.Parallel()

	_, err := Assemble(writeDefinition(t, `
componentTypes:
  mongod:
    versions:
      - version: "8.0.12-4"
        image: percona/psmdb:8.0.12-4
        default: true
`))
	require.ErrorContains(t, err, "per-entry `default` was removed")

	_, err = Assemble(writeDefinition(t, `
componentTypes:
  mongod:
    versions:
      - version: "8.0.12-4"
        image: percona/psmdb:8.0.12-4
versions:
  - name: "8.0"
    default: true
    components:
      engine: "8.0.12-4"
`))
	require.ErrorContains(t, err, "per-entry `default` was removed")
}

func TestBuildTopologySpecSelectsCRFields(t *testing.T) {
	t.Parallel()

	supportedFields := map[string]any{
		"required": []any{"storage"},
		"properties": map[string]any{
			"storage": map[string]any{},
			"schedulingPolicy": map[string]any{
				"properties": map[string]any{
					"nodeSelector": map[string]any{},
				},
			},
		},
	}

	spec := buildTopologySpec(map[string]any{
		"components": map[string]any{
			"engine": map[string]any{
				"optional":        false,
				"defaults":        map[string]any{"replicas": 3},
				"supportedFields": supportedFields,
			},
		},
	}, map[string]bool{})

	components, ok := spec["components"].(map[string]any)
	require.True(t, ok)
	engine, ok := components["engine"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, map[string]any{"openAPIV3Schema": supportedFields}, engine["supportedFields"],
		"the inline schema is wrapped in the envelope the CR field expects, nesting intact")
	assert.Contains(t, engine, "optional")
	assert.NotContains(t, engine, "defaults", "defaults is an authoring aid, not a CR field")
}

func TestBuildTopologySpecOmitsUndeclaredSupportedFields(t *testing.T) {
	t.Parallel()

	spec := buildTopologySpec(map[string]any{
		"components": map[string]any{
			"engine": map[string]any{"optional": true},
		},
	}, map[string]bool{})

	engine := spec["components"].(map[string]any)["engine"].(map[string]any)
	assert.NotContains(t, engine, "supportedFields",
		"an absent declaration must stay absent, since it means unconstrained")
}
