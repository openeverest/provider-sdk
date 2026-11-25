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

	objects, err := Render(dir, "templates/skipped.yaml")

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

			_, err := Render(dir)

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

			objects, err := RenderValues(dir, tt.values)

			require.NoError(t, err)
			require.Len(t, objects, 1)
			assert.Equal(t, "x", objects[0].Name, "the other defaults stay")
			assert.Equal(t, tt.want, objects[0].Annotations)
		})
	}
}
