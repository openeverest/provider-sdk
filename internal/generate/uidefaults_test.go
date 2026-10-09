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
)

func versionPicker(fieldParams map[string]any) *AssembledConfig {
	return &AssembledConfig{
		DefaultVersion: "8.0",
		ComponentTypes: map[string]any{
			"mongod": map[string]any{"defaultVersion": "8.0.12-4"},
		},
		UISchema: map[string]any{
			"standalone": map[string]any{
				"sections": map[string]any{
					"databaseVersion": map[string]any{
						"components": map[string]any{
							"version": map[string]any{
								"uiType":      "select",
								"path":        "spec.version",
								"fieldParams": fieldParams,
							},
						},
					},
				},
			},
		},
	}
}

func fieldParamsOf(cfg *AssembledConfig) map[string]any {
	return cfg.UISchema["standalone"].(map[string]any)["sections"].(map[string]any)["databaseVersion"].(map[string]any)["components"].(map[string]any)["version"].(map[string]any)["fieldParams"].(map[string]any)
}

func TestInjectDefaultValuePathForBundlePicker(t *testing.T) {
	t.Parallel()

	cfg := versionPicker(map[string]any{
		"optionsPath":       "spec.versions",
		"optionsPathConfig": map[string]any{"labelPath": "name", "valuePath": "name"},
	})
	InjectDefaultValuePaths(cfg)

	assert.Equal(t, map[string]any{
		"labelPath":        "name",
		"valuePath":        "name",
		"defaultValuePath": "spec.defaultVersion",
	}, fieldParamsOf(cfg)["optionsPathConfig"])
}

func TestInjectDefaultValuePathForComponentTypePicker(t *testing.T) {
	t.Parallel()

	cfg := versionPicker(map[string]any{
		"optionsPath": "spec.componentTypes.mongod.versions",
	})
	InjectDefaultValuePaths(cfg)

	assert.Equal(t, map[string]any{
		"defaultValuePath": "spec.componentTypes.mongod.defaultVersion",
	}, fieldParamsOf(cfg)["optionsPathConfig"])
}

func TestInjectDefaultValuePathKeepsAuthoredValue(t *testing.T) {
	t.Parallel()

	cfg := versionPicker(map[string]any{
		"optionsPath":       "spec.versions",
		"optionsPathConfig": map[string]any{"defaultValuePath": "spec.custom"},
	})
	InjectDefaultValuePaths(cfg)

	assert.Equal(t, "spec.custom", fieldParamsOf(cfg)["optionsPathConfig"].(map[string]any)["defaultValuePath"])
}

func TestInjectDefaultValuePathSkipsWhenNoDefaultDeclared(t *testing.T) {
	t.Parallel()

	cfg := versionPicker(map[string]any{"optionsPath": "spec.versions"})
	cfg.DefaultVersion = ""
	InjectDefaultValuePaths(cfg)

	assert.NotContains(t, fieldParamsOf(cfg), "optionsPathConfig")
}

func TestInjectDefaultValuePathIgnoresOtherLists(t *testing.T) {
	t.Parallel()

	for _, optionsPath := range []string{
		"spec.storageClasses",
		"spec.componentTypes.unknown.versions",
		"spec.componentTypes.mongod.versions.nested",
		"spec.componentTypes..versions",
	} {
		cfg := versionPicker(map[string]any{"optionsPath": optionsPath})
		InjectDefaultValuePaths(cfg)
		assert.NotContains(t, fieldParamsOf(cfg), "optionsPathConfig", optionsPath)
	}
}
