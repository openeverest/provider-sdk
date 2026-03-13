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

// Package scaffold implements the provider project scaffolding engine.
//
// It uses an embedded filesystem containing the provider template files from
// the _template/ directory and executes Go templates to generate a new provider
// project. The underscore prefix causes the Go toolchain to skip _template/
// during compilation, so the template Go sources are never built directly.
//
// Template files use [[ ]] as delimiters (instead of the default {{ }}) to
// avoid conflicts with Helm chart templates that use {{ }} natively.
package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// templateFS contains the embedded provider template files.
// The "all:" prefix ensures dotfiles (.gitignore, .github/, .helmignore, etc.)
// are included. The leading underscore causes `go build ./...` to skip the
// directory, preventing it from trying to compile the template Go sources.
//
//go:embed all:_template
var templateFS embed.FS

const templateRoot = "_template"

// Config holds the configuration for scaffolding a new provider project.
type Config struct {
	// ProviderName is the name of the provider (e.g., "provider-percona-server-mongodb").
	ProviderName string

	// ModulePath is the Go module path (e.g., "github.com/openeverest/provider-percona-server-mongodb").
	ModulePath string

	// UpstreamAPIGroup is the API group of the upstream operator CRDs (e.g., "psmdb.percona.com").
	// Optional: used as a hint in RBAC marker comments.
	UpstreamAPIGroup string

	// UpstreamResource is the plural name of the upstream operator's primary resource
	// (e.g., "perconaservermongodbs").
	// Optional: used as a hint in RBAC marker comments.
	UpstreamResource string

	// ComponentType is the primary component type name (e.g., "mongod").
	ComponentType string

	// TopologyName is the name of the initial topology (e.g., "standalone", "replicaSet").
	// Default: "standalone".
	TopologyName string

	// GoPackage is the Go package name for the root package.
	// If empty, it is derived from ProviderName by stripping hyphens and lowercasing.
	GoPackage string

	// TopologyPackage is the Go package name for the topology (e.g., "standalone", "replicaset").
	// Derived from TopologyName.
	TopologyPackage string

	// TopologyTypeName is the PascalCase name for the topology config struct (e.g., "Standalone", "ReplicaSet").
	// Derived from TopologyName.
	TopologyTypeName string
}

// Validate checks that all required fields are set.
func (c *Config) Validate() error {
	var missing []string
	if c.ProviderName == "" {
		missing = append(missing, "ProviderName")
	}
	if c.ModulePath == "" {
		missing = append(missing, "ModulePath")
	}
	// ComponentType, TopologyName, UpstreamAPIGroup and UpstreamResource are optional.
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

// DeriveDefaults fills in optional fields from required ones.
func (c *Config) DeriveDefaults() {
	if c.GoPackage == "" {
		c.GoPackage = strings.ToLower(strings.ReplaceAll(c.ProviderName, "-", ""))
	}
	// TopologyName intentionally left empty when not provided — the topology
	// template directory is skipped during scaffolding in that case.
	if c.TopologyName != "" {
		if c.TopologyPackage == "" {
			c.TopologyPackage = strings.ToLower(c.TopologyName)
		}
		if c.TopologyTypeName == "" {
			c.TopologyTypeName = toPascalCase(c.TopologyName)
		}
	}
	if c.UpstreamAPIGroup == "" {
		c.UpstreamAPIGroup = "<upstream-api-group>"
	}
	if c.UpstreamResource == "" {
		c.UpstreamResource = "<upstream-resources>"
	}
}

// toPascalCase converts a string to PascalCase.
// Handles camelCase ("replicaSet" → "ReplicaSet") and kebab-case ("replica-set" → "ReplicaSet").
func toPascalCase(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "-")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// pathReplacer returns a Replacer that substitutes provider-name placeholder
// tokens in file-system path segments (directory and file names).
//
// Only ProviderName is used in path segments; all other values are substituted
// inside file contents via the Go template engine.
func (c *Config) pathReplacer() *strings.Replacer {
	return strings.NewReplacer(
		"__PROVIDER_NAME__", c.ProviderName,
		"__TOPOLOGY_NAME__", c.TopologyName,
	)
}

// Scaffold generates a new provider project at outputDir from the embedded template.
func Scaffold(cfg *Config, outputDir string) error {
	cfg.DeriveDefaults()

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Check that the output directory does not already exist.
	if info, err := os.Stat(outputDir); err == nil && info.IsDir() {
		return fmt.Errorf("output directory already exists: %s", outputDir)
	}

	pathReplacer := cfg.pathReplacer()

	return fs.WalkDir(templateFS, templateRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the topology template directory when no topology name was given.
		if cfg.TopologyName == "" && strings.Contains(path, "__TOPOLOGY_NAME__") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// Strip the "testdata/template" prefix to get the relative path.
		relPath := strings.TrimPrefix(path, templateRoot)
		if relPath == "" || relPath == "/" {
			return nil
		}
		relPath = strings.TrimPrefix(relPath, "/")

		// Handle special renames.
		relPath = applyRenames(relPath)

		// Substitute the provider-name token in path segments
		// (e.g., charts/__PROVIDER_NAME__/ → charts/provider-foo/).
		outPath := pathReplacer.Replace(relPath)
		destPath := filepath.Join(outputDir, outPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		// Read the template file.
		content, err := fs.ReadFile(templateFS, path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		// Render file content via the Go template engine.
		result, err := renderTemplate(path, string(content), cfg)
		if err != nil {
			return fmt.Errorf("rendering %s: %w", path, err)
		}

		// Ensure parent directory exists.
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}

		// Determine file permissions.
		perm := os.FileMode(0o644)
		if isExecutable(relPath) {
			perm = 0o755
		}

		return os.WriteFile(destPath, []byte(result), perm)
	})
}

// renderTemplate executes a Go text/template against cfg using [[ ]] as
// delimiters. Custom delimiters prevent conflicts with Helm's {{ }} syntax
// that appears in chart templates and NOTES.txt.
func renderTemplate(name, content string, cfg *Config) (string, error) {
	tmpl, err := template.New(name).Delims("[[", "]]").Parse(content)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// applyRenames handles special file renames during scaffolding.
func applyRenames(path string) string {
	// go.mod.tmpl -> go.mod (avoids Go treating the _template as a nested module).
	if path == "go.mod.tmpl" {
		return "go.mod"
	}
	return path
}

// isExecutable returns true for files that should have the executable bit set.
func isExecutable(path string) bool {
	switch filepath.Ext(path) {
	case ".sh", ".py":
		return true
	}
	return false
}

// CountFiles counts the number of regular files under a directory.
func CountFiles(dir string) (int, error) {
	count := 0
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}
