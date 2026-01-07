package main

// PSMDB Provider
//
// This example shows how to implement a provider using the SDK interface.

import (
	"fmt"

	provider "github.com/openeverest/provider-sdk/examples/psmdb/internal"
	"github.com/openeverest/provider-sdk/pkg/reconciler"
)

func main() {
	provider := provider.NewPSMDBProviderInterface()

	r, err := reconciler.New(provider,
		// Enable HTTP server for schema and validation endpoints
		reconciler.WithServer(reconciler.ServerConfig{
			Port:           8082,
			SchemaPath:     "/schema",
			ValidationPath: "/validate",
		}),
	)
	if err != nil {
		panic(fmt.Errorf("failed to create reconciler: %w", err))
	}

	if err := r.StartWithSignalHandler(); err != nil {
		panic(err)
	}
}
