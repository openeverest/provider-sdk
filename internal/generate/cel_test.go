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

func withRule(rule string) map[string]any {
	return map[string]any{"x-kubernetes-validations": []any{map[string]any{"rule": rule}}}
}

func TestValidateCELRules(t *testing.T) {
	schemas := map[string]any{
		"EngineParameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cacheSize":  withRule(`quantity(self).isGreaterThan(quantity("1Gi"))`),
				"endpoint":   withRule(`isURL(self)`),
				"replicas":   withRule(`self % 2 == 1`),
				"name":       withRule(`self.matches('^[a-z]+$') && size(self) <= 22`),
				"pitrWindow": withRule(`self.lowerAscii() == self`),
			},
		},
	}
	cfg := &AssembledConfig{Topologies: map[string]any{
		"sharded": map[string]any{"components": map[string]any{
			"configServer": map[string]any{"supportedFields": map[string]any{
				"openAPIV3Schema": map[string]any{"properties": map[string]any{
					"replicas": withRule(`self >= 3`),
					"storage":  withRule(`format.dns1123Label().validate(self.storageClass).hasValue()`),
				}},
			}},
		}},
	}}

	issues, err := ValidateCELRules(cfg, schemas)
	require.NoError(t, err)

	got := make([]string, 0, len(issues))
	for _, issue := range issues {
		got = append(got, issue.Source+" "+issue.Path)
	}
	assert.Equal(t, []string{
		"type EngineParameters .cacheSize",
		"type EngineParameters .endpoint",
		"type EngineParameters .pitrWindow",
		`topology sharded: component "configServer" supportedFields .storage`,
	}, got)
}
