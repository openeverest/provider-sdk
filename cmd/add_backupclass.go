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

var addBackupClassOpts struct {
	name          string
	executionMode string
}

func init() {
	f := addBackupClassCmd.Flags()
	f.StringVar(&addBackupClassOpts.name, "name", "", "BackupClass name (used for both metadata.name and directory)")
	f.StringVar(&addBackupClassOpts.executionMode, "execution-mode", "ProviderManaged", "BackupClass execution mode: ProviderManaged or Job")

	addCmd.AddCommand(addBackupClassCmd)
}

var addBackupClassCmd = &cobra.Command{
	Use:   "backupclass",
	Short: "Add a new BackupClass to the provider",
	Long: `Add a new BackupClass to an existing provider project.

This command creates the following files:
  - definition/backupclasses/<name>/class.yaml  (BackupClass metadata, limits, schema refs)
  - definition/backupclasses/<name>/ui.yaml     (UI rendering hints, grouped by modal)
  - definition/backupclasses/<name>/types.go    (Go types for backup/restore/PITR config)

The class.yaml is pre-populated with the calling provider as the only entry in
supportedProviders. The Go types in types.go are referenced by openAPIV3Schema
fields in class.yaml and converted to OpenAPI schemas at generation time.

Run from the provider project root directory.

Examples:
  # Add a ProviderManaged BackupClass
  provider-sdk add backupclass --name everest-percona-psmdb-operator

  # Add a Job-based BackupClass
  provider-sdk add backupclass --name pg-dump --execution-mode Job`,
	RunE: runAddBackupClass,
}

func runAddBackupClass(_ *cobra.Command, _ []string) error {
	fmt.Println()
	fmt.Println("=== Add BackupClass ===")
	fmt.Println()

	if err := promptTUI(&addBackupClassOpts.name,
		"BackupClass name", "everest-percona-psmdb-operator", "", true); err != nil {
		return err
	}

	cfg := &scaffold.AddBackupClassConfig{
		Name:          addBackupClassOpts.name,
		ExecutionMode: addBackupClassOpts.executionMode,
	}
	if err := scaffold.AddBackupClass(cfg); err != nil {
		return fmt.Errorf("adding backup class: %w", err)
	}

	fmt.Println()
	fmt.Println("=== BackupClass added successfully! ===")
	fmt.Printf("  Name:           %s\n", cfg.Name)
	fmt.Printf("  Execution mode: %s\n", cfg.ExecutionMode)
	fmt.Println()
	fmt.Println("Created files:")
	fmt.Printf("  - definition/backupclasses/%s/class.yaml\n", cfg.Name)
	fmt.Printf("  - definition/backupclasses/%s/ui.yaml\n", cfg.Name)
	fmt.Printf("  - definition/backupclasses/%s/types.go\n", cfg.Name)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Fill in the Go types in types.go to describe backup/restore/PITR config")
	fmt.Println("  2. Edit class.yaml to set displayName, description, supportsPITR, and limits")
	fmt.Println("  3. Edit ui.yaml to add form fields under backup / pitr / restore")
	fmt.Println("  4. Run `make generate` to render charts/.../generated/backupclasses/<name>.yaml")
	fmt.Println()

	return nil
}
