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

const suspendTestVersions = `
componentTypes:
  mongod:
    versions:
      - version: "8.0.12-4"
        image: percona/psmdb:8.0.12-4
`

func writeProviderYAML(t *testing.T, dir, suspend string) {
	t.Helper()
	providerYAML := "name: test-provider\ncomponents:\n  engine:\n    type: mongod\n" + suspend
	require.NoError(t, os.WriteFile(filepath.Join(dir, "provider.yaml"), []byte(providerYAML), 0o600))
}

func TestAssembleSuspend(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		suspend string
		want    map[string]any
		wantErr string
	}{
		{name: "not declared"},
		{name: "declared", suspend: "suspend: {}\n", want: map[string]any{}},
		{name: "not an object", suspend: "suspend: true\n", wantErr: "suspend must be an object"},
		{name: "unknown keys", suspend: "suspend:\n  auto: true\n", wantErr: "suspend has unknown keys [auto]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := writeDefinition(t, suspendTestVersions)
			writeProviderYAML(t, dir, tc.suspend)

			cfg, err := Assemble(dir)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			spec := buildSpecMap(cfg, nil, nil, nil)
			got, declared := spec["suspend"]
			assert.Equal(t, tc.want != nil, declared)
			if declared {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
