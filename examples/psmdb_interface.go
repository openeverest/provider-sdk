package main

// PSMDB Provider - Interface-Based Approach
//
// This example shows how to implement a provider using the interface pattern.
// Compare with psmdb_builder.go for the builder-based approach.

import (
	"fmt"

	sdk "github.com/openeverest/provider-sdk/pkg/controller"
	"github.com/openeverest/provider-sdk/pkg/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	psmdbspec "github.com/openeverest/provider-sdk/examples/psmdbspec"
	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
)

// PSMDBProvider implements the sdk.ProviderIface interface.
type PSMDBProvider struct {
	sdk.BaseProvider
}

// NewPSMDBProviderInterface creates a new PSMDB provider.
func NewPSMDBProviderInterface() *PSMDBProvider {
	return &PSMDBProvider{
		BaseProvider: sdk.BaseProvider{
			ProviderName: "psmdb",
			SchemeFuncs: []func(*runtime.Scheme) error{
				psmdbv1.SchemeBuilder.AddToScheme,
			},
			Owned: []client.Object{
				&psmdbv1.PerconaServerMongoDB{},
			},
			Metadata: PSMDBMetadata(),
		},
	}
}

// Interface implementation - delegates to shared functions in psmdb_impl.go

func (p *PSMDBProvider) Validate(c *sdk.Cluster) error {
	return ValidatePSMDB(c)
}

func (p *PSMDBProvider) Sync(c *sdk.Cluster) error {
	return SyncPSMDB(c)
}

func (p *PSMDBProvider) Status(c *sdk.Cluster) (sdk.Status, error) {
	return StatusPSMDB(c)
}

func (p *PSMDBProvider) Cleanup(c *sdk.Cluster) error {
	return CleanupPSMDB(c)
}

func main() {
	provider := NewPSMDBProviderInterface()

	r, err := reconciler.NewFromInterface(provider,
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

// Compile-time interface checks
var _ sdk.ProviderIface = (*PSMDBProvider)(nil)
var _ sdk.MetadataProvider = (*PSMDBProvider)(nil)
var _ sdk.SchemaProvider = (*PSMDBProvider)(nil)

// SchemaProvider implementation for OpenAPI schema generation

func (p *PSMDBProvider) ComponentSchemas() map[string]interface{} {
	return map[string]interface{}{
		ComponentEngine:       &psmdbspec.MongodCustomSpec{},
		ComponentConfigServer: &psmdbspec.MongodCustomSpec{},
		ComponentProxy:        &psmdbspec.MongosCustomSpec{},
		ComponentBackupAgent:  &psmdbspec.BackupCustomSpec{},
		ComponentMonitoring:   &psmdbspec.PMMCustomSpec{},
	}
}

func (p *PSMDBProvider) Topologies() map[string]sdk.TopologyDefinition {
	return PSMDBTopologyDefinitions()
}

func (p *PSMDBProvider) GlobalSchema() interface{} {
	return &psmdbspec.GlobalConfig{}
}
