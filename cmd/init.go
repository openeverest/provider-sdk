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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openeverest/provider-sdk/internal/scaffold"
	"github.com/openeverest/provider-sdk/internal/tui"
)

var initOpts struct {
	name           string
	modulePath     string
	apiGroup       string
	resource       string
	outputDir      string
	nonInteractive bool
}

func init() {
	f := initCmd.Flags()
	f.StringVar(&initOpts.name, "name", "", "Provider name (e.g., provider-my-database)")
	f.StringVar(&initOpts.modulePath, "module", "", "Go module path (e.g., github.com/my-org/provider-my-database)")
	f.StringVar(&initOpts.apiGroup, "api-group", "", "Upstream operator API group (optional, used as RBAC hint)")
	f.StringVar(&initOpts.resource, "resource", "", "Upstream operator resource, plural (optional, used as RBAC hint)")
	f.StringVarP(&initOpts.outputDir, "output-dir", "o", "", "Output directory (default: ./<name>)")
	f.BoolVar(&initOpts.nonInteractive, "non-interactive", false, "Fail instead of prompting for missing values")

	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new provider project",
	Long: `Create a new OpenEverest provider project from the built-in template.

The command prompts for required values interactively. Pass --non-interactive
with all required flags for CI/automation usage.

After scaffolding, use 'provider-sdk add component' and 'provider-sdk add topology'
to define the provider's components and topologies.

Example:
  provider-sdk init \\
    --name provider-my-database \\
    --module github.com/my-org/provider-my-database`,
	RunE: runInit,
}

func runInit(_ *cobra.Command, _ []string) error {
	fmt.Println()
	fmt.Println("=== OpenEverest Provider Scaffolding ===")
	fmt.Println()

	if initOpts.nonInteractive {
		// In non-interactive mode all required values must come from flags.
		var missing []string
		if initOpts.name == "" {
			missing = append(missing, "--name")
		}
		if initOpts.modulePath == "" {
			missing = append(missing, "--module")
		}
		if len(missing) > 0 {
			return fmt.Errorf("required flags not set in non-interactive mode: %s", strings.Join(missing, ", "))
		}
	} else {
		if err := promptTUI(&initOpts.name, "Provider name", "provider-my-database", "", true); err != nil {
			return err
		}
		if err := promptTUI(&initOpts.modulePath, "Go module path", "github.com/my-org/provider-my-database", "", true); err != nil {
			return err
		}
	}

	if initOpts.outputDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
		initOpts.outputDir = filepath.Join(cwd, initOpts.name)
	}

	cfg := &scaffold.Config{
		ProviderName:     initOpts.name,
		ModulePath:       initOpts.modulePath,
		UpstreamAPIGroup: initOpts.apiGroup,
		UpstreamResource: initOpts.resource,
	}
	cfg.DeriveDefaults()

	fmt.Println()
	fmt.Println("=== Configuration Summary ===")
	fmt.Printf("  Provider name:        %s\n", cfg.ProviderName)
	fmt.Printf("  Go module path:       %s\n", cfg.ModulePath)
	fmt.Printf("  Go package name:      %s\n", cfg.GoPackage)
	fmt.Printf("  Output directory:     %s\n", initOpts.outputDir)
	fmt.Println()

	if !initOpts.nonInteractive {
		val, err := tui.RunPrompt("Proceed?", "Y/n", "y", false)
		if err != nil {
			return err
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(val)), "n") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	fmt.Println()
	fmt.Println("Scaffolding provider project...")

	if err := scaffold.Scaffold(cfg, initOpts.outputDir); err != nil {
		return fmt.Errorf("scaffolding failed: %w", err)
	}

	fileCount, err := scaffold.CountFiles(initOpts.outputDir)
	if err != nil {
		fileCount = -1 // non-fatal
	}

	fmt.Println()
	fmt.Println("=== Provider project created successfully! ===")
	if fileCount > 0 {
		fmt.Printf("  Created %d files in %s\n", fileCount, initOpts.outputDir)
	}
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. cd %s\n", initOpts.outputDir)
	fmt.Println("  2. Read definition/PROVIDER_DEVELOPMENT.md for a complete development guide")
	fmt.Println("  3. Add your upstream operator dependency: go get <operator-module>@latest")
	fmt.Println("  4. Run: go mod tidy")
	fmt.Println()
	fmt.Println("  Define your provider:")
	fmt.Println("  5. Add components:  provider-sdk add component")
	fmt.Println("  6. Add topologies:  provider-sdk add topology")
	fmt.Println("  7. Configure versions in definition/versions.yaml")
	fmt.Println("  8. Configure topology UI in definition/topologies/*/topology.yaml")
	fmt.Println("  9. Define custom types in definition/components/types.go")
	fmt.Println()
	fmt.Println("  Implement and test:")
	fmt.Println("  10. Implement provider logic in internal/provider/provider.go")
	fmt.Println("  11. Add RBAC markers in internal/provider/rbac.go")
	fmt.Println("  12. Run: make generate")
	fmt.Println("  13. Test: make run")
	fmt.Println()

	return nil
}
