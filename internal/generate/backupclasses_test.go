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
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	// The generated package imports them; TestShippedBackupClassesCompile
	// builds it with this module's graph.
	_ "k8s.io/apimachinery/pkg/util/yaml"

	_ "github.com/openeverest/openeverest/v2/api/backup/v1alpha1"
	"github.com/openeverest/openeverest/v2/backupstorageclasses/s3"
	_ "github.com/openeverest/openeverest/v2/provider-runtime/controller"
)

func TestAssembleBackupClasses_BackupStorageClasses(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		class string
		err   string
	}{
		"declared": {
			class: "executionMode: ProviderManaged\nbackupStorageClasses:\n  - name: s3\n    authModes: [AccessKey]\n",
		},
		"missing": {
			class: "executionMode: ProviderManaged\n",
			err:   "backupclass pbm: class.yaml must list the backupStorageClasses it can write to",
		},
		"empty": {
			class: "executionMode: ProviderManaged\nbackupStorageClasses: []\n",
			err:   "backupclass pbm: class.yaml must list the backupStorageClasses it can write to",
		},
		"execution mode left to generate": {
			class: "backupStorageClasses:\n  - name: s3\n    authModes: [AccessKey]\n",
		},
		"tls declared": {
			class: "backupStorageClasses:\n  - name: s3\n    authModes: [AccessKey]\n    tls: {customCA: Supported, insecureSkipVerify: None}\n",
		},
		"supportedProviders": {
			class: "supportedProviders: [mongo]\nbackupStorageClasses:\n  - name: s3\n    authModes: [AccessKey]\n",
			err:   "backupclass pbm: class.yaml must not list supportedProviders: generate names the provider in providerManaged.providerRef",
		},
		"another execution mode": {
			class: "executionMode: Job\nbackupStorageClasses:\n  - name: s3\n    authModes: [AccessKey]\n",
			err:   "backupclass pbm: executionMode Job is not supported: a provider's classes are ProviderManaged",
		},
		"providerRef": {
			class: "providerManaged:\n  providerRef: {name: mongo}\nbackupStorageClasses:\n  - name: s3\n    authModes: [AccessKey]\n",
			err:   "backupclass pbm: class.yaml must not set providerManaged.providerRef: generate sets it to the provider",
		},
		"unknown tls setting": {
			class: "backupStorageClasses:\n  - name: s3\n    authModes: [AccessKey]\n    tls: {ca: Supported}\n",
			err:   "backupclass pbm: backupStorageClasses[0].tls.ca is not a TLS setting: write customCA or insecureSkipVerify",
		},
		"tls value": {
			class: "backupStorageClasses:\n  - name: s3\n    authModes: [AccessKey]\n    tls: {customCA: true}\n",
			err:   "backupclass pbm: backupStorageClasses[0].tls.customCA must be Supported or None, not true",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			defDir := t.TempDir()
			classDir := filepath.Join(defDir, "backupclasses", "pbm")
			require.NoError(t, os.MkdirAll(classDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(classDir, "class.yaml"), []byte(tt.class), 0o600))

			classes, err := AssembleBackupClasses(defDir, map[string]bool{})
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Len(t, classes, 1)
			assert.Equal(t, "pbm", classes[0].Name)
		})
	}
}

func TestBuildBackupClassManifest(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		class   string
		want    string
		wantErr string
	}{
		"s3 with the contract's schema": {
			class: `
backupStorageClasses:
  - name: s3
    authModes: [AccessKey]
`,
			want: `
executionMode: ProviderManaged
providerManaged:
  providerRef: {name: percona-server-mongodb}
backupStorageClasses:
  - name: s3
    authModes: [AccessKey]
    parametersSchema:
      openAPIV3Schema: CONTRACT
`,
		},
		"s3 narrowed, with tls and PITR": {
			class: `
providerManaged:
  supportsPITR: true
backupStorageClasses:
  - name: s3
    authModes: [AccessKey]
    parametersSchema:
      openAPIV3Schema:
        properties:
          implementation: {enum: [aws, minio, ceph, other]}
          endpointURL: {pattern: "^https://"}
    tls: {customCA: Supported, insecureSkipVerify: Supported}
`,
			want: `
executionMode: ProviderManaged
providerManaged:
  supportsPITR: true
  providerRef: {name: percona-server-mongodb}
backupStorageClasses:
  - name: s3
    authModes: [AccessKey]
    parametersSchema:
      openAPIV3Schema: NARROWED
    tls: {customCA: Supported, insecureSkipVerify: Supported}
`,
		},
		"a storage class without a known contract keeps its schema": {
			class: `
backupStorageClasses:
  - name: swift.example.com
    authModes: [Token]
    parametersSchema:
      openAPIV3Schema: {type: object}
`,
			want: `
executionMode: ProviderManaged
providerManaged:
  providerRef: {name: percona-server-mongodb}
backupStorageClasses:
  - name: swift.example.com
    authModes: [Token]
    parametersSchema:
      openAPIV3Schema: {type: object}
`,
		},
		"a parameter the contract does not have": {
			class: `
backupStorageClasses:
  - name: s3
    authModes: [AccessKey]
    parametersSchema:
      openAPIV3Schema:
        properties:
          storageClass: {enum: [STANDARD]}
`,
			wantErr: "backupclass pbm: storage class s3: parametersSchema.openAPIV3Schema.storageClass is not a parameter of the contract",
		},
		"an enum value the contract does not have": {
			class: `
backupStorageClasses:
  - name: s3
    authModes: [AccessKey]
    parametersSchema:
      openAPIV3Schema:
        properties:
          implementation: {enum: [aws, wasabi]}
`,
			wantErr: "backupclass pbm: storage class s3: parametersSchema.openAPIV3Schema.implementation: enum value wasabi is not one of the contract's [aws minio ceph gcs other]",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var class map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(tt.class), &class))
			got, err := buildBackupClassManifest(BackupClassAssembled{Name: "pbm", Class: class}, nil, "percona-server-mongodb")
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			contract, ok, err := storageClassContract(s3.ClassName)
			require.NoError(t, err)
			require.True(t, ok)
			closed := deepCopyMap(contract)
			closed["additionalProperties"] = false
			narrowed := deepCopyMap(closed)
			props := narrowed["properties"].(map[string]any)
			props["implementation"].(map[string]any)["enum"] = []any{"aws", "minio", "ceph", "other"}
			props["endpointURL"].(map[string]any)["pattern"] = "^https://"

			var want map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(tt.want), &want))
			for _, e := range want["backupStorageClasses"].([]any) {
				ps := e.(map[string]any)["parametersSchema"].(map[string]any)
				switch ps["openAPIV3Schema"] {
				case "CONTRACT":
					ps["openAPIV3Schema"] = closed
				case "NARROWED":
					ps["openAPIV3Schema"] = narrowed
				}
			}
			assert.Equal(t, roundTrip(t, want), roundTrip(t, got["spec"]))
			assert.Equal(t, map[string]any{"name": "pbm"}, got["metadata"])
		})
	}
}

func TestNarrowSchemaRequiredAndType(t *testing.T) {
	t.Parallel()

	contract := map[string]any{
		"type":     "object",
		"required": []any{"region"},
		"properties": map[string]any{
			"region":         map[string]any{"type": "string"},
			"forcePathStyle": map[string]any{"type": "boolean"},
		},
	}
	got, err := narrowSchema(contract, map[string]any{"required": []any{"forcePathStyle", "region"}})
	require.NoError(t, err)
	assert.Equal(t, []any{"region", "forcePathStyle"}, got["required"])
	assert.Equal(t, false, got["additionalProperties"])
	assert.NotContains(t, contract, "additionalProperties", "the contract is not modified")

	_, err = narrowSchema(contract, map[string]any{"properties": map[string]any{"region": map[string]any{"type": "integer"}}})
	require.EqualError(t, err, "region: type integer is not the contract's string")
	_, err = narrowSchema(contract, map[string]any{"additionalProperties": true})
	require.EqualError(t, err, "additionalProperties cannot narrow the contract: write properties and required")
}

func TestWriteShippedBackupClasses(t *testing.T) {
	t.Parallel()

	defDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(defDir, "backupclasses"), 0o755))
	docs := [][]byte{[]byte("kind: BackupClass\nmetadata: {name: a}\n"), []byte("kind: BackupClass\nmetadata: {name: b}\n")}
	require.NoError(t, writeShippedBackupClasses(defDir, docs))

	data, err := os.ReadFile(filepath.Join(defDir, "backupclasses", shippedBackupClassesYAML))
	require.NoError(t, err)
	assert.Contains(t, string(data), "metadata: {name: a}\n---\nkind: BackupClass\nmetadata: {name: b}\n")
	src, err := os.ReadFile(filepath.Join(defDir, "backupclasses", shippedBackupClassesGo))
	require.NoError(t, err)
	assert.Equal(t, shippedBackupClassesSource, string(src))

	require.NoError(t, writeShippedBackupClasses(defDir, nil))
	assert.NoFileExists(t, filepath.Join(defDir, "backupclasses", shippedBackupClassesYAML))
	assert.NoFileExists(t, filepath.Join(defDir, "backupclasses", shippedBackupClassesGo))
	require.NoError(t, writeShippedBackupClasses(t.TempDir(), nil), "nothing to remove")
}

// TestShippedBackupClassesCompile type-checks the generated package against
// the openeverest version provider-sdk builds with and runs it on a rendered
// class.
func TestShippedBackupClassesCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}
	t.Parallel()

	var class map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(`
backupStorageClasses:
  - name: s3
    authModes: [AccessKey]
    tls: {customCA: Supported}
`), &class))
	manifest, err := buildBackupClassManifest(BackupClassAssembled{Name: "pbm", Class: class}, nil, "mongo")
	require.NoError(t, err)
	doc, err := yaml.Marshal(manifest)
	require.NoError(t, err)

	// A directory of this module, so the package builds with its module graph.
	tmp, err := os.MkdirTemp(".", "zz-shipped-")
	require.NoError(t, err)
	dir := filepath.Base(tmp)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	defDir := filepath.Join(dir, "definition")
	require.NoError(t, os.MkdirAll(filepath.Join(defDir, "backupclasses"), 0o755))
	require.NoError(t, writeShippedBackupClasses(defDir, [][]byte{doc}))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main_test.go"), []byte(`package main_test

import (
	"testing"

	"github.com/openeverest/provider-sdk/internal/generate/`+dir+`/definition/backupclasses"
)

func TestShipped(t *testing.T) {
	classes := backupclasses.Shipped()
	if len(classes) != 1 || classes[0].Spec.ExecutorProvider() != "mongo" {
		t.Fatalf("shipped %+v", classes)
	}
	if got := backupclasses.TLSSupport("pbm", "s3"); !got.CustomCA || got.SkipVerify {
		t.Fatalf("TLSSupport(pbm, s3) = %+v", got)
	}
	if got := backupclasses.TLSSupport("pbm", "swift"); got.CustomCA || got.SkipVerify {
		t.Fatalf("TLSSupport(pbm, swift) = %+v", got)
	}
}
`), 0o600))

	out, err := exec.CommandContext(t.Context(), "go", "test", "./"+dir+"/...").CombinedOutput()
	require.NoError(t, err, string(out))
}

// roundTrip normalises a YAML-shaped value through JSON, so numbers and maps
// compare equal whatever produced them.
func roundTrip(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	var out any
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

// TestResolveSchemasSkipsShippedBackupClasses runs schema resolution over a
// definition directory that already holds the generated backupclasses
// package, as every generate after the first does.
func TestResolveSchemasSkipsShippedBackupClasses(t *testing.T) {
	t.Parallel()

	tmp, err := os.MkdirTemp("testdata", "zz-definition-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "types.go"), []byte(`package definition

// Parameters are parameters.
type Parameters struct {
	// Size is a size.
	Size int32 `+"`json:\"size\"`"+`
}
`), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "backupclasses"), 0o755))
	require.NoError(t, writeShippedBackupClasses(tmp, [][]byte{[]byte("kind: BackupClass\nmetadata: {name: a}\n")}))

	schemas, err := ResolveSchemas([]string{"./" + tmp + "/..."}, []string{"Parameters"})
	require.NoError(t, err)
	assert.Contains(t, schemas, "Parameters")
}
