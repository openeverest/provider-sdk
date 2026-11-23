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
	"gopkg.in/yaml.v3"
)

const stampKey = "openeverest.io/contract-version"

func TestSetChartAnnotation(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   string
		want string
		// wantParsed is checked instead of want when re-encoding may change
		// the layout.
		wantParsed map[string]any
		wantErr    string
	}{
		"no annotations": {
			in: `apiVersion: v2
name: provider-x
version: 0.1.0
appVersion: "0.1.0"
keywords:
  - openeverest

# TODO: dependencies
# dependencies:
#   - name: op
`,
			want: `apiVersion: v2
name: provider-x
version: 0.1.0
appVersion: "0.1.0"
keywords:
  - openeverest

# TODO: dependencies
# dependencies:
#   - name: op
annotations:
  openeverest.io/contract-version: "2.0.0"
`,
		},
		"no trailing newline": {
			in:   "apiVersion: v2\nname: provider-x\nversion: 0.1.0",
			want: "apiVersion: v2\nname: provider-x\nversion: 0.1.0\nannotations:\n  openeverest.io/contract-version: \"2.0.0\"\n",
		},
		"other annotations, comments and blank lines": {
			in: `apiVersion: v2
name: provider-x
annotations: # chart metadata

  # Artifact Hub category.
  artifacthub.io/category: database
    
  example.com/team: "dbs"
version: 0.1.0
`,
			want: `apiVersion: v2
name: provider-x
annotations: # chart metadata
  openeverest.io/contract-version: "2.0.0"

  # Artifact Hub category.
  artifacthub.io/category: database
    
  example.com/team: "dbs"
version: 0.1.0
`,
		},
		"four-space indent": {
			in:   "name: provider-x\nannotations:\n    example.com/team: dbs\n",
			want: "name: provider-x\nannotations:\n    openeverest.io/contract-version: \"2.0.0\"\n    example.com/team: dbs\n",
		},
		"plain stamp with a comment": {
			in:   "name: provider-x\nannotations:\n  a: b\n  openeverest.io/contract-version: 2.0.0-alpha.1 # stamped\n  c: d\n",
			want: "name: provider-x\nannotations:\n  a: b\n  openeverest.io/contract-version: \"2.0.0\" # stamped\n  c: d\n",
		},
		"double-quoted stamp": {
			in:   "name: provider-x\nannotations:\n  openeverest.io/contract-version: \"2.0.0-alpha.1\"\nversion: 0.1.0\n",
			want: "name: provider-x\nannotations:\n  openeverest.io/contract-version: \"2.0.0\"\nversion: 0.1.0\n",
		},
		"single-quoted stamp": {
			in:   "name: provider-x\nannotations:\n  'openeverest.io/contract-version': 'it''s 1'\n",
			want: "name: provider-x\nannotations:\n  'openeverest.io/contract-version': \"2.0.0\"\n",
		},
		"stamp unchanged": {
			in:   "name: provider-x\nannotations:\n  openeverest.io/contract-version: 2.0.0 # kept as written\n",
			want: "name: provider-x\nannotations:\n  openeverest.io/contract-version: 2.0.0 # kept as written\n",
		},
		"flow annotations": {
			in: "name: provider-x\nannotations: {example.com/team: dbs}\nversion: 0.1.0\n",
			wantParsed: map[string]any{
				"name":        "provider-x",
				"annotations": map[string]any{"example.com/team": "dbs", stampKey: "2.0.0"},
				"version":     "0.1.0",
			},
		},
		"empty annotations": {
			in: "name: provider-x\nannotations:\nversion: 0.1.0\n",
			wantParsed: map[string]any{
				"name":        "provider-x",
				"annotations": map[string]any{stampKey: "2.0.0"},
				"version":     "0.1.0",
			},
		},
		"multi-line stamp": {
			in: "name: provider-x\nannotations:\n  openeverest.io/contract-version: >-\n    2.0.0-alpha.1\n",
			wantParsed: map[string]any{
				"name":        "provider-x",
				"annotations": map[string]any{stampKey: "2.0.0"},
			},
		},
		"not a mapping": {
			in:      "- name: provider-x\n",
			wantErr: "not a YAML mapping",
		},
		"annotations not a mapping": {
			in:      "name: provider-x\nannotations: [a, b]\n",
			wantErr: "annotations is not a mapping",
		},
		"invalid YAML": {
			in:      "name: [provider-x\n",
			wantErr: "parsing",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := setChartAnnotation([]byte(tt.in), stampKey, "2.0.0")

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.wantParsed != nil {
				var parsed map[string]any
				require.NoError(t, yaml.Unmarshal(got, &parsed))
				assert.Equal(t, tt.wantParsed, parsed)
				return
			}
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestStampChart(t *testing.T) {
	t.Parallel()

	chartDir := t.TempDir()
	chartFile := filepath.Join(chartDir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartFile, []byte("apiVersion: v2\nname: provider-x\nversion: 0.1.0\n"), 0o600))

	require.NoError(t, stampChart(chartDir, "2.0.0-alpha.1"))

	chart, err := os.ReadFile(chartFile)
	require.NoError(t, err)
	assert.Equal(t, "apiVersion: v2\nname: provider-x\nversion: 0.1.0\nannotations:\n  openeverest.io/contract-version: \"2.0.0-alpha.1\"\n", string(chart))
	helper, err := os.ReadFile(filepath.Join(chartDir, "templates", ContractHelperFile))
	require.NoError(t, err)
	assert.Equal(t, ContractHelper, string(helper))

	require.NoError(t, stampChart(chartDir, "2.0.0-alpha.2"))

	chart, err = os.ReadFile(chartFile)
	require.NoError(t, err)
	assert.Equal(t, "apiVersion: v2\nname: provider-x\nversion: 0.1.0\nannotations:\n  openeverest.io/contract-version: \"2.0.0-alpha.2\"\n", string(chart))
}

func TestRunStampsTheChart(t *testing.T) {
	t.Parallel()

	chartDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"),
		[]byte("apiVersion: v2\nname: provider-test\nversion: 0.1.0\nappVersion: \"0.1.0\"\n"), 0o600))
	defDir := writeDefinition(t, `
componentTypes:
  mongod:
    versions:
      - version: "8.0.12-4"
        image: percona/psmdb:8.0.12-4
`)
	writeFiles(t, filepath.Join(defDir, "backupclasses", "pbm"), map[string]string{
		"class.yaml": "backupStorageClasses:\n  - name: s3\n    authModes: [AccessKey]\n",
	})

	require.NoError(t, Run(Options{DefinitionDir: defDir, ChartDir: chartDir, ContractVersion: "2.0.0-alpha.2"}))

	var chart struct {
		Annotations map[string]string `yaml:"annotations"`
	}
	data, err := os.ReadFile(filepath.Join(chartDir, "Chart.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &chart))
	assert.Equal(t, "2.0.0-alpha.2", chart.Annotations[stampKey])

	var class struct {
		Metadata struct {
			Annotations map[string]string `yaml:"annotations"`
		} `yaml:"metadata"`
	}
	data, err = os.ReadFile(filepath.Join(chartDir, "generated", "backupclasses", "pbm.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &class))
	assert.Equal(t, "2.0.0-alpha.2", class.Metadata.Annotations[stampKey])
	assert.Equal(t, "0.1.0", class.Metadata.Annotations["openeverest.io/provider-version"])

	assert.FileExists(t, filepath.Join(chartDir, "templates", ContractHelperFile))
}
