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

func TestAddTopology(t *testing.T) {
	tests := map[string]struct {
		name         string
		expectedType string
		expectedPkg  string
		err          string
	}{
		"simple": {
			name:         "sharded",
			expectedPkg:  "package sharded",
			expectedType: "type ShardedTopologyParameters struct{}",
		},
		"camelCase": {
			name:         "replicaSet",
			expectedPkg:  "package replicaset",
			expectedType: "type ReplicaSetTopologyParameters struct{}",
		},
		"hyphenated": {
			name:         "replica-set",
			expectedPkg:  "package replicaset",
			expectedType: "type ReplicaSetTopologyParameters struct{}",
		},
		"empty name": {
			name: "",
			err:  "invalid topology name for Go identifier: identifier is required",
		},
		"leading digit name": {
			name: "2fa",
			err:  `invalid topology name for Go identifier: invalid identifier "2fa"`,
		},
		"symbol name": {
			name: "mycomp@",
			err:  `invalid topology name for Go identifier: invalid identifier "mycomp@"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tmpDir := setupMinimalProvider(t)

			err := AddTopology(&AddTopologyConfig{
				TopologyName: tt.name,
			})

			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)

				return
			}

			require.NoError(t, err)

			topoDir := filepath.Join(tmpDir, "definition", "topologies", tt.name)

			typesFile := filepath.Join(topoDir, "types.go")
			typesContent, err := os.ReadFile(typesFile)
			require.NoError(t, err)
			assert.Contains(t, string(typesContent), tt.expectedPkg)
			assert.Contains(t, string(typesContent), tt.expectedType)

			// check that the generated types.go is valid Go code
			_, err = format.Source(typesContent)
			assert.NoError(t, err, "invalid types.go"+string(typesContent))
		})
	}
}
