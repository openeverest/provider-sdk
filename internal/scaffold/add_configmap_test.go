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
	"go/format"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddConfigMap(t *testing.T) {
	tests := map[string]struct {
		name               string
		expectedPkg        string
		expectedSecretType string
		err                string
	}{
		"simple": {
			name:               "app-config",
			expectedPkg:        "package appconfig",
			expectedSecretType: "AppConfigConfigMapData",
		},
		"empty name": {
			name: "",
			err:  "name is required",
		},
		"leading digit": {
			name: "2config",
			err:  `invalid configmap name for Kubernetes resource: invalid name "2config"`,
		},
		"camelCase": {
			name: "appConfig",
			err:  `invalid configmap name for Kubernetes resource: invalid name "appConfig"`,
		},
		"symbol": {
			name: "app-config!",
			err:  `invalid configmap name for Kubernetes resource: invalid name "app-config!"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			projectDir := setupMinimalProvider(t)

			cfg := &AddConfigMapConfig{
				Name: tt.name,
			}
			err := AddConfigMap(cfg)

			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)

				return
			}

			require.NoError(t, err)

			// Verify definition.yaml content.
			defFile := filepath.Join(projectDir, "definition", "configmaps", tt.name, "definition.yaml")
			defYAML, err := os.ReadFile(defFile)
			require.NoError(t, err)

			expected := fmt.Sprintf(`
parametersSchema:
  openAPIV3Schema: %s`, tt.expectedSecretType)
			assert.YAMLEq(t, string(defYAML), expected)

			// Verify ui.yaml exists.
			uiFile := filepath.Join(projectDir, "definition", "configmaps", tt.name, "ui.yaml")
			_, err = os.Stat(uiFile)
			assert.NoError(t, err, "ui.yaml not created")

			// Verify types.go content.
			typesFile := filepath.Join(projectDir, "definition", "configmaps", tt.name, "types.go")
			typesContent, err := os.ReadFile(typesFile)
			require.NoError(t, err)
			assert.Contains(t, string(typesContent), tt.expectedPkg)
			assert.Contains(t, string(typesContent), tt.expectedSecretType)

			// check that the generated types.go is valid Go code
			_, err = format.Source(typesContent)
			assert.NoError(t, err, "invalid types.go"+string(typesContent))

			// Verify that running again fails (directory exists).
			err = AddConfigMap(cfg)
			assert.Error(t, err)
		})
	}
}

func TestAddConfigMap_NotInProviderProject(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	cfg := &AddConfigMapConfig{
		Name: "test-config",
	}
	err := AddConfigMap(cfg)
	assert.Error(t, err, "AddConfigMap() should fail when not in provider project")
}
