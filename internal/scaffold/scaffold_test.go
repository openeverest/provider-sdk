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

func TestScaffold(t *testing.T) {
	outputDir := t.TempDir()
	// TempDir creates the dir, but Scaffold expects it not to exist.
	// Use a subdirectory.
	dest := filepath.Join(outputDir, "provider-test-db")

	cfg := &Config{
		ProviderName: "provider-test-db",
		ModulePath:   "github.com/example/provider-test-db",
	}

	if err := Scaffold(cfg, dest); err != nil {
		t.Fatalf("Scaffold() error: %v", err)
	}

	// Verify go.mod was created (renamed from go.mod.tmpl).
	goMod := filepath.Join(dest, "go.mod")
	if _, err := os.Stat(goMod); err != nil {
		t.Errorf("go.mod not found: %v", err)
	}

	// Verify go.mod.tmpl does NOT exist.
	goModTmpl := filepath.Join(dest, "go.mod.tmpl")
	if _, err := os.Stat(goModTmpl); err == nil {
		t.Error("go.mod.tmpl should not exist in output (should be renamed to go.mod)")
	}

	// Verify placeholder substitution in go.mod.
	content, err := os.ReadFile(goMod)
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	if !strings.Contains(string(content), "module github.com/example/provider-test-db") {
		t.Error("go.mod does not contain expected module path")
	}

	// Verify chart directory was renamed.
	chartDir := filepath.Join(dest, "charts", "provider-test-db")
	if _, err := os.Stat(chartDir); err != nil {
		t.Errorf("chart directory not renamed: %v", err)
	}

	// Verify __PROVIDER_NAME__ directory does NOT exist.
	placeholderDir := filepath.Join(dest, "charts", "__PROVIDER_NAME__")
	if _, err := os.Stat(placeholderDir); err == nil {
		t.Error("placeholder directory __PROVIDER_NAME__ should not exist in output")
	}

	// Verify __TOPOLOGY_NAME__ directory does NOT exist.
	topoPlaceholderDir := filepath.Join(dest, "definition", "topologies", "__TOPOLOGY_NAME__")
	if _, err := os.Stat(topoPlaceholderDir); err == nil {
		t.Error("placeholder directory __TOPOLOGY_NAME__ should not exist in output")
	}

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
		if strings.Contains(string(data), "[[ .") {
			t.Errorf("unresolved Go template directive in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking output directory: %v", err)
	}

	// Verify Go package derivation.
	specFile := filepath.Join(dest, "internal", "common", "spec.go")
	specContent, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatalf("reading spec.go: %v", err)
	}
	if !strings.Contains(string(specContent), "ProviderName = \"provider-test-db\"") {
		t.Error("spec.go does not contain expected ProviderName constant")
	}

	// Verify derived GoPackage was substituted.
	genFile := filepath.Join(dest, "gen.go")
	genContent, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatalf("reading gen.go: %v", err)
	}
	if !strings.Contains(string(genContent), "package providertestdb") {
		t.Error("gen.go does not contain derived package name providertestdb")
	}
	if !strings.Contains(string(genContent), "go tool provider-sdk generate") {
		t.Error("gen.go does not contain provider-sdk generate directive")
	}

	// Verify definition/ directory structure exists (no topology files by default).
	for _, defFile := range []string{
		"definition/provider.yaml",
		"definition/versions.yaml",
		"definition/types.go",
		"definition/README.md",
		"definition/PROVIDER_DEVELOPMENT.md",
		"definition/components/types.go",
	} {
		if _, err := os.Stat(filepath.Join(dest, defFile)); err != nil {
			t.Errorf("definition file %s not found: %v", defFile, err)
		}
	}

	// Verify NO topology directory is created when none is specified.
	toposDir := filepath.Join(dest, "definition", "topologies")
	if entries, err := os.ReadDir(toposDir); err == nil && len(entries) > 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected empty topologies/ directory, found: %v", names)
	}

	// Verify definition/provider.yaml has correct provider name and empty components.
	providerYAML, err := os.ReadFile(filepath.Join(dest, "definition", "provider.yaml"))
	if err != nil {
		t.Fatalf("reading definition/provider.yaml: %v", err)
	}
	if !strings.Contains(string(providerYAML), "name: provider-test-db") {
		t.Error("definition/provider.yaml does not contain expected provider name")
	}
	if strings.Contains(string(providerYAML), "\n  engine:") {
		t.Error("definition/provider.yaml should not have a hardcoded engine component")
	}
	if !strings.Contains(string(providerYAML), "components: {}") {
		t.Error("definition/provider.yaml should have an empty components map")
	}

	// Verify generated/provider-spec.yaml exists in chart.
	providerSpec := filepath.Join(chartDir, "generated", "provider-spec.yaml")
	if _, err := os.Stat(providerSpec); err != nil {
		t.Errorf("generated/provider-spec.yaml not found in chart: %v", err)
	}

	// Verify old flat files do NOT exist.
	for _, oldFile := range []string{"provider-config.yaml", "provider.yaml"} {
		if _, err := os.Stat(filepath.Join(dest, oldFile)); err == nil {
			t.Errorf("old file %s should not exist in output", oldFile)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "types")); err == nil {
		t.Error("old types/ directory should not exist in output")
	}

	// Verify go.mod has provider-sdk tool dependency.
	if !strings.Contains(string(content), "tool github.com/openeverest/provider-sdk") {
		t.Error("go.mod does not contain provider-sdk tool dependency")
	}

	// Verify file count.
	count, err := CountFiles(dest)
	if err != nil {
		t.Fatalf("CountFiles() error: %v", err)
	}
	if count < 35 {
		t.Errorf("expected at least 35 files, got %d", count)
	}

	// Verify README has correct content.
	readme, err := os.ReadFile(filepath.Join(dest, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	if !strings.Contains(string(readme), "# provider-test-db") {
		t.Error("README.md does not contain expected heading")
	}

	// Verify dotfiles are present.
	for _, dotfile := range []string{".gitignore", ".dockerignore"} {
		if _, err := os.Stat(filepath.Join(dest, dotfile)); err != nil {
			t.Errorf("dotfile %s not found: %v", dotfile, err)
		}
	}

	// Verify .github/workflows/ are present.
	workflowDir := filepath.Join(dest, ".github", "workflows")
	if _, err := os.Stat(workflowDir); err != nil {
		t.Errorf(".github/workflows/ not found: %v", err)
	}

	// Verify executable permissions on .sh files.
	varsSh := filepath.Join(dest, "test", "vars.sh")
	info, err := os.Stat(varsSh)
	if err != nil {
		t.Fatalf("test/vars.sh not found: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("test/vars.sh should be executable")
	}
}

func TestScaffoldOutputDirExists(t *testing.T) {
	outputDir := t.TempDir()

	cfg := &Config{
		ProviderName: "test",
		ModulePath:   "github.com/example/test",
	}

	err := Scaffold(cfg, outputDir)
	if err == nil {
		t.Error("expected error when output directory exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error message: %v", err)
	}
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
			if err == nil {
				t.Error("expected validation error")
			}
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

	if err := Scaffold(cfg, dest); err != nil {
		t.Fatalf("Scaffold() error: %v", err)
	}

	// Verify the custom topology directory was created.
	topoDir := filepath.Join(dest, "definition", "topologies", "replicaSet")
	if _, err := os.Stat(topoDir); err != nil {
		t.Errorf("custom topology directory not created: %v", err)
	}

	// Verify the default standalone does NOT exist.
	standaloneDir := filepath.Join(dest, "definition", "topologies", "standalone")
	if _, err := os.Stat(standaloneDir); err == nil {
		t.Error("default standalone topology should not exist when custom topology is specified")
	}

	// Verify topology types.go has correct package and type name.
	typesFile := filepath.Join(topoDir, "types.go")
	typesContent, err := os.ReadFile(typesFile)
	if err != nil {
		t.Fatalf("reading topology types.go: %v", err)
	}
	if !strings.Contains(string(typesContent), "package replicaset") {
		t.Error("topology types.go does not contain correct package name 'replicaset'")
	}
	if !strings.Contains(string(typesContent), "ReplicaSetTopologyConfig") {
		t.Error("topology types.go does not contain correct type name 'ReplicaSetTopologyConfig'")
	}

	// Verify provider-spec.yaml exists in chart (content is a placeholder).
	specFile := filepath.Join(dest, "charts", "provider-test", "generated", "provider-spec.yaml")
	if _, err := os.Stat(specFile); err != nil {
		t.Errorf("generated/provider-spec.yaml not found in chart: %v", err)
	}
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
			if result != tt.expected {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
