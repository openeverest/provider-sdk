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

// writeKindsDefinition writes a definition directory whose provider.yaml maps
// engine to pxc and proxy to haproxy and ends with providerTail, and whose
// versions.yaml is versionsYAML.
func writeKindsDefinition(t *testing.T, providerTail, versionsYAML string) string {
	t.Helper()
	dir := t.TempDir()
	providerYAML := "name: test-provider\ncomponents:\n  engine:\n    type: pxc\n  proxy:\n    type: haproxy\n" + providerTail
	require.NoError(t, os.WriteFile(filepath.Join(dir, "provider.yaml"), []byte(providerYAML), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "versions.yaml"), []byte(versionsYAML), 0o600))
	return dir
}

const pxcVersions = `
componentTypes:
  pxc:
    defaultVersion: 8.0.42
    versions:
    - version: 8.0.42
      image: percona/percona-xtradb-cluster:8.0.42
`

func TestGenerateEmitsComponentTypeKinds(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		providerTail string
		want         string
	}{
		"kind next to the version catalog and for a type without one": {
			providerTail: "componentTypes:\n  pxc:\n    kind: mysql\n  haproxy:\n    kind: haproxy\n",
			want: `
pxc:
  kind: mysql
  defaultVersion: 8.0.42
  versions:
  - version: 8.0.42
    image: percona/percona-xtradb-cluster:8.0.42
haproxy:
  kind: haproxy
`,
		},
		"kind with the dots and underscores of OpenTelemetry db.system.name": {
			providerTail: "componentTypes:\n  pxc:\n    kind: microsoft.sql_server\n",
			want: `
pxc:
  kind: microsoft.sql_server
  defaultVersion: 8.0.42
  versions:
  - version: 8.0.42
    image: percona/percona-xtradb-cluster:8.0.42
`,
		},
		"domain-qualified kind": {
			providerTail: "componentTypes:\n  haproxy:\n    kind: example.com/balancer\n",
			want: `
pxc:
  defaultVersion: 8.0.42
  versions:
  - version: 8.0.42
    image: percona/percona-xtradb-cluster:8.0.42
haproxy:
  kind: example.com/balancer
`,
		},
		"no kinds": {
			want: `
pxc:
  defaultVersion: 8.0.42
  versions:
  - version: 8.0.42
    image: percona/percona-xtradb-cluster:8.0.42
`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			chartDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"),
				[]byte("apiVersion: v2\nname: provider-test\nversion: 0.1.0\n"), 0o600))

			require.NoError(t, Run(Options{
				DefinitionDir: writeKindsDefinition(t, tt.providerTail, pxcVersions),
				ChartDir:      chartDir,
			}))

			data, err := os.ReadFile(filepath.Join(chartDir, "generated", "provider-spec.yaml"))
			require.NoError(t, err)
			var generated struct {
				Spec map[string]any `yaml:"spec"`
			}
			require.NoError(t, yaml.Unmarshal(data, &generated))
			got, err := yaml.Marshal(generated.Spec["componentTypes"])
			require.NoError(t, err)
			assert.YAMLEq(t, tt.want, string(got))
		})
	}
}

func TestAssembleRejectsInvalidComponentTypeKinds(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		providerTail string
		versionsYAML string
		wantErrs     []string
	}{
		"kind outside the core pattern": {
			providerTail: "componentTypes:\n  pxc:\n    kind: MySQL\n",
			wantErrs:     []string{`invalid provider.yaml: componentTypes["pxc"].kind: "MySQL" does not match`},
		},
		"kind missing": {
			providerTail: "componentTypes:\n  pxc: {}\n",
			wantErrs:     []string{`componentTypes["pxc"].kind: required`},
		},
		"entry that is not a map": {
			providerTail: "componentTypes:\n  pxc: mysql\n",
			wantErrs:     []string{`componentTypes["pxc"]: must be a map with a kind`},
		},
		"component type not used by any component": {
			providerTail: "componentTypes:\n  mongod:\n    kind: mongodb\n",
			wantErrs:     []string{`componentTypes["mongod"]: no component in provider.yaml has type "mongod" (declared types: haproxy, pxc)`},
		},
		"version catalog in provider.yaml": {
			providerTail: "componentTypes:\n  pxc:\n    kind: mysql\n    defaultVersion: 8.0.42\n",
			wantErrs:     []string{`componentTypes["pxc"].defaultVersion: unknown field; version catalogs belong in versions.yaml`},
		},
		"field outside the declaration": {
			providerTail: "componentTypes:\n  pxc:\n    kind: mysql\n    protocol: mysql\n",
			wantErrs:     []string{`componentTypes["pxc"].protocol: unknown field; provider.yaml componentTypes declare only kind`},
		},
		"kind in versions.yaml": {
			versionsYAML: "componentTypes:\n  pxc:\n    kind: mysql\n    defaultVersion: 8.0.42\n    versions:\n    - version: 8.0.42\n      image: percona/percona-xtradb-cluster:8.0.42\n",
			wantErrs:     []string{`invalid versions.yaml: componentTypes["pxc"].kind: declare the kind in provider.yaml (componentTypes.pxc.kind), not in versions.yaml`},
		},
		"every problem reported at once": {
			providerTail: "componentTypes:\n  mongod:\n    kind: MongoDB\n  pxc: {}\n",
			wantErrs: []string{
				`componentTypes["mongod"]: no component in provider.yaml has type "mongod"`,
				`componentTypes["mongod"].kind: "MongoDB" does not match`,
				`componentTypes["pxc"].kind: required`,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			versionsYAML := tt.versionsYAML
			if versionsYAML == "" {
				versionsYAML = pxcVersions
			}
			_, err := Assemble(writeKindsDefinition(t, tt.providerTail, versionsYAML))

			require.Error(t, err)
			for _, want := range tt.wantErrs {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}
