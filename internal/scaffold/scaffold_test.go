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

func TestScaffold(t *testing.T) {
	outputDir := t.TempDir()
	// TempDir creates the dir, but Scaffold expects it not to exist.
	// Use a subdirectory.
	dest := filepath.Join(outputDir, "provider-test-db")

	cfg := &Config{
		ProviderName: "provider-test-db",
		ModulePath:   "github.com/example/provider-test-db",
	}

	err := Scaffold(cfg, dest)
	require.NoError(t, err)

	// Verify go.mod was created (renamed from go.mod.tmpl).
	goMod := filepath.Join(dest, "go.mod")
	_, err = os.Stat(goMod)
	assert.NoError(t, err, "go.mod not found")

	// Verify go.mod.tmpl does NOT exist.
	goModTmpl := filepath.Join(dest, "go.mod.tmpl")
	_, err = os.Stat(goModTmpl)
	assert.Error(t, err, "go.mod.tmpl should not exist in output (should be renamed to go.mod)")

	// Verify placeholder substitution in go.mod.
	content, err := os.ReadFile(goMod)
	require.NoError(t, err)
	assert.Contains(t, string(content), "module github.com/example/provider-test-db")

	// Verify chart directory was renamed.
	chartDir := filepath.Join(dest, "charts", "provider-test-db")
	_, err = os.Stat(chartDir)
	assert.NoError(t, err, "chart directory not renamed")

	// Verify __PROVIDER_NAME__ directory does NOT exist.
	placeholderDir := filepath.Join(dest, "charts", "__PROVIDER_NAME__")
	_, err = os.Stat(placeholderDir)
	assert.Error(t, err, "placeholder directory __PROVIDER_NAME__ should not exist in output")

	// Verify __TOPOLOGY_NAME__ directory does NOT exist.
	topoPlaceholderDir := filepath.Join(dest, "definition", "topologies", "__TOPOLOGY_NAME__")
	_, err = os.Stat(topoPlaceholderDir)
	assert.Error(t, err, "placeholder directory __TOPOLOGY_NAME__ should not exist in output")

	// Verify no unresolved Go template directives remain in any file.
	// Template files use [[ .Field ]] syntax; after rendering all should be gone.
	err = filepath.WalkDir(dest, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		assert.NotContains(t, string(data), "[[ .", "unresolved Go template directive in %s", path)
		return nil
	})
	require.NoError(t, err, "walking output directory")

	// Verify Go package derivation.
	specFile := filepath.Join(dest, "internal", "common", "spec.go")
	specContent, err := os.ReadFile(specFile)
	require.NoError(t, err)
	assert.Contains(t, string(specContent), "ProviderName = \"provider-test-db\"")

	// Verify derived GoPackage was substituted.
	genFile := filepath.Join(dest, "gen.go")
	genContent, err := os.ReadFile(genFile)
	require.NoError(t, err)
	assert.Contains(t, string(genContent), "package providertestdb")
	assert.Contains(t, string(genContent), "go tool provider-sdk generate")

	// Verify definition/ directory structure exists (no topology files by default).
	for _, defFile := range []string{
		"definition/provider.yaml",
		"definition/versions.yaml",
		"definition/types.go",
		"definition/README.md",
		"definition/components/types.go",
	} {
		_, err := os.Stat(filepath.Join(dest, defFile))
		assert.NoError(t, err, "definition file %s not found", defFile)
	}

	// Verify NO topology directory is created when none is specified.
	toposDir := filepath.Join(dest, "definition", "topologies")
	entries, err := os.ReadDir(toposDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "expected empty topologies/ directory")

	// Verify definition/provider.yaml has correct provider name and empty components.
	providerYAML, err := os.ReadFile(filepath.Join(dest, "definition", "provider.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(providerYAML), "name: provider-test-db")
	assert.NotContains(t, string(providerYAML), "\n  engine:", "definition/provider.yaml should not have a hardcoded engine component")
	assert.Contains(t, string(providerYAML), "components: {}", "definition/provider.yaml should have an empty components map")

	// Verify generated/provider-spec.yaml exists in chart.
	providerSpec := filepath.Join(chartDir, "generated", "provider-spec.yaml")
	_, err = os.Stat(providerSpec)
	assert.NoError(t, err, "generated/provider-spec.yaml not found in chart")

	// Verify old flat files do NOT exist.
	for _, oldFile := range []string{"provider-config.yaml", "provider.yaml"} {
		_, err := os.Stat(filepath.Join(dest, oldFile))
		assert.Error(t, err, "old file %s should not exist in output", oldFile)
	}
	_, err = os.Stat(filepath.Join(dest, "types"))
	assert.Error(t, err, "old types/ directory should not exist in output")

	// Verify go.mod has provider-sdk tool dependency.
	assert.Contains(t, string(content), "tool github.com/openeverest/provider-sdk")

	// Verify file count.
	count, err := CountFiles(dest)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 35, "expected at least 35 files")

	// Verify README has correct content.
	readme, err := os.ReadFile(filepath.Join(dest, "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readme), "# provider-test-db")

	// Verify dotfiles are present.
	for _, dotfile := range []string{".gitignore", ".dockerignore"} {
		_, err := os.Stat(filepath.Join(dest, dotfile))
		assert.NoError(t, err, "dotfile %s not found", dotfile)
	}

	// Verify .github/workflows/ are present.
	workflowDir := filepath.Join(dest, ".github", "workflows")
	_, err = os.Stat(workflowDir)
	assert.NoError(t, err, ".github/workflows/ not found")

	// Verify executable permissions on .sh files.
	varsSh := filepath.Join(dest, "test", "vars.sh")
	info, err := os.Stat(varsSh)
	require.NoError(t, err, "test/vars.sh not found")
	assert.NotZero(t, info.Mode()&0o111, "test/vars.sh should be executable")
}

func TestScaffoldOutputDirExists(t *testing.T) {
	outputDir := t.TempDir()

	cfg := &Config{
		ProviderName: "test",
		ModulePath:   "github.com/example/test",
	}

	err := Scaffold(cfg, outputDir)
	require.Error(t, err, "expected error when output directory exists")
	assert.Contains(t, err.Error(), "already exists")
}

func TestScaffoldValidation(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "out")

	tests := []struct {
		name string
		cfg  Config
	}{
		{"missing name", Config{ModulePath: "x"}},
		{"missing module", Config{ProviderName: "x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			err := Scaffold(&cfg, dest)
			assert.Error(t, err, "expected validation error")
		})
	}
}

func TestScaffoldCustomTopologyName(t *testing.T) {
	outputDir := t.TempDir()
	dest := filepath.Join(outputDir, "provider-test")

	cfg := &Config{
		ProviderName: "provider-test",
		ModulePath:   "github.com/example/provider-test",
		TopologyName: "replicaSet",
	}

	err := Scaffold(cfg, dest)
	require.NoError(t, err)

	// Verify the custom topology directory was created.
	topoDir := filepath.Join(dest, "definition", "topologies", "replicaSet")
	_, err = os.Stat(topoDir)
	assert.NoError(t, err, "custom topology directory not created")

	// Verify the default standalone does NOT exist.
	standaloneDir := filepath.Join(dest, "definition", "topologies", "standalone")
	_, err = os.Stat(standaloneDir)
	assert.Error(t, err, "default standalone topology should not exist when custom topology is specified")

	// Verify topology types.go has correct package and type name.
	typesFile := filepath.Join(topoDir, "types.go")
	typesContent, err := os.ReadFile(typesFile)
	require.NoError(t, err)
	assert.Contains(t, string(typesContent), "package replicaset")
	assert.Contains(t, string(typesContent), "ReplicaSetTopologyParameters")

	// Verify provider-spec.yaml exists in chart (content is a placeholder).
	specFile := filepath.Join(dest, "charts", "provider-test", "generated", "provider-spec.yaml")
	_, err = os.Stat(specFile)
	assert.NoError(t, err, "generated/provider-spec.yaml not found in chart")
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"standalone", "Standalone"},
		{"replicaSet", "ReplicaSet"},
		{"sharded", "Sharded"},
		{"replica-set", "ReplicaSet"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
