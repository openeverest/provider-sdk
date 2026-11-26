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

const (
	providersCRD         = "providers.core.openeverest.io"
	backupClassesCRD     = "backupclasses.backup.openeverest.io"
	monitoringClassesCRD = "monitoringclasses.monitoring.openeverest.io"
	presetsCRD           = "clusterinstancepresets.core.openeverest.io"
)

// stampedChart is a chart whose Provider and BackupClass carry the stamp
// released, the way generate and the scaffolded templates write them, with
// the check of the CRDs of both.
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
		"templates/" + generate.CompatCheckFile:    generate.CompatCheck([]string{providersCRD, backupClassesCRD}),
		"templates/provider.yaml": `apiVersion: core.openeverest.io/v1alpha1
kind: Provider
metadata:
  name: {{ .Values.provider.name }}
  annotations:
    {{- include "openeverest.contract.annotations" . | nindent 4 }}
spec: {}
`,
		"templates/backupclasses.yaml": backupClassesTemplate,
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

// backupClassesTemplate renders the generated BackupClasses like the
// scaffolded templates/backupclasses.yaml.
const backupClassesTemplate = `{{- range $path, $_ := .Files.Glob "generated/backupclasses/*.yaml" }}
---
{{ include "openeverest.contract.generatedObject" (dict "root" $ "path" $path) }}
{{- end }}
`

// rawBackupClassesTemplate renders the generated BackupClasses as they are
// written, without the acknowledgement.
const rawBackupClassesTemplate = `{{- range $path, $_ := .Files.Glob "generated/backupclasses/*.yaml" }}
---
{{ $.Files.Get $path }}
{{- end }}
`

// baseRules are rules of a provider's ClusterRole other than those of the
// startup gate.
const baseRules = `- apiGroups: [core.openeverest.io]
  resources: [providers]
  verbs: [get, list, watch]
`

// gateRules are the rules the startup gate of the Provider x needs.
const gateRules = baseRules + `- apiGroups: [apiextensions.k8s.io]
  resources: [customresourcedefinitions]
  resourceNames: [providers.core.openeverest.io]
  verbs: [get]
- apiGroups: [core.openeverest.io]
  resources: [providers/status]
  resourceNames: [x]
  verbs: [patch]
`

// clusterRole returns a ClusterRole with rules.
func clusterRole(rules string) string {
	return "apiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\nmetadata:\n  name: provider-x\nrules:\n" + rules
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
				"templates/" + generate.CompatCheckFile: generate.CompatCheck([]string{
					providersCRD, backupClassesCRD, "backupstorageclasses.backup.openeverest.io", "monitoringclasses.monitoring.openeverest.io",
				}),
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
				"templates/" + generate.CompatCheckFile: generate.CompatCheck([]string{providersCRD, backupClassesCRD, presetsCRD}),
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
				"templates/" + generate.CompatCheckFile: generate.CompatCheck([]string{providersCRD, backupClassesCRD, presetsCRD}),
				"templates/presets.yaml":                "apiVersion: core.openeverest.io/v1alpha1\nkind: ClusterInstancePreset\nmetadata:\n  name: x-small\nspec: {}\n",
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
				"Chart.yaml":                            "apiVersion: v2\nname: provider-x\nversion: 0.1.0\nannotations:\n  openeverest.io/contract-version: " + development + "\n",
				"generated/backupclasses/pbm.yaml":      "",
				"templates/" + generate.CompatCheckFile: generate.CompatCheck([]string{providersCRD}),
			}),
			version:     development,
			wantStamped: []string{"Chart.yaml", "Provider x (templates/provider.yaml)"},
		},
		"development value, release": {
			files: with(stampedChart(), map[string]string{
				"Chart.yaml":                            "apiVersion: v2\nname: provider-x\nversion: 0.1.0\nannotations:\n  openeverest.io/contract-version: " + development + "\n",
				"generated/backupclasses/pbm.yaml":      "",
				"templates/" + generate.CompatCheckFile: generate.CompatCheck([]string{providersCRD}),
			}),
			version:     development,
			release:     true,
			core:        compat.Module{Path: compat.CoreModule, Version: "v2.0.0-dev.2.0.20261123230825-5a38fe56add1"},
			wantStamped: []string{"Chart.yaml", "Provider x (templates/provider.yaml)"},
			wantProblems: []string{
				"built against an unreleased OpenEverest (2.0.0-alpha.1.next): its contract has changed since the last release; require an OpenEverest whose contract is a released one",
			},
		},
		"generated templates missing": {
			files: with(stampedChart(), map[string]string{
				"templates/" + generate.CompatCheckFile:    "",
				"templates/provider.yaml":                  "apiVersion: core.openeverest.io/v1alpha1\nkind: Provider\nmetadata:\n  name: x\n  annotations:\n    openeverest.io/contract-version: " + released + "\n",
				"templates/" + generate.ContractHelperFile: "",
				"templates/backupclasses.yaml":             rawBackupClassesTemplate,
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
			wantProblems: []string{
				"templates/_openeverest_contract.tpl is missing; run `make generate`",
				"templates/openeverest-compat-check.yaml is missing; run `make generate`",
				"BackupClass pbm (templates/backupclasses.yaml) has no openeverest.io/compatibility-acknowledged annotation when compatibility.acknowledge.coreVersion is set, so OpenEverest would refuse it even once the user acknowledged it; render the generated BackupClasses with {{ include \"openeverest.contract.generatedObject\" (dict \"root\" $ \"path\" $path) }}, as the scaffolded templates/backupclasses.yaml does",
				"Provider x (templates/provider.yaml) has no openeverest.io/compatibility-acknowledged annotation when compatibility.acknowledge.coreVersion is set, so OpenEverest would refuse it even once the user acknowledged it; include \"openeverest.contract.annotations\", from templates/_openeverest_contract.tpl, in its metadata.annotations",
			},
		},
		"BackupClasses rendered without the acknowledgement": {
			files: with(stampedChart(), map[string]string{"templates/backupclasses.yaml": rawBackupClassesTemplate}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
			wantProblems: []string{
				"BackupClass pbm (templates/backupclasses.yaml) has no openeverest.io/compatibility-acknowledged annotation when compatibility.acknowledge.coreVersion is set, so OpenEverest would refuse it even once the user acknowledged it; render the generated BackupClasses with {{ include \"openeverest.contract.generatedObject\" (dict \"root\" $ \"path\" $path) }}, as the scaffolded templates/backupclasses.yaml does",
			},
		},
		"generated templates of another provider-sdk": {
			files: with(stampedChart(), map[string]string{
				"templates/" + generate.ContractHelperFile: generate.ContractHelper + "{{/* changed */}}\n",
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
			wantProblems: []string{"templates/_openeverest_contract.tpl is out of date; run `make generate`"},
		},
		"check of CRDs the chart no longer applies": {
			files: with(stampedChart(), map[string]string{"generated/backupclasses/pbm.yaml": ""}),
			wantStamped: []string{
				"Chart.yaml",
				"Provider x (templates/provider.yaml)",
			},
			wantProblems: []string{"templates/openeverest-compat-check.yaml is out of date; run `make generate`"},
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
		"ClusterRole allows the startup gate": {
			files: with(stampedChart(), map[string]string{"templates/clusterrole.yaml": clusterRole(gateRules)}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
		},
		"ClusterRoles allow the startup gate with wildcards": {
			files: with(stampedChart(), map[string]string{
				"templates/clusterrole.yaml": clusterRole(`- apiGroups: ["*"]
  resources: [customresourcedefinitions]
  verbs: ["*"]
`) + "---\n" + clusterRole(`- apiGroups: [core.openeverest.io]
  resources: ["*/status"]
  verbs: [patch]
`),
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
		},
		"ClusterRole without the startup gate's rules": {
			files: with(stampedChart(), map[string]string{"templates/clusterrole.yaml": clusterRole(baseRules)}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
			wantProblems: []string{
				"no ClusterRole of the chart allows get on customresourcedefinitions providers.core.openeverest.io (apiextensions.k8s.io), which provider-runtime needs to read the OpenEverest versions the cluster supports; without it, a new build of the provider never becomes ready; add `// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,resourceNames=providers.core.openeverest.io,verbs=get` to internal/provider/rbac.go and run `make generate`",
				"no ClusterRole of the chart allows patch on providers/status x (core.openeverest.io), which provider-runtime needs to record on its Provider the version that started; without it, every restart waits for the check like a new build; add `// +kubebuilder:rbac:groups=core.openeverest.io,resources=providers/status,resourceNames=x,verbs=patch` to internal/provider/rbac.go and run `make generate`",
			},
		},
		"ClusterRole for another Provider": {
			files: with(stampedChart(), map[string]string{
				"templates/clusterrole.yaml": clusterRole(strings.ReplaceAll(gateRules, "resourceNames: [x]", "resourceNames: [y]")),
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
			wantProblems: []string{
				"no ClusterRole of the chart allows patch on providers/status x (core.openeverest.io), which provider-runtime needs to record on its Provider the version that started; without it, every restart waits for the check like a new build; add `// +kubebuilder:rbac:groups=core.openeverest.io,resources=providers/status,resourceNames=x,verbs=patch` to internal/provider/rbac.go and run `make generate`",
			},
		},
		"startup gate allowed only by a subchart's ClusterRole": {
			files: with(stampedChart(), map[string]string{
				"templates/clusterrole.yaml":                 clusterRole(baseRules),
				"charts/operator/Chart.yaml":                 "apiVersion: v2\nname: operator\nversion: 0.1.0\n",
				"charts/operator/templates/clusterrole.yaml": clusterRole(gateRules),
			}),
			wantStamped: []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			},
			wantProblems: []string{
				"no ClusterRole of the chart allows get on customresourcedefinitions providers.core.openeverest.io (apiextensions.k8s.io), which provider-runtime needs to read the OpenEverest versions the cluster supports; without it, a new build of the provider never becomes ready; add `// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,resourceNames=providers.core.openeverest.io,verbs=get` to internal/provider/rbac.go and run `make generate`",
				"no ClusterRole of the chart allows patch on providers/status x (core.openeverest.io), which provider-runtime needs to record on its Provider the version that started; without it, every restart waits for the check like a new build; add `// +kubebuilder:rbac:groups=core.openeverest.io,resources=providers/status,resourceNames=x,verbs=patch` to internal/provider/rbac.go and run `make generate`",
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

// TestRunBundledOperator checks that verify renders a bundled operator chart
// that refuses to render without the API of a prerequisite, as users install
// it, rather than skipping it.
func TestRunBundledOperator(t *testing.T) {
	t.Parallel()

	operator := with(stampedChart(), map[string]string{
		"charts/operator/Chart.yaml": "apiVersion: v2\nname: operator\nversion: 0.1.0\n",
		"charts/operator/templates/preflight.yaml": `{{- if not (.Capabilities.APIVersions.Has "cert-manager.io/v1/Certificate") -}}
{{- fail "cert-manager is required but was not found in the cluster" -}}
{{- end -}}
`,
	})
	declared := with(operator, map[string]string{
		"Chart.yaml": operator["Chart.yaml"] + "  openeverest.io/api-versions: cert-manager.io/v1/Certificate\n",
	})
	tests := map[string]struct {
		files       map[string]string
		apiVersions []string
		wantErr     string
	}{
		"the API it requires not declared": {
			files:   operator,
			wantErr: "cert-manager is required but was not found in the cluster",
		},
		"the API declared in Chart.yaml": {
			files: declared,
		},
		"the API passed with --api-versions": {
			files:       operator,
			apiVersions: []string{"cert-manager.io/v1/Certificate"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			res, err := Run(Options{ChartDir: writeChart(t, tt.files), ContractVersion: released, APIVersions: tt.apiVersions})

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Empty(t, res.Problems)
			assert.Equal(t, []string{
				"Chart.yaml",
				"BackupClass pbm (templates/backupclasses.yaml)",
				"Provider x (templates/provider.yaml)",
			}, res.Stamped)
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
	assert.Equal(t, []string{"templates/openeverest-compat-check.yaml is out of date; run `make generate`"}, res.Problems,
		"the check reads the CRD of the presets the chart now applies")
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "templates", generate.CompatCheckFile),
		[]byte(generate.CompatCheck([]string{providersCRD, presetsCRD})), 0o600))

	res, err = Run(Options{ChartDir: chartDir, ContractVersion: released})
	require.NoError(t, err)
	assert.Empty(t, res.Problems)
	assert.Equal(t, []string{
		"Chart.yaml",
		"ClusterInstancePreset test-db-small (templates/clusterinstancepresets.yaml)",
		"Provider test-db (templates/provider.yaml)",
	}, res.Stamped)

	// The scaffolded ClusterRole allows the startup gate with the rules of
	// the scaffolded rbac.go, which make generate writes to the chart.
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "generated", "rbac-rules.yaml"), []byte(baseRules), 0o600))

	res, err = Run(Options{ChartDir: chartDir, ContractVersion: released})
	require.NoError(t, err)
	require.Len(t, res.Problems, 2)
	assert.Contains(t, res.Problems[0], "allows get on customresourcedefinitions providers.core.openeverest.io (apiextensions.k8s.io)")
	assert.Contains(t, res.Problems[1], "allows patch on providers/status test-db (core.openeverest.io)")
}
