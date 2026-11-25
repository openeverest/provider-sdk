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

package verify

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openeverest/provider-sdk/internal/compat"
	"github.com/openeverest/provider-sdk/internal/generate"
	"github.com/openeverest/provider-sdk/internal/scaffold"
)

const (
	released    = "2.0.0-alpha.1"
	development = "2.0.0-alpha.1.next"
)

var releasedCore = compat.Module{Path: compat.CoreModule, Version: "v2.0.0-alpha.1"}

// stampedChart is a chart whose Provider and BackupClass carry the stamp
// released, the way generate and the scaffolded templates write them.
func stampedChart() map[string]string {
	return map[string]string{
		"Chart.yaml": `apiVersion: v2
name: provider-x
version: 0.1.0
annotations:
  openeverest.io/contract-version: "` + released + `"
`,
		"values.yaml": "provider:\n  name: x\n",
		"templates/" + generate.ContractHelperFile: generate.ContractHelper,
		"templates/provider.yaml": `apiVersion: core.openeverest.io/v1alpha1
kind: Provider
metadata:
  name: {{ .Values.provider.name }}
  annotations:
    {{- include "openeverest.contract.annotations" . | nindent 4 }}
spec: {}
`,
		"templates/backupclasses.yaml": `{{- range $path, $_ := .Files.Glob "generated/backupclasses/*.yaml" }}
---
{{ $.Files.Get $path }}
{{- end }}
`,
		"generated/backupclasses/pbm.yaml": `apiVersion: backup.openeverest.io/v1alpha1
kind: BackupClass
metadata:
  name: pbm
  annotations:
    openeverest.io/contract-version: ` + released + `
spec: {}
`,
	}
}

func with(files map[string]string, changes map[string]string) map[string]string {
	out := maps.Clone(files)
	for name, content := range changes {
		if content == "" {
			delete(out, name)
			continue
		}
		out[name] = content
	}
	return out
}

func writeChart(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	return dir
}

func TestRun(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		files        map[string]string
		version      string
		release      bool
		core         compat.Module
		wantStamped  []string
		wantProblems []string
	}{
		"stamped": {
			files: stampedChart(),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
		},
		"stamped, release": {
			files:   stampedChart(),
			release: true,
			core:    releasedCore,
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
		},
		"Chart.yaml not stamped": {
			files: with(stampedChart(), map[string]string{
				"Chart.yaml": "apiVersion: v2\nname: provider-x\nversion: 0.1.0\n",
			}),
			wantStamped: []string{"BackupClass pbm (templates/backupclasses.yaml)"},
			wantProblems: []string{
				"Chart.yaml has no openeverest.io/contract-version annotation; run `make generate`",
				"Provider x (templates/provider.yaml) has no openeverest.io/contract-version annotation; include \"openeverest.contract.annotations\", from templates/_openeverest_contract.tpl, in its metadata.annotations",
			},
		},
		"generated with another core": {
			files:   stampedChart(),
			version: "2.0.0-alpha.2",
			wantProblems: []string{
				"Chart.yaml is stamped 2.0.0-alpha.1, but this provider-sdk builds against 2.0.0-alpha.2; run `make generate`",
				"BackupClass pbm (templates/backupclasses.yaml) is stamped 2.0.0-alpha.1, but this provider-sdk builds against 2.0.0-alpha.2; run `make generate`",
				"Provider x (templates/provider.yaml) is stamped 2.0.0-alpha.1, but this provider-sdk builds against 2.0.0-alpha.2; run `make generate`",
			},
		},
		"Provider template without the helper": {
			files: with(stampedChart(), map[string]string{
				"templates/provider.yaml": "apiVersion: core.openeverest.io/v1alpha1\nkind: Provider\nmetadata:\n  name: x\n",
			}),
			wantStamped: []string{"Chart.yaml", "BackupClass pbm (templates/backupclasses.yaml)"},
			wantProblems: []string{
				"Provider x (templates/provider.yaml) has no openeverest.io/contract-version annotation; include \"openeverest.contract.annotations\", from templates/_openeverest_contract.tpl, in its metadata.annotations",
			},
		},
		"BackupClass generated before stamping": {
			files: with(stampedChart(), map[string]string{
				"generated/backupclasses/pbm.yaml": "apiVersion: backup.openeverest.io/v1alpha1\nkind: BackupClass\nmetadata:\n  name: pbm\n",
			}),
			wantStamped: []string{"Chart.yaml", "Provider x (templates/provider.yaml)"},
			wantProblems: []string{
				"BackupClass pbm (templates/backupclasses.yaml) has no openeverest.io/contract-version annotation; run `make generate`, which stamps the BackupClasses it generates",
			},
		},
		"hand-written classes": {
			files: with(stampedChart(), map[string]string{
				"templates/classes.yaml": `apiVersion: backup.openeverest.io/v1alpha1
kind: BackupStorageClass
metadata:
  name: swift
---
apiVersion: monitoring.openeverest.io/v1alpha1
kind: MonitoringClass
metadata:
  name: pmm
  annotations:
    {{- include "openeverest.contract.annotations" . | nindent 4 }}
`,
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"MonitoringClass pmm (templates/classes.yaml)",
				"Provider x (templates/provider.yaml)",
			},
			wantProblems: []string{
				"BackupStorageClass swift (templates/classes.yaml) has no openeverest.io/contract-version annotation; include \"openeverest.contract.annotations\", from templates/_openeverest_contract.tpl, in its metadata.annotations",
			},
		},
		"ClusterInstancePresets with the helper": {
			files: with(stampedChart(), map[string]string{
				"templates/presets.yaml": `{{- range list "small" "large" }}
---
apiVersion: core.openeverest.io/v1alpha1
kind: ClusterInstancePreset
metadata:
  name: x-{{ . }}
  annotations:
    {{- include "openeverest.contract.annotations" $ | nindent 4 }}
spec: {}
{{- end }}
`,
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"ClusterInstancePreset x-small (templates/presets.yaml)",
				"ClusterInstancePreset x-large (templates/presets.yaml)",
				"Provider x (templates/provider.yaml)",
			},
		},
		"ClusterInstancePreset without the helper": {
			files: with(stampedChart(), map[string]string{
				"templates/presets.yaml": "apiVersion: core.openeverest.io/v1alpha1\nkind: ClusterInstancePreset\nmetadata:\n  name: x-small\nspec: {}\n",
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
			wantProblems: []string{
				"ClusterInstancePreset x-small (templates/presets.yaml) has no openeverest.io/contract-version annotation; include \"openeverest.contract.annotations\", from templates/_openeverest_contract.tpl, in its metadata.annotations",
			},
		},
		"objects of a subchart": {
			files: with(stampedChart(), map[string]string{
				"charts/operator/Chart.yaml": "apiVersion: v2\nname: operator\nversion: 0.1.0\n",
				"charts/operator/templates/provider.yaml": `apiVersion: core.openeverest.io/v1alpha1
kind: Provider
metadata:
  name: of-the-subchart
`,
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
		},
		"annotations set in the values": {
			files: with(stampedChart(), map[string]string{
				"values.yaml": "provider:\n  name: x\nopeneverest:\n  annotations:\n    argocd.argoproj.io/sync-wave: \"-1\"\n",
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
		},
		"no Provider": {
			files:        with(stampedChart(), map[string]string{"templates/provider.yaml": ""}),
			wantStamped:  []string{"Chart.yaml", "BackupClass pbm (templates/backupclasses.yaml)"},
			wantProblems: []string{"the chart renders no Provider with its default values"},
		},
		"development value, not a release": {
			files: with(stampedChart(), map[string]string{
				"Chart.yaml":                       "apiVersion: v2\nname: provider-x\nversion: 0.1.0\nannotations:\n  openeverest.io/contract-version: " + development + "\n",
				"generated/backupclasses/pbm.yaml": "",
			}),
			version:     development,
			wantStamped: []string{"Chart.yaml", "Provider x (templates/provider.yaml)"},
		},
		"development value, release": {
			files: with(stampedChart(), map[string]string{
				"Chart.yaml":                       "apiVersion: v2\nname: provider-x\nversion: 0.1.0\nannotations:\n  openeverest.io/contract-version: " + development + "\n",
				"generated/backupclasses/pbm.yaml": "",
			}),
			version:     development,
			release:     true,
			core:        compat.Module{Path: compat.CoreModule, Version: "v2.0.0-dev.2.0.20261123230825-5a38fe56add1"},
			wantStamped: []string{"Chart.yaml", "Provider x (templates/provider.yaml)"},
			wantProblems: []string{
				"built against an unreleased OpenEverest (2.0.0-alpha.1.next): its contract has changed since the last release; require an OpenEverest whose contract is a released one",
			},
		},
		"released value, pseudo-version core, release": {
			files:   stampedChart(),
			release: true,
			core:    compat.Module{Path: compat.CoreModule, Version: "v2.0.0-alpha.1.0.20261123230825-5a38fe56add1"},
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			version := tt.version
			if version == "" {
				version = released
			}

			res, err := Run(Options{
				ChartDir:        writeChart(t, tt.files),
				ContractVersion: version,
				Release:         tt.release,
				Core:            tt.core,
			})

			require.NoError(t, err)
			assert.Equal(t, tt.wantStamped, res.Stamped)
			assert.Equal(t, tt.wantProblems, res.Problems)
		})
	}
}

func TestRunErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		files   map[string]string
		wantErr string
	}{
		"no Chart.yaml": {
			files:   map[string]string{"values.yaml": "{}\n"},
			wantErr: "loading",
		},
		"chart does not render": {
			files:   with(stampedChart(), map[string]string{"templates/bad.yaml": "{{ fail \"broken\" }}\n"}),
			wantErr: "broken",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := Run(Options{ChartDir: writeChart(t, tt.files), ContractVersion: released})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestRunScaffoldedChart checks that the chart init scaffolds passes once
// generate has stamped it.
func TestRunScaffoldedChart(t *testing.T) {
	t.Parallel()

	dest := filepath.Join(t.TempDir(), "provider-test-db")
	require.NoError(t, scaffold.Scaffold(&scaffold.Config{
		ProviderName: "test-db",
		ModulePath:   "github.com/example/provider-test-db",
	}, dest))
	chartDir := filepath.Join(dest, "charts", "provider-test-db")

	res, err := Run(Options{ChartDir: chartDir, ContractVersion: released})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"Chart.yaml has no openeverest.io/contract-version annotation; run `make generate`",
		"Provider test-db (templates/provider.yaml) has no openeverest.io/contract-version annotation; include \"openeverest.contract.annotations\", from templates/_openeverest_contract.tpl, in its metadata.annotations",
	}, res.Problems, "the scaffold is stamped by its first make generate")

	chartFile := filepath.Join(chartDir, "Chart.yaml")
	chart, err := os.ReadFile(chartFile)
	require.NoError(t, err)
	chart = append(chart, []byte("annotations:\n  openeverest.io/contract-version: "+released+"\n")...)
	require.NoError(t, os.WriteFile(chartFile, chart, 0o600))

	res, err = Run(Options{ChartDir: chartDir, ContractVersion: released})
	require.NoError(t, err)
	assert.Empty(t, res.Problems)
	assert.Equal(t, []string{"Chart.yaml", "Provider test-db (templates/provider.yaml)"}, res.Stamped)

	// The scaffolded presets template stamps the presets it renders.
	valuesFile := filepath.Join(chartDir, "values.yaml")
	values, err := os.ReadFile(valuesFile)
	require.NoError(t, err)
	require.Contains(t, string(values), "\npresets: []\n")
	values = []byte(strings.Replace(string(values), "\npresets: []\n", "\npresets:\n  - name: small\n    enabled: true\n    spec: {}\n", 1))
	require.NoError(t, os.WriteFile(valuesFile, values, 0o600))

	res, err = Run(Options{ChartDir: chartDir, ContractVersion: released})
	require.NoError(t, err)
	assert.Empty(t, res.Problems)
	assert.Equal(t, []string{
		"Chart.yaml",
		"ClusterInstancePreset test-db-small (templates/clusterinstancepresets.yaml)",
		"Provider test-db (templates/provider.yaml)",
	}, res.Stamped)
}
