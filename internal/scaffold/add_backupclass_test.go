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
	"go/format"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddBackupClass(t *testing.T) {
	tests := map[string]struct {
		name                string
		expectedPkg         string
		expectedBackupType  string
		expectedRestoreType string
		expectedPITRType    string

		err string
	}{
		"simple": {
			name:                "percona-server-mongodb",
			expectedPkg:         "package perconaservermongodb",
			expectedBackupType:  "type PerconaServerMongodbBackupParameters struct{}",
			expectedRestoreType: "type PerconaServerMongodbRestoreParameters struct{}",
			expectedPITRType:    "type PerconaServerMongodbPITRParameters struct{}",
		},
		"empty name": {
			name: "",
			err:  "invalid backup class name for Kubernetes resource: name is required",
		},
		"leading digit": {
			name: "2-backup-class",
			err:  `invalid name "2-backup-class"`,
		},
		"camelCase": {
			name: "perconaServerMongodb",
			err:  "invalid backup class name for Kubernetes resource",
		},
		"symbol": {
			name: "perconaServer!",
			err:  `invalid name "perconaServer!"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tmpDir := setupMinimalProvider(t)

			err := AddBackupClass(&AddBackupClassConfig{
				Name:          tt.name,
				ExecutionMode: "ProviderManaged",
			})

			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)

				return
			}

			require.NoError(t, err)

			bcDir := filepath.Join(tmpDir, "definition", "backupclasses", tt.name)

			typesFile := filepath.Join(bcDir, "types.go")
			typesContent, err := os.ReadFile(typesFile)
			require.NoError(t, err)
			assert.Contains(t, string(typesContent), tt.expectedPkg)
			assert.Contains(t, string(typesContent), tt.expectedBackupType)
			assert.Contains(t, string(typesContent), tt.expectedRestoreType)
			assert.Contains(t, string(typesContent), tt.expectedPITRType)

			// check that the generated types.go is valid Go code
			_, err = format.Source(typesContent)
			assert.NoError(t, err, "invalid types.go"+string(typesContent))
		})
	}
}
