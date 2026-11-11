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

// writeMonitoringDefinition writes a definition directory whose provider.yaml
// declares the component types pxc and haproxy, plus monitoring.yaml unless
// monitoringYAML is nil.
func writeMonitoringDefinition(t *testing.T, monitoringYAML *string) string {
	t.Helper()
	dir := t.TempDir()
	providerYAML := "name: test-provider\ncomponents:\n  engine:\n    type: pxc\n  proxy:\n    type: haproxy\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "provider.yaml"), []byte(providerYAML), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "versions.yaml"), []byte("componentTypes: {}\n"), 0o600))
	if monitoringYAML != nil {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "monitoring.yaml"), []byte(*monitoringYAML), 0o600))
	}
	return dir
}

func TestGenerateEmitsMonitoring(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		monitoringYAML *string
		want           string
	}{
		"operator-native integration plus exporter and native endpoints": {
			monitoringYAML: new(`
integrations:
  pmm:
    agentVersions: ">=3.0.0 <4.0.0"
    authModes: [ServiceAccountToken]
componentTypes:
  pxc:
    kind: mysql
    metrics: Exporter
  haproxy:
    kind: haproxy
    metrics: Native
`),
			want: `
integrations:
  pmm:
    agentVersions: ">=3.0.0 <4.0.0"
    authModes: [ServiceAccountToken]
componentTypes:
  pxc:
    kind: mysql
    metrics: Exporter
  haproxy:
    kind: haproxy
    metrics: Native
`,
		},
		"native endpoints only": {
			monitoringYAML: new(`
componentTypes:
  haproxy:
    kind: haproxy
    metrics: Native
`),
			want: `
componentTypes:
  haproxy:
    kind: haproxy
    metrics: Native
`,
		},
		"metrics omitted is left to the CRD default": {
			monitoringYAML: new(`
componentTypes:
  haproxy:
    kind: haproxy
`),
			want: `
componentTypes:
  haproxy:
    kind: haproxy
`,
		},
		"integration without agentVersions": {
			monitoringYAML: new(`
integrations:
  pmm:
    authModes: [ServiceAccountToken]
`),
			want: `
integrations:
  pmm:
    authModes: [ServiceAccountToken]
`,
		},
		"kind with the dots and underscores of OpenTelemetry db.system.name": {
			monitoringYAML: new(`
componentTypes:
  pxc:
    kind: microsoft.sql_server
    metrics: Exporter
`),
			want: `
componentTypes:
  pxc:
    kind: microsoft.sql_server
    metrics: Exporter
`,
		},
		"no monitoring.yaml": {},
		"monitoring.yaml with every line commented out": {
			monitoringYAML: new(`
# componentTypes:
#   pxc:
#     kind: mysql
#     metrics: None
`),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			chartDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"),
				[]byte("apiVersion: v2\nname: provider-test\nversion: 0.1.0\n"), 0o600))

			require.NoError(t, Run(Options{
				DefinitionDir: writeMonitoringDefinition(t, tt.monitoringYAML),
				ChartDir:      chartDir,
			}))

			data, err := os.ReadFile(filepath.Join(chartDir, "generated", "provider-spec.yaml"))
			require.NoError(t, err)
			var generated struct {
				Spec map[string]any `yaml:"spec"`
			}
			require.NoError(t, yaml.Unmarshal(data, &generated))

			if tt.want == "" {
				assert.NotContains(t, generated.Spec, "monitoring")
				return
			}
			got, err := yaml.Marshal(generated.Spec["monitoring"])
			require.NoError(t, err)
			assert.YAMLEq(t, tt.want, string(got))
		})
	}
}

func TestAssembleRejectsInvalidMonitoring(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		monitoringYAML string
		wantErrs       []string
	}{
		"component type not declared in provider.yaml": {
			monitoringYAML: "componentTypes: {mongod: {kind: mongodb, metrics: Exporter}}",
			wantErrs:       []string{`componentTypes["mongod"]: no component in provider.yaml has type "mongod" (declared types: haproxy, pxc)`},
		},
		"integration key that is not a DNS label": {
			monitoringYAML: "integrations: {PMM_3: {}}",
			wantErrs:       []string{`integrations["PMM_3"]: a lowercase RFC 1123 label must consist of`},
		},
		"file wrapped in a top-level monitoring key": {
			monitoringYAML: "monitoring:\n  integrations: {pmm: {}}\n",
			wantErrs:       []string{`this file is spec.monitoring itself; remove the top-level monitoring key`},
		},
		"metrics outside the enum": {
			monitoringYAML: "componentTypes: {pxc: {kind: mysql, metrics: Prometheus}}",
			wantErrs:       []string{`componentTypes["pxc"].metrics: "Prometheus" is not one of Native, Exporter, None`},
		},
		"connection outside the enum": {
			monitoringYAML: "componentTypes: {pxc: {kind: mysql, connection: Always}}",
			wantErrs:       []string{`componentTypes["pxc"].connection: "Always" is not one of Supported, None`},
		},
		"integration without authModes": {
			monitoringYAML: `integrations: {pmm: {agentVersions: ">=3.0.0 <4.0.0"}}`,
			wantErrs:       []string{`integrations["pmm"].authModes: required`},
		},
		"authModes that are not PascalCase or are repeated": {
			monitoringYAML: `integrations: {pmm: {authModes: [api-key, ServiceAccountToken, ServiceAccountToken]}}`,
			wantErrs: []string{
				`integrations["pmm"].authModes: "api-key" does not match`,
				`integrations["pmm"].authModes: "ServiceAccountToken" is listed twice`,
			},
		},
		"agentVersions that core cannot evaluate": {
			monitoringYAML: `integrations: {pmm: {agentVersions: ">=3.0 <4.0"}}`,
			wantErrs:       []string{`integrations["pmm"].agentVersions: ">=3.0 <4.0" is not a semver range`},
		},
		"kind outside the core pattern": {
			monitoringYAML: "componentTypes: {pxc: {kind: MySQL, metrics: Exporter}}",
			wantErrs:       []string{`componentTypes["pxc"].kind: "MySQL" does not match`},
		},
		"kind missing": {
			monitoringYAML: "componentTypes: {haproxy: {metrics: Native}}",
			wantErrs:       []string{`componentTypes["haproxy"].kind: required`},
		},
		"misspelled field": {
			monitoringYAML: "componentTypes: {pxc: {kind: mysql, metric: Exporter}}",
			wantErrs:       []string{`parsing monitoring.yaml`, `unknown field "metric"`},
		},
		"field outside the contract": {
			monitoringYAML: "componentTypes: {pxc: {kind: mysql, metrics: Exporter, credentialProfiles: [metrics]}}",
			wantErrs:       []string{`parsing monitoring.yaml`, `unknown field "credentialProfiles"`},
		},
		"every problem reported at once": {
			monitoringYAML: "componentTypes: {mongod: {kind: MongoDB}, pxc: {kind: mysql, metrics: Prometheus}}",
			wantErrs: []string{
				`componentTypes["mongod"]: no component in provider.yaml has type "mongod"`,
				`componentTypes["mongod"].kind: "MongoDB" does not match`,
				`componentTypes["pxc"].metrics: "Prometheus" is not one of`,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := Assemble(writeMonitoringDefinition(t, &tt.monitoringYAML))

			require.Error(t, err)
			for _, want := range tt.wantErrs {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}
