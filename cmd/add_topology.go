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
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openeverest/provider-sdk/internal/scaffold"
	"github.com/openeverest/provider-sdk/internal/tui"
)

var addTopologyOpts struct {
	name string
}

func init() {
	f := addTopologyCmd.Flags()
	f.StringVar(&addTopologyOpts.name, "name", "", "Topology name (e.g., replicaSet, sharded)")

	addCmd.AddCommand(addTopologyCmd)
}

var addTopologyCmd = &cobra.Command{
	Use:   "topology",
	Short: "Add a new topology to the provider",
	Long: `Add a new topology to an existing provider project.

This command creates the following files:
  - definition/topologies/<name>/topology.yaml  (topology config + UI schema)
  - definition/topologies/<name>/types.go        (config type struct)

The topology.yaml is pre-populated with all components from definition/provider.yaml.

Run from the provider project root directory.

Examples:
  # Add a replica set topology
  provider-sdk add topology --name replicaSet

  # Add a sharded topology
  provider-sdk add topology --name sharded`,
	RunE: runAddTopology,
}

func runAddTopology(_ *cobra.Command, _ []string) error {
	fmt.Println()
	fmt.Println("=== Add Topology ===")
	fmt.Println()

	if err := promptTUI(&addTopologyOpts.name,
		"Topology name", "replicaSet, sharded…", "", true); err != nil {
		return err
	}

	// Read and present available components for multi-select.
	allComponents, err := scaffold.ReadProviderComponents()
	if err != nil {
		return fmt.Errorf("reading provider components: %w", err)
	}

	var selectedComponents []string
	if len(allComponents) == 0 {
		fmt.Println()
		fmt.Println("  No components found in definition/provider.yaml.")
		fmt.Println("  Run 'provider-sdk add component' first, then re-run this command.")
		fmt.Println("  Continuing — edit topology.yaml manually to add components.")
	} else {
		sort.Strings(allComponents)
		fmt.Println()
		selectedComponents, err = tui.RunMultiSelect(
			"Select components to include in this topology:",
			allComponents,
			true, // start with all checked
		)
		if err != nil {
			return err
		}
	}

	cfg := &scaffold.AddTopologyConfig{
		TopologyName:       addTopologyOpts.name,
		SelectedComponents: selectedComponents,
	}

	if err := scaffold.AddTopology(cfg); err != nil {
		return fmt.Errorf("adding topology: %w", err)
	}

	fmt.Println()
	fmt.Println("=== Topology added successfully! ===")
	fmt.Printf("  Topology:   %s\n", cfg.TopologyName)
	if len(selectedComponents) > 0 {
		fmt.Printf("  Components: %s\n", strings.Join(selectedComponents, ", "))
	}
	fmt.Println()
	fmt.Println("Created files:")
	fmt.Printf("  - definition/topologies/%s/topology.yaml\n", cfg.TopologyName)
	fmt.Printf("  - definition/topologies/%s/types.go\n", cfg.TopologyName)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit topology.yaml to configure which components this topology uses")
	fmt.Println("  2. Mark optional components with 'optional: true'")
	fmt.Println("  3. Set default values under 'defaults:' for each component")
	fmt.Println("  4. Configure the UI schema sections for the frontend form")
	fmt.Println("  5. Add fields to the TopologyConfig struct in types.go if needed")
	fmt.Println("  6. Reference the config type via 'configSchema:' in topology.yaml")
	fmt.Println("  7. Run: make generate")
	fmt.Println()

	return nil
}
