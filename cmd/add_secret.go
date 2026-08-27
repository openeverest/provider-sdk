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

var addSecretOpts struct {
	name string
}

func init() {
	f := addSecretCmd.Flags()
	f.StringVar(&addSecretOpts.name, "name", "", "Secret definition name (used for directory and schema key)")

	addCmd.AddCommand(addSecretCmd)
}

var addSecretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Add a new secret type definition to the provider",
	Long: `Add a new secret type definition to an existing provider project.

This command creates the following files:
  - definition/secrets/<name>/definition.yaml  (Secret schema configuration)
  - definition/secrets/<name>/ui.yaml          (UI rendering hints)
  - definition/secrets/<name>/types.go         (Go types for secret data schema)

The Go types in types.go are referenced by the parametersSchema.openAPIV3Schema
field in definition.yaml and converted to OpenAPI schemas at generation time.

Run from the provider project root directory.

Examples:
  # Add a credentials secret type
  provider-sdk add secret --name credentials

  # Add a TLS certificates secret type
  provider-sdk add secret --name tls-certificates`,
	RunE: runAddSecret,
}

func runAddSecret(_ *cobra.Command, _ []string) error {
	fmt.Println()
	fmt.Println("=== Add Secret Definition ===")
	fmt.Println()

	if err := promptTUI(&addSecretOpts.name,
		"Secret name", "credentials", "", true); err != nil {
		return err
	}

	cfg := &scaffold.AddSecretConfig{
		Name: addSecretOpts.name,
	}
	if err := scaffold.AddSecret(cfg); err != nil {
		return fmt.Errorf("adding secret: %w", err)
	}

	fmt.Println()
	fmt.Println("=== Secret definition added successfully! ===")
	fmt.Printf("  Name: %s\n", cfg.Name)
	fmt.Println()
	fmt.Println("Created files:")
	fmt.Printf("  - definition/secrets/%s/definition.yaml\n", cfg.Name)
	fmt.Printf("  - definition/secrets/%s/ui.yaml\n", cfg.Name)
	fmt.Printf("  - definition/secrets/%s/types.go\n", cfg.Name)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit types.go to define the expected secret data keys")
	fmt.Println("  2. Edit ui.yaml to add form fields for secret creation")
	fmt.Println("  3. Run `make generate` to update the provider spec")
	fmt.Println()

	return nil
}
