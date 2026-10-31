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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssembleBackupClasses_BackupStorageClasses(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		class string
		err   string
	}{
		"declared": {
			class: "executionMode: ProviderManaged\nbackupStorageClasses:\n  - name: s3\n    authModes: [AccessKey]\n",
		},
		"missing": {
			class: "executionMode: ProviderManaged\n",
			err:   "backupclass pbm: class.yaml must list the backupStorageClasses it can write to",
		},
		"empty": {
			class: "executionMode: ProviderManaged\nbackupStorageClasses: []\n",
			err:   "backupclass pbm: class.yaml must list the backupStorageClasses it can write to",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			defDir := t.TempDir()
			classDir := filepath.Join(defDir, "backupclasses", "pbm")
			require.NoError(t, os.MkdirAll(classDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(classDir, "class.yaml"), []byte(tt.class), 0o600))

			classes, err := AssembleBackupClasses(defDir, map[string]bool{})
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Len(t, classes, 1)
			assert.Equal(t, "pbm", classes[0].Name)
		})
	}
}
