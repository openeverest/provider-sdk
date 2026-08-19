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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddSecret(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal provider project structure.
	defDir := filepath.Join(tmpDir, "definition")
	err := os.MkdirAll(defDir, 0o755)
	require.NoError(t, err)
	providerYAML := filepath.Join(defDir, "provider.yaml")
	err = os.WriteFile(providerYAML, []byte("name: test-provider\n"), 0o644)
	require.NoError(t, err)

	// Change to the temp directory to simulate running from project root.
	oldWD, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer os.Chdir(oldWD)

	// Run AddSecret.
	cfg := &AddSecretConfig{
		Name: "credentials",
	}
	err = AddSecret(cfg)
	require.NoError(t, err)

	// Verify files were created.
	secretDir := filepath.Join(defDir, "secrets", "credentials")

	defFile := filepath.Join(secretDir, "definition.yaml")
	_, err = os.Stat(defFile)
	assert.NoError(t, err, "definition.yaml not created")

	uiFile := filepath.Join(secretDir, "ui.yaml")
	_, err = os.Stat(uiFile)
	assert.NoError(t, err, "ui.yaml not created")

	typesFile := filepath.Join(secretDir, "types.go")
	_, err = os.Stat(typesFile)
	assert.NoError(t, err, "types.go not created")

	// Verify definition.yaml content.
	defContent, err := os.ReadFile(defFile)
	require.NoError(t, err)
	assert.Contains(t, string(defContent), "CredentialsSecretData")

	// Verify types.go content.
	typesContent, err := os.ReadFile(typesFile)
	require.NoError(t, err)
	assert.Contains(t, string(typesContent), "type CredentialsSecretData struct")
	assert.Contains(t, string(typesContent), "package credentials")

	// Verify that running again fails (directory exists).
	err = AddSecret(cfg)
	assert.Error(t, err, "AddSecret() should fail when secret already exists")
}

func TestAddSecret_EmptyName(t *testing.T) {
	cfg := &AddSecretConfig{
		Name: "",
	}

	err := AddSecret(cfg)
	assert.Error(t, err, "AddSecret() should fail with empty name")
}

func TestAddSecret_NotInProviderProject(t *testing.T) {
	tmpDir := t.TempDir()

	oldWD, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer os.Chdir(oldWD)

	cfg := &AddSecretConfig{
		Name: "test-secret",
	}
	err = AddSecret(cfg)
	assert.Error(t, err, "AddSecret() should fail when not in provider project")
}

func TestAddConfigMap(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal provider project structure.
	defDir := filepath.Join(tmpDir, "definition")
	err := os.MkdirAll(defDir, 0o755)
	require.NoError(t, err)
	providerYAML := filepath.Join(defDir, "provider.yaml")
	err = os.WriteFile(providerYAML, []byte("name: test-provider\n"), 0o644)
	require.NoError(t, err)

	// Change to the temp directory to simulate running from project root.
	oldWD, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer os.Chdir(oldWD)

	// Run AddConfigMap.
	cfg := &AddConfigMapConfig{
		Name: "app-config",
	}
	err = AddConfigMap(cfg)
	require.NoError(t, err)

	// Verify files were created.
	cmDir := filepath.Join(defDir, "configmaps", "app-config")

	defFile := filepath.Join(cmDir, "definition.yaml")
	_, err = os.Stat(defFile)
	assert.NoError(t, err, "definition.yaml not created")

	uiFile := filepath.Join(cmDir, "ui.yaml")
	_, err = os.Stat(uiFile)
	assert.NoError(t, err, "ui.yaml not created")

	typesFile := filepath.Join(cmDir, "types.go")
	_, err = os.Stat(typesFile)
	assert.NoError(t, err, "types.go not created")

	// Verify definition.yaml content.
	defContent, err := os.ReadFile(defFile)
	require.NoError(t, err)
	assert.Contains(t, string(defContent), "AppConfigConfigMapData")

	// Verify types.go content.
	typesContent, err := os.ReadFile(typesFile)
	require.NoError(t, err)
	assert.Contains(t, string(typesContent), "type AppConfigConfigMapData struct")
	assert.Contains(t, string(typesContent), "package appconfig")

	// Verify that running again fails (directory exists).
	err = AddConfigMap(cfg)
	assert.Error(t, err, "AddConfigMap() should fail when configmap already exists")
}

func TestAddConfigMap_EmptyName(t *testing.T) {
	cfg := &AddConfigMapConfig{
		Name: "",
	}
	err := AddConfigMap(cfg)
	assert.Error(t, err, "AddConfigMap() should fail with empty name")
}

func TestAddConfigMap_NotInProviderProject(t *testing.T) {
	tmpDir := t.TempDir()

	oldWD, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer os.Chdir(oldWD)

	cfg := &AddConfigMapConfig{
		Name: "test-config",
	}
	err = AddConfigMap(cfg)
	assert.Error(t, err, "AddConfigMap() should fail when not in provider project")
}
