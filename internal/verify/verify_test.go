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
	development = "2.0.0-alpha.1.dev"
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
    argocd.argoproj.io/sync-wave: "-1"
spec: {}
`,
		"templates/deployment.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: provider-x
spec: {}
`,
	}
}

// deployment is a workload template with annotations.
func deployment(annotations string) string {
	return "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: provider-x\n  annotations:\n" + annotations + "spec: {}\n"
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
		"BackupClass generated before the sync wave": {
			files: with(stampedChart(), map[string]string{
				"generated/backupclasses/pbm.yaml": "apiVersion: backup.openeverest.io/v1alpha1\nkind: BackupClass\nmetadata:\n  name: pbm\n  annotations:\n    openeverest.io/contract-version: " + released + "\n",
			}),
			wantStamped: []string{"Chart.yaml", "Provider x (templates/provider.yaml)"},
			wantProblems: []string{
				"BackupClass pbm (templates/backupclasses.yaml) is at Argo CD sync wave 0, not -1; run `make generate`, which stamps the BackupClasses it generates",
			},
		},
		"stamped object at another sync wave": {
			files: with(stampedChart(), map[string]string{
				"templates/classes.yaml": `apiVersion: monitoring.openeverest.io/v1alpha1
kind: MonitoringClass
metadata:
  name: pmm
  annotations:
    openeverest.io/contract-version: "` + released + `"
    argocd.argoproj.io/sync-wave: "-3"
`,
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
			wantProblems: []string{
				"MonitoringClass pmm (templates/classes.yaml) is at Argo CD sync wave -3, not -1; include \"openeverest.contract.annotations\", from templates/_openeverest_contract.tpl, in its metadata.annotations",
			},
		},
		"stamped with another core and at another sync wave": {
			files: with(stampedChart(), map[string]string{
				"templates/classes.yaml": `apiVersion: monitoring.openeverest.io/v1alpha1
kind: MonitoringClass
metadata:
  name: pmm
  annotations:
    openeverest.io/contract-version: "2.0.0-alpha.2"
`,
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
			wantProblems: []string{
				"MonitoringClass pmm (templates/classes.yaml) is stamped 2.0.0-alpha.2, but this provider-sdk builds against 2.0.0-alpha.1; run `make generate`",
				"MonitoringClass pmm (templates/classes.yaml) is at Argo CD sync wave 0, not -1; include \"openeverest.contract.annotations\", from templates/_openeverest_contract.tpl, in its metadata.annotations",
			},
		},
		"workloads after the stamped objects": {
			files: with(stampedChart(), map[string]string{
				"templates/deployment.yaml": deployment("    argocd.argoproj.io/sync-wave: \"0\"\n") + "---\n" +
					"apiVersion: apps/v1\nkind: StatefulSet\nmetadata:\n  name: s\n  annotations:\n    argocd.argoproj.io/sync-wave: \"5\"\n",
				"templates/hooks.yaml": `apiVersion: batch/v1
kind: Job
metadata:
  name: preflight
  annotations:
    helm.sh/hook: pre-upgrade
    helm.sh/hook-weight: "-5"
---
apiVersion: batch/v1
kind: Job
metadata:
  name: argo-presync
  annotations:
    argocd.argoproj.io/hook: PreSync
    argocd.argoproj.io/sync-wave: "-1"
`,
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
		},
		"workloads at or before the stamped objects": {
			files: with(stampedChart(), map[string]string{
				"templates/deployment.yaml": deployment("    argocd.argoproj.io/sync-wave: \"-1\"\n"),
				"templates/workloads.yaml": `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: d
  annotations:
    argocd.argoproj.io/sync-wave: "-2"
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: c
  annotations:
    helm.sh/hook-weight: "-1"
---
apiVersion: v1
kind: Pod
metadata:
  name: p
  annotations:
    argocd.argoproj.io/hook: Sync,PostSync
    argocd.argoproj.io/sync-wave: "-10"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: not-a-workload
  annotations:
    argocd.argoproj.io/sync-wave: "-5"
`,
				"charts/operator/Chart.yaml": "apiVersion: v2\nname: operator\nversion: 0.1.0\n",
				"charts/operator/templates/deployment.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: operator
  annotations:
    argocd.argoproj.io/sync-wave: "-1"
---
apiVersion: core.openeverest.io/v1alpha1
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
			wantProblems: []string{
				"Deployment operator (charts/operator/templates/deployment.yaml) is at Argo CD sync wave -1; put it after wave -1, that of the objects OpenEverest checks, so that Argo CD stops the sync before it runs when OpenEverest refuses one of them",
				"Deployment provider-x (templates/deployment.yaml) is at Argo CD sync wave -1; put it after wave -1, that of the objects OpenEverest checks, so that Argo CD stops the sync before it runs when OpenEverest refuses one of them",
				"DaemonSet d (templates/workloads.yaml) is at Argo CD sync wave -2; put it after wave -1, that of the objects OpenEverest checks, so that Argo CD stops the sync before it runs when OpenEverest refuses one of them",
				"CronJob c (templates/workloads.yaml) is at Argo CD sync wave -1; put it after wave -1, that of the objects OpenEverest checks, so that Argo CD stops the sync before it runs when OpenEverest refuses one of them",
				"Pod p (templates/workloads.yaml) is at Argo CD sync wave -10; put it after wave -1, that of the objects OpenEverest checks, so that Argo CD stops the sync before it runs when OpenEverest refuses one of them",
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
				"built against an unreleased OpenEverest (2.0.0-alpha.1.dev); require a released OpenEverest in go.mod",
				"go.mod requires github.com/openeverest/openeverest/v2 at the pseudo-version v2.0.0-dev.2.0.20261123230825-5a38fe56add1, an unreleased commit; require a released OpenEverest",
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
			wantProblems: []string{
				"go.mod requires github.com/openeverest/openeverest/v2 at the pseudo-version v2.0.0-alpha.1.0.20261123230825-5a38fe56add1, an unreleased commit; require a released OpenEverest",
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

func TestSyncWave(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		annotations map[string]string
		want        int
	}{
		"none":                            {want: 0},
		"sync wave":                       {annotations: map[string]string{"argocd.argoproj.io/sync-wave": "-1"}, want: -1},
		"Helm hook weight":                {annotations: map[string]string{"helm.sh/hook-weight": "-5"}, want: -5},
		"sync wave over Helm hook weight": {annotations: map[string]string{"argocd.argoproj.io/sync-wave": "2", "helm.sh/hook-weight": "-5"}, want: 2},
		"not an integer falls back":       {annotations: map[string]string{"argocd.argoproj.io/sync-wave": " -1", "helm.sh/hook-weight": "3"}, want: 3},
		"nothing an integer":              {annotations: map[string]string{"argocd.argoproj.io/sync-wave": "first"}, want: 0},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, syncWave(tt.annotations))
		})
	}
}

func TestSyncPhase(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		annotations map[string]string
		want        bool
	}{
		"not a hook":                        {want: true},
		"Helm hook":                         {annotations: map[string]string{"helm.sh/hook": "pre-install,pre-upgrade"}, want: false},
		"Argo CD PreSync hook":              {annotations: map[string]string{"argocd.argoproj.io/hook": "PreSync"}, want: false},
		"Argo CD Sync hook":                 {annotations: map[string]string{"argocd.argoproj.io/hook": "PostSync, Sync"}, want: true},
		"Argo CD hook over Helm hook":       {annotations: map[string]string{"argocd.argoproj.io/hook": "Sync", "helm.sh/hook": "pre-install"}, want: true},
		"Argo CD Skip":                      {annotations: map[string]string{"argocd.argoproj.io/hook": "Skip"}, want: false},
		"Helm hook weight without the hook": {annotations: map[string]string{"helm.sh/hook-weight": "-1"}, want: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, syncPhase(tt.annotations))
		})
	}
}
