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

package chartrender

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func writeChart(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
}

func TestRender(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeChart(t, dir, map[string]string{
		"Chart.yaml":  "apiVersion: v2\nname: provider-x\nversion: 0.1.0\n",
		"values.yaml": "provider:\n  name: x\nclasses: true\n",
		"templates/_helpers.tpl": `{{- define "x.secret" -}}
{{- $s := lookup "v1" "Secret" .Release.Namespace "x" -}}
{{- if $s }}found{{ else }}absent{{ end -}}
{{- end -}}`,
		"templates/provider.yaml": `apiVersion: core.openeverest.io/v1alpha1
kind: Provider
metadata:
  name: {{ .Values.provider.name }}
  annotations:
    secret: {{ include "x.secret" . }}
`,
		"templates/classes.yaml": `{{- if .Values.classes }}
# A comment-only document is not an object.
---
apiVersion: backup.openeverest.io/v1alpha1
kind: BackupClass
metadata:
  name: b
---
apiVersion: monitoring.openeverest.io/v1alpha1
kind: MonitoringClass
metadata:
  name: m
{{- end }}
`,
		"templates/service.yaml": "apiVersion: v1\nkind: Service\nmetadata:\n  name: svc\n",
		"templates/presets.yaml": "apiVersion: core.openeverest.io/v1alpha1\nkind: ClusterInstancePreset\nmetadata:\n  name: p\n",
		"templates/skipped.yaml": "{{ fail \"rendered a skipped template\" }}\n",
		"templates/NOTES.txt":    "kind: NotAnObject\n",
		"charts/sub/Chart.yaml":  "apiVersion: v2\nname: sub\nversion: 0.1.0\n",
		"charts/sub/templates/provider.yaml": `apiVersion: core.openeverest.io/v1alpha1
kind: Provider
metadata:
  name: subchart
`,
		"charts/sub/templates/deployment.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: operator\n",
	})

	objects, err := Render(dir, Options{Skip: []string{"templates/skipped.yaml"}})

	require.NoError(t, err)
	type summary struct {
		Source, Kind, Name   string
		CoreInterpreted, Own bool
	}
	got := make([]summary, 0, len(objects))
	for _, o := range objects {
		got = append(got, summary{o.Source, o.GroupKind.Kind, o.Name, o.CoreInterpreted(), o.Own()})
	}
	assert.Equal(t, []summary{
		{"charts/sub/templates/deployment.yaml", "Deployment", "operator", false, false},
		{"charts/sub/templates/provider.yaml", "Provider", "subchart", true, false},
		{"templates/classes.yaml", "BackupClass", "b", true, true},
		{"templates/classes.yaml", "MonitoringClass", "m", true, true},
		{"templates/presets.yaml", "ClusterInstancePreset", "p", true, true},
		{"templates/provider.yaml", "Provider", "x", true, true},
		{"templates/service.yaml", "Service", "svc", false, true},
	}, got, "objects of the chart's templates and its subcharts', in template order")
	assert.Equal(t, map[string]string{"secret": "absent"}, objects[5].Annotations, "lookup finds nothing without a cluster")
	assert.Equal(t, schema.GroupKind{Group: "core.openeverest.io", Kind: "Provider"}, objects[5].GroupKind)
}

func TestRenderErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		files   map[string]string
		wantErr string
	}{
		"no chart": {
			files:   map[string]string{"values.yaml": "{}\n"},
			wantErr: "loading chart",
		},
		"failing template": {
			files: map[string]string{
				"Chart.yaml":         "apiVersion: v2\nname: provider-x\nversion: 0.1.0\n",
				"templates/bad.yaml": "{{ fail \"no\" }}\n",
			},
			wantErr: "rendering chart",
		},
		"a CRD in crds/ that does not decode": {
			files: map[string]string{
				"Chart.yaml":       "apiVersion: v2\nname: provider-x\nversion: 0.1.0\n",
				"crds/bad.yaml":    "kind: [\n",
				"templates/x.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n",
			},
			wantErr: "provider-x/crds/bad.yaml: decoding CRD",
		},
		"invalid apiVersion": {
			files: map[string]string{
				"Chart.yaml":       "apiVersion: v2\nname: provider-x\nversion: 0.1.0\n",
				"templates/x.yaml": "apiVersion: a/b/c\nkind: X\nmetadata:\n  name: x\n",
			},
			wantErr: "templates/x.yaml: X x",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeChart(t, dir, tt.files)

			_, err := Render(dir, Options{})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestRenderValues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeChart(t, dir, map[string]string{
		"Chart.yaml":  "apiVersion: v2\nname: provider-x\nversion: 0.1.0\n",
		"values.yaml": "provider:\n  name: x\nack: \"\"\n",
		"templates/provider.yaml": `apiVersion: core.openeverest.io/v1alpha1
kind: Provider
metadata:
  name: {{ .Values.provider.name }}
  annotations:
    ack: {{ .Values.ack | quote }}
`,
	})

	tests := map[string]struct {
		values map[string]any
		want   map[string]string
	}{
		"default values": {want: map[string]string{"ack": ""}},
		"values over the defaults": {
			values: map[string]any{"ack": "2.1.0"},
			want:   map[string]string{"ack": "2.1.0"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			objects, err := Render(dir, Options{Values: tt.values})

			require.NoError(t, err)
			require.Len(t, objects, 1)
			assert.Equal(t, "x", objects[0].Name, "the other defaults stay")
			assert.Equal(t, tt.want, objects[0].Annotations)
		})
	}
}

// operatorChart is a provider chart bundling an operator chart that, like
// many, refuses to render without the API of a prerequisite, cert-manager,
// and ships CRDs in crds/. templates/apis.yaml reports the API versions the
// chart renders with.
func operatorChart(annotations string) map[string]string {
	return map[string]string{
		"Chart.yaml": `apiVersion: v2
name: provider-x
version: 0.1.0
dependencies:
  - name: operator
    version: 0.1.0
    condition: operator.enabled
` + annotations,
		"values.yaml": "operator:\n  enabled: true\n",
		"templates/apis.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: apis
  annotations:
    builtin: {{ .Capabilities.APIVersions.Has "apps/v1" | quote }}
    widget: {{ .Capabilities.APIVersions.Has "example.com/v1/Widget" | quote }}
    widget-group-version: {{ .Capabilities.APIVersions.Has "example.com/v1" | quote }}
    widget-not-served: {{ .Capabilities.APIVersions.Has "example.com/v1alpha1/Widget" | quote }}
    gadget: {{ .Capabilities.APIVersions.Has "example.com/v1beta1/Gadget" | quote }}
    cert-manager: {{ .Capabilities.APIVersions.Has "cert-manager.io/v1/Certificate" | quote }}
    prometheus: {{ .Capabilities.APIVersions.Has "monitoring.coreos.com/v1" | quote }}
`,
		"charts/operator/Chart.yaml": "apiVersion: v2\nname: operator\nversion: 0.1.0\n",
		"charts/operator/crds/crds.yaml": `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    kind: Widget
    plural: widgets
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
    - name: v1alpha1
      served: false
      storage: false
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gadgets.example.com
spec:
  group: example.com
  names:
    kind: Gadget
    plural: gadgets
  scope: Namespaced
  versions:
    - name: v1beta1
      served: true
      storage: true
`,
		"charts/operator/templates/preflight.yaml": `{{- if not (.Capabilities.APIVersions.Has "cert-manager.io/v1/Certificate") -}}
{{- fail "cert-manager is required but was not found in the cluster" -}}
{{- end -}}
`,
		"charts/operator/templates/deployment.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: operator\n",
	}
}

func TestRenderAPIVersions(t *testing.T) {
	t.Parallel()

	served := func(changes map[string]string) map[string]string {
		apis := map[string]string{
			"builtin":              "true",
			"widget":               "true",
			"widget-group-version": "true",
			"widget-not-served":    "false",
			"gadget":               "true",
			"cert-manager":         "true",
			"prometheus":           "false",
		}
		for k, v := range changes {
			apis[k] = v
		}
		return apis
	}
	tests := map[string]struct {
		annotations string
		opts        Options
		wantErr     string
		wantAPIs    map[string]string
	}{
		"a subchart requiring an API nothing declares": {
			wantErr: `cert-manager is required but was not found in the cluster`,
		},
		"the API declared in Chart.yaml": {
			annotations: "annotations:\n  openeverest.io/api-versions: cert-manager.io/v1/Certificate\n",
			wantAPIs:    served(nil),
		},
		"several APIs declared, separated by commas and whitespace": {
			annotations: "annotations:\n  openeverest.io/api-versions: |\n    cert-manager.io/v1/Certificate,\n    monitoring.coreos.com/v1\n",
			wantAPIs:    served(map[string]string{"prometheus": "true"}),
		},
		"the API passed as with helm template --api-versions": {
			opts:     Options{APIVersions: []string{"cert-manager.io/v1/Certificate"}},
			wantAPIs: served(nil),
		},
		"both": {
			annotations: "annotations:\n  openeverest.io/api-versions: cert-manager.io/v1/Certificate\n",
			opts:        Options{APIVersions: []string{"monitoring.coreos.com/v1"}},
			wantAPIs:    served(map[string]string{"prometheus": "true"}),
		},
		"a disabled subchart, whose CRDs are not installed": {
			opts: Options{Values: map[string]any{"operator": map[string]any{"enabled": false}}},
			wantAPIs: served(map[string]string{
				"widget": "false", "widget-group-version": "false", "gadget": "false", "cert-manager": "false",
			}),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeChart(t, dir, operatorChart(tt.annotations))

			objects, err := Render(dir, tt.opts)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr, "the subchart's error is reported")
				assert.Contains(t, err.Error(), APIVersionsAnnotation, "with where to declare the API")
				return
			}
			require.NoError(t, err)
			var apis map[string]string
			var operator bool
			for _, o := range objects {
				switch o.Source {
				case "templates/apis.yaml":
					apis = o.Annotations
				case "charts/operator/templates/deployment.yaml":
					operator = true
				}
			}
			assert.Equal(t, tt.wantAPIs, apis)
			assert.Equal(t, tt.opts.Values == nil, operator, "the objects of the subchart, when enabled")
		})
	}
}
