package main

// PSMDB Provider Manifest Generator
//
// This tool generates the Provider CR YAML manifest from the Go-defined metadata.
// Run this as part of your build process to keep the manifest in sync.
//
// Usage:
//   go run ./examples/cmd/generate-manifest
//
// Or add to Makefile:
//   generate-manifest:
//       go run ./examples/cmd/generate-manifest
//
// See docs/PROVIDER_CR_GENERATION.md for complete workflow documentation.

import (
	"flag"
	"fmt"
	"os"

	sdk "github.com/openeverest/provider-sdk/pkg/controller"
)

func main() {
	output := flag.String("output", "", "Output file path (default: stdout)")
	name := flag.String("name", "percona-server-mongodb-operator", "Provider name")
	namespace := flag.String("namespace", "", "Namespace (empty for cluster-scoped)")
	flag.Parse()

	// Define the metadata (same as in psmdb_interface.go)
	metadata := psmdbMetadata()

	// Validate the metadata
	if err := metadata.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid metadata: %v\n", err)
		os.Exit(1)
	}

	if *output == "" {
		// Write to stdout
		if err := sdk.GenerateManifestToStdout(metadata, *name, *namespace); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Write to file
		if err := sdk.GenerateManifest(metadata, *name, *namespace, *output); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Generated: %s\n", *output)
	}
}

// psmdbMetadata returns the PSMDB provider metadata.
// This is the same metadata defined in psmdb_interface.go.
// In a real project, you might share this via a common package.
func psmdbMetadata() *sdk.ProviderMetadata {
	return &sdk.ProviderMetadata{
		ComponentTypes: map[string]sdk.ComponentTypeMeta{
			"mongod": {
				Versions: []sdk.ComponentVersionMeta{
					{Version: "6.0.19-16", Image: "percona/percona-server-mongodb:6.0.19-16-multi"},
					{Version: "6.0.21-18", Image: "percona/percona-server-mongodb:6.0.21-18"},
					{Version: "7.0.18-11", Image: "percona/percona-server-mongodb:7.0.18-11"},
					{Version: "8.0.4-1", Image: "percona/percona-server-mongodb:8.0.4-1-multi"},
					{Version: "8.0.8-3", Image: "percona/percona-server-mongodb:8.0.8-3", Default: true},
				},
			},
			"backup": {
				Versions: []sdk.ComponentVersionMeta{
					{Version: "2.9.1", Image: "percona/percona-backup-mongodb:2.9.1", Default: true},
				},
			},
			"pmm": {
				Versions: []sdk.ComponentVersionMeta{
					{Version: "2.44.1", Image: "percona/pmm-client:2.44.1", Default: true},
				},
			},
			"exporter": {
				Versions: []sdk.ComponentVersionMeta{
					{Version: "0.47.2", Image: "percona/mongodb-exporter:0.47.2", Default: true},
				},
			},
		},
		Components: map[string]sdk.ComponentMeta{
			"engine":       {Type: "mongod"},
			"configServer": {Type: "mongod"},
			"proxy":        {Type: "mongod"},
			"backupAgent":  {Type: "backup"},
			"monitoring":   {Type: "pmm"},
			"metrics":      {Type: "exporter"},
		},
		Topologies: map[string]sdk.TopologyMeta{
			"standard": {
				Components: map[string]sdk.TopologyComponentMeta{
					"engine": {
						Optional: false,
						Defaults: map[string]interface{}{"replicas": 3},
					},
					"backupAgent": {Optional: true},
					"monitoring":  {Optional: true},
					"metrics":     {Optional: true},
				},
			},
			"sharded": {
				Components: map[string]sdk.TopologyComponentMeta{
					"engine": {
						Optional: false,
						Defaults: map[string]interface{}{"replicas": 3},
					},
					"proxy":        {Optional: false},
					"configServer": {Optional: false},
					"backupAgent":  {Optional: true},
					"monitoring":   {Optional: true},
					"metrics":      {Optional: true},
				},
			},
		},
	}
}
