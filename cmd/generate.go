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

	"github.com/openeverest/provider-sdk/internal/generate"
)

var generateOpts struct {
	definitionDir string
	chartDir      string
	typesPackages []string
}

func init() {
	f := generateCmd.Flags()
	f.StringVar(&generateOpts.definitionDir, "definition-dir", "definition", "Path to the definition directory")
	f.StringVar(&generateOpts.chartDir, "chart-dir", "", "Path to the Helm chart directory (default: auto-detect from provider name)")
	f.StringSliceVar(&generateOpts.typesPackages, "types-package", nil, "Go package patterns containing type definitions (default: ./definition/...)")

	rootCmd.AddCommand(generateCmd)
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Provider CR spec for Helm chart from definition/ files",
	Long: `Read the definition/ directory, resolve Go type schemas, and generate
the Provider CR spec YAML for the Helm chart.

The generated file is written to charts/<name>/generated/provider-spec.yaml,
following the same intermediate-file pattern used for RBAC rules
(generated via controller-gen + yq → charts/.../generated/rbac-rules.yaml).

The Helm chart template then includes the generated spec using:
  {{ .Files.Get "generated/provider-spec.yaml" | nindent 0 }}

Usage via go:generate (in gen.go):

  //go:generate go tool provider-sdk generate

The command reads definition/provider.yaml, definition/versions.yaml,
and definition/topologies/*/topology.yaml to build the Provider CR spec.

If any topology or component references a Go type via configSchema or
customSpecSchema, the command loads the specified Go packages and performs
static type analysis to generate OpenAPI schemas.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return generate.Run(generate.Options{
			DefinitionDir: generateOpts.definitionDir,
			ChartDir:      generateOpts.chartDir,
			TypesPackages: generateOpts.typesPackages,
		})
	},
}
