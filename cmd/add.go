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
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add components or topologies to an existing provider project",
	Long: `Add new components or topologies to an existing provider project.

Run from within a scaffolded provider project directory (must contain
definition/provider.yaml).

Subcommands:
  component  Add a new component to the provider
  topology   Add a new topology to the provider`,
}

func init() {
	rootCmd.AddCommand(addCmd)
}
