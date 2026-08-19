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

func TestAssembleSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	secretDir := filepath.Join(tmpDir, "secrets", "credentials")
	err := os.MkdirAll(secretDir, 0o755)
	require.NoError(t, err)

	defYAML := "parametersSchema:\n  openAPIV3Schema: CredentialsSecretData\n"
	err = os.WriteFile(filepath.Join(secretDir, "definition.yaml"), []byte(defYAML), 0o644)
	require.NoError(t, err)

	uiYAML := "label: Credentials Secret\ncomponentsOrder:\n  - username\n"
	err = os.WriteFile(filepath.Join(secretDir, "ui.yaml"), []byte(uiYAML), 0o644)
	require.NoError(t, err)

	typeRefs := make(map[string]bool)
	secrets, err := AssembleSecrets(tmpDir, typeRefs)
	require.NoError(t, err)
	require.Len(t, secrets, 1)
	assert.True(t, typeRefs["CredentialsSecretData"], "type reference not collected")
}

func TestAssembleSecrets_NoDir(t *testing.T) {
	tmpDir := t.TempDir()
	typeRefs := make(map[string]bool)
	secrets, err := AssembleSecrets(tmpDir, typeRefs)
	require.NoError(t, err)
	assert.Empty(t, secrets)
}

func TestAssembleConfigMaps(t *testing.T) {
	tmpDir := t.TempDir()
	cmDir := filepath.Join(tmpDir, "configmaps", "app-config")
	err := os.MkdirAll(cmDir, 0o755)
	require.NoError(t, err)

	defYAML := "parametersSchema:\n  openAPIV3Schema: AppConfigData\n"
	err = os.WriteFile(filepath.Join(cmDir, "definition.yaml"), []byte(defYAML), 0o644)
	require.NoError(t, err)

	typeRefs := make(map[string]bool)
	cms, err := AssembleConfigMaps(tmpDir, typeRefs)
	require.NoError(t, err)
	require.Len(t, cms, 1)
	assert.True(t, typeRefs["AppConfigData"], "type reference not collected")
}

func TestAssembleConfigMaps_NoDir(t *testing.T) {
	tmpDir := t.TempDir()
	typeRefs := make(map[string]bool)
	cms, err := AssembleConfigMaps(tmpDir, typeRefs)
	require.NoError(t, err)
	assert.Empty(t, cms)
}

func TestBuildSecretsSpec(t *testing.T) {
	secrets := []SecretDefinitionAssembled{{
		Name: "creds",
		Definition: map[string]any{
			"parametersSchema": map[string]any{
				"openAPIV3Schema": "CredsData",
			},
		},
		UI: map[string]any{"label": "Creds"},
	}}
	schemas := map[string]any{
		"CredsData": map[string]any{"type": "object"},
	}
	spec := buildSecretsSpec(secrets, schemas)
	require.NotNil(t, spec)
	creds, ok := spec["creds"].(map[string]any)
	require.True(t, ok, "creds not found")
	assert.Contains(t, creds, "parametersSchema")
	assert.Contains(t, creds, "uiSchema")
}

func TestBuildConfigMapsSpec(t *testing.T) {
	cms := []ConfigMapDefinitionAssembled{{
		Name: "cfg",
		Definition: map[string]any{
			"parametersSchema": map[string]any{
				"openAPIV3Schema": "CfgData",
			},
		},
		UI: map[string]any{"label": "Cfg"},
	}}
	schemas := map[string]any{
		"CfgData": map[string]any{"type": "object"},
	}
	spec := buildConfigMapsSpec(cms, schemas)
	require.NotNil(t, spec)
	cfg, ok := spec["cfg"].(map[string]any)
	require.True(t, ok, "cfg not found")
	assert.Contains(t, cfg, "parametersSchema")
	assert.Contains(t, cfg, "uiSchema")
}
