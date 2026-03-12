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
	"github.com/openeverest/provider-sdk/internal/tui"
)

var addComponentOpts struct {
	name          string
	componentType string
}

func init() {
	f := addComponentCmd.Flags()
	f.StringVar(&addComponentOpts.name, "name", "", "Component name (e.g., backupAgent, proxy, monitoring)")
	f.StringVar(&addComponentOpts.componentType, "type", "", "Component type (e.g., backup, mongos, pmm)")

	addCmd.AddCommand(addComponentCmd)
}

var addComponentCmd = &cobra.Command{
	Use:   "component",
	Short: "Add a new component to the provider",
	Long: `Add a new component to an existing provider project.

This command updates the following files:
  - definition/provider.yaml       (adds the component entry)
  - definition/versions.yaml       (adds the component type if new)
  - definition/components/types.go (adds a CustomSpec struct)
  - internal/common/spec.go        (adds name/type constants)

Run from the provider project root directory.

Examples:
  # Add a backup agent component
  provider-sdk add component --name backupAgent --type backup

  # Add a monitoring component
  provider-sdk add component --name monitoring --type pmm

  # Add a proxy component (reusing an existing type)
  provider-sdk add component --name proxy --type mongod`,
	RunE: runAddComponent,
}

func runAddComponent(_ *cobra.Command, _ []string) error {
	fmt.Println()
	fmt.Println("=== Add Component ===")
	fmt.Println()

	if err := promptTUI(&addComponentOpts.name,
		"Component name", "backupAgent, proxy, monitoring…", "", true); err != nil {
		return err
	}
	if err := promptTUI(&addComponentOpts.componentType,
		"Component type", "backup, mongos, pmm…", "", true); err != nil {
		return err
	}

	cfg := &scaffold.AddComponentConfig{
		ComponentName: addComponentOpts.name,
		ComponentType: addComponentOpts.componentType,
	}

	if err := scaffold.AddComponent(cfg); err != nil {
		return fmt.Errorf("adding component: %w", err)
	}

	fmt.Println()
	fmt.Println("=== Component added successfully! ===")
	fmt.Printf("  Component: %s (type: %s)\n", cfg.ComponentName, cfg.ComponentType)
	fmt.Println()
	fmt.Println("Updated files:")
	fmt.Println("  - definition/provider.yaml")
	fmt.Println("  - definition/versions.yaml")
	fmt.Println("  - definition/components/types.go")
	fmt.Println("  - internal/common/spec.go")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Update definition/versions.yaml with real version/image entries")
	fmt.Println("  2. Add fields to the CustomSpec struct in definition/components/types.go if needed")
	fmt.Println("  3. Reference the component in your topology files (definition/topologies/*/topology.yaml)")
	fmt.Println("  4. Add RBAC hints in internal/provider/rbac.go if the component needs new permissions")
	fmt.Println("  5. Run: make generate")
	fmt.Println()

	return nil
}

// promptTUI runs a bubbletea text-input prompt when value is not already set via flag.
// It prints the pre-set flag value instead of prompting when the flag is already set.
func promptTUI(value *string, label, placeholder, defaultValue string, required bool) error {
	if *value != "" {
		fmt.Printf("  %s: %s (from flag)\n", label, *value)
		return nil
	}
	val, err := tui.RunPrompt(label, placeholder, defaultValue, required)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	*value = val
	return nil
}
