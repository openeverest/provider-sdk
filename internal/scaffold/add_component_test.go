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

// setupMinimalProvider creates a minimal provider project structure in a temp
// directory and changes into it, returning the project directory path.
func setupMinimalProvider(t *testing.T) string {
	t.Helper()

	cfg := &Config{
		ProviderName: "test",
		ModulePath:   "github.com/example/test",
	}

	tmpDir := filepath.Join(t.TempDir(), "test")
	err := Scaffold(cfg, tmpDir)
	require.NoError(t, err)

	t.Chdir(tmpDir)

	return tmpDir
}

func TestAddComponent(t *testing.T) {
	tests := map[string]struct {
		name             string
		componentType    string
		expectedType     string
		expectedSpecName string
		expectedSpecType string
		err              string
	}{
		"simple": {
			name:             "engine",
			componentType:    "mongod",
			expectedType:     "type MongodParameters struct{}",
			expectedSpecName: `ComponentEngine = "engine"`,
			expectedSpecType: `ComponentTypeMongod = "mongod"`,
		},
		"camelCase": {
			name:             "backupAgent",
			componentType:    "backup",
			expectedType:     "type BackupParameters struct{}",
			expectedSpecName: `ComponentBackupAgent = "backupAgent"`,
			expectedSpecType: `ComponentTypeBackup = "backup"`,
		},
		"hyphenated": {
			name:             "my-comp",
			componentType:    "my-type",
			expectedType:     `type MyTypeParameters struct{}`,
			expectedSpecName: `ComponentMyComp = "my-comp"`,
			expectedSpecType: `ComponentTypeMyType = "my-type"`,
		},
		"empty name": {
			name:          "",
			componentType: "myType",
			err:           "invalid component name for Go: identifier is required",
		},
		"leading digit name": {
			name:          "2fa",
			componentType: "myType",
			err:           `invalid component name for Go: invalid identifier "2fa"`,
		},
		"symbol name": {
			name:          "mycomp@",
			componentType: "myType",
			err:           `invalid component name for Go: invalid identifier "mycomp@"`,
		},
		"empty type": {
			name:          "comp",
			componentType: "",
			err:           "invalid component type for Go: identifier is required",
		},
		"leading digit type": {
			name:          "comp",
			componentType: "2fa",
			err:           `invalid component type for Go: invalid identifier "2fa"`,
		},
		"symbol type": {
			name:          "comp",
			componentType: "mytype@",
			err:           `invalid component type for Go: invalid identifier "mytype@"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tmpDir := setupMinimalProvider(t)

			err := AddComponent(&AddComponentConfig{
				ComponentName: tt.name,
				ComponentType: tt.componentType,
			})
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)

				return
			}

			require.NoError(t, err)

			typesFile := filepath.Join(tmpDir, "definition", "components", "types.go")
			typesContent, err := os.ReadFile(typesFile)
			require.NoError(t, err)
			assert.Contains(t, string(typesContent), tt.expectedType)

			// check that the generated types.go is valid Go code
			_, err = format.Source(typesContent)
			assert.NoError(t, err)

			specFile := filepath.Join(tmpDir, "internal", "common", "spec.go")
			specContent, err := os.ReadFile(specFile)
			require.NoError(t, err)
			assert.Contains(t, string(specContent), tt.expectedSpecName)
			assert.Contains(t, string(specContent), tt.expectedSpecType)

			// check that the generated spec.go is valid Go code
			_, err = format.Source(specContent)
			assert.NoError(t, err)

			// check provider.yaml has the component name and type
			providerYAML, err := os.ReadFile(filepath.Join(tmpDir, "definition", "provider.yaml"))
			require.NoError(t, err)

			expected := fmt.Sprintf(`
name: test
components:
  %s:
    type: %s`, tt.name, tt.componentType)
			assert.YAMLEq(t, expected, string(providerYAML))
		})
	}
}
