package main

// Provider SDK CLI Tool
//
// This tool provides utilities for provider developers, including:
// - generate-manifest: Generate a Provider CR YAML from Go code
//
// Usage:
//   provider-sdk generate-manifest --name <provider-name> --namespace <namespace> --output <file>
//
// See docs/PROVIDER_CR_GENERATION.md for detailed documentation.

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate-manifest":
		generateManifestCmd(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Provider SDK CLI Tool

Usage:
  provider-sdk <command> [options]

Commands:
  generate-manifest    Generate a Provider CR YAML manifest from Go code
  help                 Show this help message

Use "provider-sdk <command> -h" for more information about a command.`)
}

func generateManifestCmd(args []string) {
	fs := flag.NewFlagSet("generate-manifest", flag.ExitOnError)
	name := fs.String("name", "", "Provider name (required)")
	namespace := fs.String("namespace", "", "Namespace for the Provider CR (optional, omit for cluster-scoped)")
	output := fs.String("output", "", "Output file path (default: stdout)")

	fs.Usage = func() {
		fmt.Println(`Generate a Provider CR YAML manifest from Go code.

This command is intended to be called from a Go generate directive in your
provider's main package. It reads the provider metadata from your Go code
and generates a YAML manifest that can be included in your Helm chart.

Usage:
  provider-sdk generate-manifest [options]

Options:`)
		fs.PrintDefaults()
		fmt.Println(`
Example usage in your provider code:

  //go:generate provider-sdk generate-manifest --name percona-server-mongodb-operator --output ../../charts/provider/templates/provider.yaml

The actual metadata is read from your provider implementation via a special
init mechanism. See the PSMDB example for details.

See docs/PROVIDER_CR_GENERATION.md for complete workflow documentation.`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *name == "" {
		fmt.Fprintln(os.Stderr, "Error: --name is required")
		fs.Usage()
		os.Exit(1)
	}

	// Note: In a real implementation, this would load the provider metadata
	// from a compiled Go binary or through a plugin mechanism.
	// For now, we provide a library function that providers call directly.
	fmt.Fprintf(os.Stderr, `Note: This CLI is a placeholder for the generate-manifest workflow.

In practice, provider developers should use the library function directly:

  // In your provider's gen.go file:
  package main

  import (
      "os"
      sdk "github.com/openeverest/provider-sdk/pkg/controller"
  )

  func main() {
      metadata := defineMetadata() // Your metadata definition
      yaml, err := metadata.ToYAML("%s", "%s")
      if err != nil {
          panic(err)
      }
      
      // Write to file or stdout
      if err := os.WriteFile("provider.yaml", []byte(yaml), 0644); err != nil {
          panic(err)
      }
  }

See examples/psmdb_interface.go for a complete example.
`, *name, *namespace)

	// For demonstration, we'll generate a template
	if *output != "" {
		fmt.Fprintf(os.Stderr, "Would write to: %s\n", *output)
	}
}

