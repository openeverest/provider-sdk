package main

// PSMDB Provider - Builder-Based Approach
//
// This example shows how to implement a provider using the builder pattern.
// Compare with psmdb_interface.go for the interface-based approach.

import (
	"fmt"

	psmdbspec "github.com/openeverest/provider-sdk/examples/psmdbspec"
	sdk "github.com/openeverest/provider-sdk/pkg/controller"
	"github.com/openeverest/provider-sdk/pkg/reconciler"

	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
)

// NewPSMDBProviderBuilder creates a PSMDB provider using the builder API.
func NewPSMDBProviderBuilder() *sdk.Provider {
	builder := sdk.Build("psmdb").
		WithTypes(psmdbv1.SchemeBuilder.AddToScheme).
		Owns(&psmdbv1.PerconaServerMongoDB{}).
		WithMetadata(PSMDBMetadata()).
		WithComponentSchema(ComponentEngine, &psmdbspec.MongodCustomSpec{}).
		WithComponentSchema(ComponentConfigServer, &psmdbspec.MongodCustomSpec{}).
		WithComponentSchema(ComponentProxy, &psmdbspec.MongosCustomSpec{}).
		WithComponentSchema(ComponentBackupAgent, &psmdbspec.BackupCustomSpec{}).
		WithComponentSchema(ComponentMonitoring, &psmdbspec.PMMCustomSpec{})

	// Register topologies
	for name, def := range PSMDBTopologyDefinitions() {
		builder = builder.WithTopology(name, def)
	}

	return builder.
		WithGlobalSchema(&psmdbspec.GlobalConfig{}).
		Validate(ValidatePSMDB).
		Sync("Sync PSMDB", SyncPSMDB).
		Status(StatusPSMDB).
		Cleanup("Cleanup PSMDB", CleanupPSMDB).
		Done()
}

func main() {
	provider := NewPSMDBProviderBuilder()

	r, err := reconciler.NewFromBuilder(provider,
		reconciler.WithServer(reconciler.ServerConfig{
			Port:           8080,
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

// Compile-time interface checks
var _ sdk.MetadataProvider = (*sdk.Provider)(nil)
var _ sdk.SchemaProvider = (*sdk.Provider)(nil)
