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
