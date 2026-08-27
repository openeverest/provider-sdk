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

	"github.com/spf13/cobra"

	"github.com/openeverest/provider-sdk/internal/scaffold"
)

var addConfigMapOpts struct {
	name string
}

func init() {
	f := addConfigMapCmd.Flags()
	f.StringVar(&addConfigMapOpts.name, "name", "", "ConfigMap definition name (used for directory and schema key)")

	addCmd.AddCommand(addConfigMapCmd)
}

var addConfigMapCmd = &cobra.Command{
	Use:   "configmap",
	Short: "Add a new configmap type definition to the provider",
	Long: `Add a new configmap type definition to an existing provider project.

This command creates the following files:
  - definition/configmaps/<name>/definition.yaml  (ConfigMap schema configuration)
  - definition/configmaps/<name>/ui.yaml          (UI rendering hints)
  - definition/configmaps/<name>/types.go         (Go types for configmap data schema)

The Go types in types.go are referenced by the parametersSchema.openAPIV3Schema
field in definition.yaml and converted to OpenAPI schemas at generation time.

Run from the provider project root directory.

Examples:
  # Add a configuration configmap type
  provider-sdk add configmap --name app-config

  # Add a settings configmap type
  provider-sdk add configmap --name database-settings`,
	RunE: runAddConfigMap,
}

func runAddConfigMap(_ *cobra.Command, _ []string) error {
	fmt.Println()
	fmt.Println("=== Add ConfigMap Definition ===")
	fmt.Println()

	if err := promptTUI(&addConfigMapOpts.name,
		"ConfigMap name", "app-config", "", true); err != nil {
		return err
	}

	cfg := &scaffold.AddConfigMapConfig{
		Name: addConfigMapOpts.name,
	}
	if err := scaffold.AddConfigMap(cfg); err != nil {
		return fmt.Errorf("adding configmap: %w", err)
	}

	fmt.Println()
	fmt.Println("=== ConfigMap definition added successfully! ===")
	fmt.Printf("  Name: %s\n", cfg.Name)
	fmt.Println()
	fmt.Println("Created files:")
	fmt.Printf("  - definition/configmaps/%s/definition.yaml\n", cfg.Name)
	fmt.Printf("  - definition/configmaps/%s/ui.yaml\n", cfg.Name)
	fmt.Printf("  - definition/configmaps/%s/types.go\n", cfg.Name)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit types.go to define the expected configmap data keys")
	fmt.Println("  2. Edit ui.yaml to add form fields for configmap creation")
	fmt.Println("  3. Run `make generate` to update the provider spec")
	fmt.Println()

	return nil
}
