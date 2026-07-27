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

package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddSecret(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal provider project structure.
	defDir := filepath.Join(tmpDir, "definition")
	if err := os.MkdirAll(defDir, 0o755); err != nil {
		t.Fatalf("creating definition directory: %v", err)
	}
	providerYAML := filepath.Join(defDir, "provider.yaml")
	if err := os.WriteFile(providerYAML, []byte("name: test-provider\n"), 0o644); err != nil {
		t.Fatalf("writing provider.yaml: %v", err)
	}

	// Change to the temp directory to simulate running from project root.
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to temp directory: %v", err)
	}
	defer os.Chdir(oldWD)

	// Run AddSecret.
	cfg := &AddSecretConfig{
		Name: "credentials",
	}
	if err := AddSecret(cfg); err != nil {
		t.Fatalf("AddSecret() error: %v", err)
	}

	// Verify files were created.
	secretDir := filepath.Join(defDir, "secrets", "credentials")

	defFile := filepath.Join(secretDir, "definition.yaml")
	if _, err := os.Stat(defFile); err != nil {
		t.Errorf("definition.yaml not created: %v", err)
	}

	uiFile := filepath.Join(secretDir, "ui.yaml")
	if _, err := os.Stat(uiFile); err != nil {
		t.Errorf("ui.yaml not created: %v", err)
	}

	typesFile := filepath.Join(secretDir, "types.go")
	if _, err := os.Stat(typesFile); err != nil {
		t.Errorf("types.go not created: %v", err)
	}

	// Verify definition.yaml content.
	defContent, err := os.ReadFile(defFile)
	if err != nil {
		t.Fatalf("reading definition.yaml: %v", err)
	}
	if !strings.Contains(string(defContent), "CredentialsSecretData") {
		t.Error("definition.yaml does not reference CredentialsSecretData type")
	}

	// Verify types.go content.
	typesContent, err := os.ReadFile(typesFile)
	if err != nil {
		t.Fatalf("reading types.go: %v", err)
	}
	if !strings.Contains(string(typesContent), "type CredentialsSecretData struct") {
		t.Error("types.go does not contain CredentialsSecretData struct")
	}
	if !strings.Contains(string(typesContent), "package credentials") {
		t.Error("types.go does not have correct package name")
	}

	// Verify that running again fails (directory exists).
	if err := AddSecret(cfg); err == nil {
		t.Error("AddSecret() should fail when secret already exists")
	}
}

func TestAddSecret_EmptyName(t *testing.T) {
	cfg := &AddSecretConfig{
		Name: "",
	}
	if err := AddSecret(cfg); err == nil {
		t.Error("AddSecret() should fail with empty name")
	}
}

func TestAddSecret_NotInProviderProject(t *testing.T) {
	tmpDir := t.TempDir()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to temp directory: %v", err)
	}
	defer os.Chdir(oldWD)

	cfg := &AddSecretConfig{
		Name: "test-secret",
	}
	if err := AddSecret(cfg); err == nil {
		t.Error("AddSecret() should fail when not in provider project")
	}
}

func TestAddConfigMap(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal provider project structure.
	defDir := filepath.Join(tmpDir, "definition")
	if err := os.MkdirAll(defDir, 0o755); err != nil {
		t.Fatalf("creating definition directory: %v", err)
	}
	providerYAML := filepath.Join(defDir, "provider.yaml")
	if err := os.WriteFile(providerYAML, []byte("name: test-provider\n"), 0o644); err != nil {
		t.Fatalf("writing provider.yaml: %v", err)
	}

	// Change to the temp directory to simulate running from project root.
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to temp directory: %v", err)
	}
	defer os.Chdir(oldWD)

	// Run AddConfigMap.
	cfg := &AddConfigMapConfig{
		Name: "app-config",
	}
	if err := AddConfigMap(cfg); err != nil {
		t.Fatalf("AddConfigMap() error: %v", err)
	}

	// Verify files were created.
	cmDir := filepath.Join(defDir, "configmaps", "app-config")

	defFile := filepath.Join(cmDir, "definition.yaml")
	if _, err := os.Stat(defFile); err != nil {
		t.Errorf("definition.yaml not created: %v", err)
	}

	uiFile := filepath.Join(cmDir, "ui.yaml")
	if _, err := os.Stat(uiFile); err != nil {
		t.Errorf("ui.yaml not created: %v", err)
	}

	typesFile := filepath.Join(cmDir, "types.go")
	if _, err := os.Stat(typesFile); err != nil {
		t.Errorf("types.go not created: %v", err)
	}

	// Verify definition.yaml content.
	defContent, err := os.ReadFile(defFile)
	if err != nil {
		t.Fatalf("reading definition.yaml: %v", err)
	}
	if !strings.Contains(string(defContent), "AppConfigConfigMapData") {
		t.Error("definition.yaml does not reference AppConfigConfigMapData type")
	}

	// Verify types.go content.
	typesContent, err := os.ReadFile(typesFile)
	if err != nil {
		t.Fatalf("reading types.go: %v", err)
	}
	if !strings.Contains(string(typesContent), "type AppConfigConfigMapData struct") {
		t.Error("types.go does not contain AppConfigConfigMapData struct")
	}
	if !strings.Contains(string(typesContent), "package appconfig") {
		t.Error("types.go does not have correct package name")
	}

	// Verify that running again fails (directory exists).
	if err := AddConfigMap(cfg); err == nil {
		t.Error("AddConfigMap() should fail when configmap already exists")
	}
}

func TestAddConfigMap_EmptyName(t *testing.T) {
	cfg := &AddConfigMapConfig{
		Name: "",
	}
	if err := AddConfigMap(cfg); err == nil {
		t.Error("AddConfigMap() should fail with empty name")
	}
}

func TestAddConfigMap_NotInProviderProject(t *testing.T) {
	tmpDir := t.TempDir()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to temp directory: %v", err)
	}
	defer os.Chdir(oldWD)

	cfg := &AddConfigMapConfig{
		Name: "test-config",
	}
	if err := AddConfigMap(cfg); err == nil {
		t.Error("AddConfigMap() should fail when not in provider project")
	}
}
