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
)

func TestAssembleSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	secretDir := filepath.Join(tmpDir, "secrets", "credentials")
	if err := os.MkdirAll(secretDir, 0o755); err != nil {
		t.Fatalf("creating secrets directory: %v", err)
	}

	defYAML := "parametersSchema:\n  openAPIV3Schema: CredentialsSecretData\n"
	if err := os.WriteFile(filepath.Join(secretDir, "definition.yaml"), []byte(defYAML), 0o644); err != nil {
		t.Fatalf("writing definition.yaml: %v", err)
	}

	uiYAML := "label: Credentials Secret\ncomponentsOrder:\n  - username\n"
	if err := os.WriteFile(filepath.Join(secretDir, "ui.yaml"), []byte(uiYAML), 0o644); err != nil {
		t.Fatalf("writing ui.yaml: %v", err)
	}

	typeRefs := make(map[string]bool)
	secrets, err := AssembleSecrets(tmpDir, typeRefs)
	if err != nil {
		t.Fatalf("AssembleSecrets() error: %v", err)
	}
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}
	if !typeRefs["CredentialsSecretData"] {
		t.Error("type reference not collected")
	}
}

func TestAssembleSecrets_NoDir(t *testing.T) {
	tmpDir := t.TempDir()
	typeRefs := make(map[string]bool)
	secrets, err := AssembleSecrets(tmpDir, typeRefs)
	if err != nil {
		t.Fatalf("AssembleSecrets() error: %v", err)
	}
	if len(secrets) != 0 {
		t.Errorf("expected 0 secrets, got %d", len(secrets))
	}
}

func TestAssembleConfigMaps(t *testing.T) {
	tmpDir := t.TempDir()
	cmDir := filepath.Join(tmpDir, "configmaps", "app-config")
	if err := os.MkdirAll(cmDir, 0o755); err != nil {
		t.Fatalf("creating configmaps directory: %v", err)
	}

	defYAML := "parametersSchema:\n  openAPIV3Schema: AppConfigData\n"
	if err := os.WriteFile(filepath.Join(cmDir, "definition.yaml"), []byte(defYAML), 0o644); err != nil {
		t.Fatalf("writing definition.yaml: %v", err)
	}

	typeRefs := make(map[string]bool)
	cms, err := AssembleConfigMaps(tmpDir, typeRefs)
	if err != nil {
		t.Fatalf("AssembleConfigMaps() error: %v", err)
	}
	if len(cms) != 1 {
		t.Fatalf("expected 1 configmap, got %d", len(cms))
	}
	if !typeRefs["AppConfigData"] {
		t.Error("type reference not collected")
	}
}

func TestAssembleConfigMaps_NoDir(t *testing.T) {
	tmpDir := t.TempDir()
	typeRefs := make(map[string]bool)
	cms, err := AssembleConfigMaps(tmpDir, typeRefs)
	if err != nil {
		t.Fatalf("AssembleConfigMaps() error: %v", err)
	}
	if len(cms) != 0 {
		t.Errorf("expected 0 configmaps, got %d", len(cms))
	}
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
	if spec == nil {
		t.Fatal("buildSecretsSpec returned nil")
	}
	creds, ok := spec["creds"].(map[string]any)
	if !ok {
		t.Fatal("creds not found")
	}
	if _, ok := creds["parametersSchema"]; !ok {
		t.Error("parametersSchema not found")
	}
	if _, ok := creds["uiSchema"]; !ok {
		t.Error("uiSchema not found")
	}
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
	if spec == nil {
		t.Fatal("buildConfigMapsSpec returned nil")
	}
	cfg, ok := spec["cfg"].(map[string]any)
	if !ok {
		t.Fatal("cfg not found")
	}
	if _, ok := cfg["parametersSchema"]; !ok {
		t.Error("parametersSchema not found")
	}
	if _, ok := cfg["uiSchema"]; !ok {
		t.Error("uiSchema not found")
	}
}
