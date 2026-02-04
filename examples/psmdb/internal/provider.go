package provider

// PSMDB Provider Implementation
//
// This file contains the business logic for the PSMDB provider.
//
// Key functions:
//   - ValidatePSMDB: Validate DataStore spec
//   - SyncPSMDB: Create/update PSMDB resources
//   - StatusPSMDB: Compute cluster status
//   - CleanupPSMDB: Handle deletion cleanup

import (
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/openeverest/provider-sdk/pkg/apis/v2alpha1"
	sdk "github.com/openeverest/provider-sdk/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	types "github.com/openeverest/provider-sdk/examples/psmdb/types"
	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
)

// Component name and type constants for PSMDB
const (
	ComponentEngine       = "engine"
	ComponentConfigServer = "configServer"
	ComponentProxy        = "proxy"
	ComponentBackupAgent  = "backupAgent"
	ComponentMonitoring   = "monitoring"
	ComponentMetrics      = "metrics"

	ComponentTypeMongod   = "mongod"
	ComponentTypeBackup   = "backup"
	ComponentTypePMM      = "pmm"
	ComponentTypeExporter = "exporter"
)

const (
	psmdbDefaultConfigurationTemplate = `
      operationProfiling:
        mode: slowOp
`
	defaultBackupStartingTimeout = 120
)

var maxUnavailable = intstr.FromInt(1)

func defaultSpec() psmdbv1.PerconaServerMongoDBSpec {
	return psmdbv1.PerconaServerMongoDBSpec{
		UpdateStrategy: psmdbv1.SmartUpdateStatefulSetStrategyType,
		UpgradeOptions: psmdbv1.UpgradeOptions{
			Apply:    "disabled",
			Schedule: "0 4 * * *",
			SetFCV:   true,
		},
		PMM: psmdbv1.PMMSpec{},
		Replsets: []*psmdbv1.ReplsetSpec{
			{
				Name:          "rs0",
				Configuration: psmdbv1.MongoConfiguration(psmdbDefaultConfigurationTemplate),
				MultiAZ: psmdbv1.MultiAZ{
					PodDisruptionBudget: &psmdbv1.PodDisruptionBudgetSpec{
						MaxUnavailable: &maxUnavailable,
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{},
					},
				},
				Size: 3,
				VolumeSpec: &psmdbv1.VolumeSpec{
					PersistentVolumeClaim: psmdbv1.PVCSpec{
						PersistentVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("10Gi"),
								},
							},
						},
					},
				},
			},
		},
		Sharding: psmdbv1.Sharding{
			Enabled: false,
		},
		VolumeExpansionEnabled: true,
		// FIXME
		CRVersion: "1.21.1",
	}
}

// ValidatePSMDB validates the DataStore spec for PSMDB.
func ValidatePSMDB(c *sdk.Context) error {
	fmt.Println("Validating PSMDB cluster:", c.Name())
	// TODO: Add actual validation logic
	// Example: Check for required components, validate storage sizes, etc.
	return nil
}

func configureReplset(name string, replicas *int32, resources *v2alpha1.Resources, storageSize *v2alpha1.Storage, expose bool) *psmdbv1.ReplsetSpec {
	rsSpec := &psmdbv1.ReplsetSpec{
		Name:          name,
		Configuration: psmdbv1.MongoConfiguration(psmdbDefaultConfigurationTemplate),
		MultiAZ: psmdbv1.MultiAZ{
			PodDisruptionBudget: &psmdbv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{},
			},
		},
		Size: 3,
		VolumeSpec: &psmdbv1.VolumeSpec{
			PersistentVolumeClaim: psmdbv1.PVCSpec{
				PersistentVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("10Gi"),
						},
					},
				},
			},
		},
		Expose: psmdbv1.ExposeTogglable{
			Enabled: expose,
			// TODO: implement exposing replset
			Expose: psmdbv1.Expose{
				ExposeType:         corev1.ServiceTypeClusterIP,
				ServiceAnnotations: map[string]string{},
			},
		},
	}

	if replicas != nil {
		rsSpec.Size = *replicas
	}
	if resources != nil && !resources.CPU.IsZero() {
		rsSpec.MultiAZ.Resources.Limits[corev1.ResourceCPU] = resources.CPU
	}
	if resources != nil && !resources.Memory.IsZero() {
		rsSpec.MultiAZ.Resources.Limits[corev1.ResourceMemory] = resources.Memory
	}
	if storageSize != nil && !storageSize.Size.IsZero() {
		rsSpec.VolumeSpec.PersistentVolumeClaim.PersistentVolumeClaimSpec.Resources.Requests[corev1.ResourceStorage] = storageSize.Size
	}

	return rsSpec
}

func rsName(i int) string {
	return fmt.Sprintf("rs%v", i)
}

func configureReplsets(c *sdk.Context) []*psmdbv1.ReplsetSpec {
	var replsets []*psmdbv1.ReplsetSpec

	ds := c.DB()
	spec := ds.Spec
	engine := spec.Components[ComponentEngine]

	// TODO: implement disabling
	if spec.Topology == nil || spec.Topology.Type != "sharded" {
		return []*psmdbv1.ReplsetSpec{
			configureReplset(rsName(0), engine.Replicas, engine.Resources, engine.Storage, true),
		}
	}

	numShards := 2 // default
	var shardedConfig types.ShardedTopologyConfig
	if c.TryDecodeTopologyConfig(&shardedConfig) && shardedConfig.NumShards > 0 {
		numShards = int(shardedConfig.NumShards)
	}

	// Create replsets for each shard
	for i := 0; i < numShards; i++ {
		replsets = append(replsets, configureReplset(rsName(i), engine.Replicas, engine.Resources, engine.Storage, false))
	}

	return replsets
}

func configureConfigServerReplset(c *sdk.Context) *psmdbv1.ReplsetSpec {
	var replset *psmdbv1.ReplsetSpec

	ds := c.DB()
	spec := ds.Spec
	cfgSrv := spec.Components[ComponentConfigServer]

	// TODO: implement disabling
	if spec.Topology == nil || spec.Topology.Type != "sharded" {
		return replset
	}

	// TODO: check if this is okay. It adds the configuration, expose.type,
	// name, podDisruptionBudget that we didn't have in the everest operator
	return configureReplset("configsvr", cfgSrv.Replicas, cfgSrv.Resources, cfgSrv.Storage, false)
}

func configureMongos(c *sdk.Context) *psmdbv1.MongosSpec {
	ds := c.DB()
	spec := ds.Spec
	proxy := spec.Components[ComponentProxy]

	mongosSpec := &psmdbv1.MongosSpec{
		Size: 3,
		MultiAZ: psmdbv1.MultiAZ{
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{},
			},
		},
	}

	if proxy.Replicas != nil {
		mongosSpec.Size = *proxy.Replicas
	}
	if proxy.Resources != nil && !proxy.Resources.CPU.IsZero() {
		mongosSpec.MultiAZ.Resources.Limits[corev1.ResourceCPU] = proxy.Resources.CPU
	}
	if proxy.Resources != nil && !proxy.Resources.Memory.IsZero() {
		mongosSpec.MultiAZ.Resources.Limits[corev1.ResourceMemory] = proxy.Resources.Memory
	}

	// TODO: implement exposing mongos
	mongosSpec.Expose = psmdbv1.MongosExpose{
		Expose: psmdbv1.Expose{
			ExposeType:         corev1.ServiceTypeClusterIP,
			ServiceAnnotations: map[string]string{},
		},
	}

	return mongosSpec
}

func configureBackup(c *sdk.Context) psmdbv1.BackupSpec {
	// TODO: Implement proper backup configuration
	var backupImage string
	if metadata := c.Metadata(); metadata != nil {
		backupImage = metadata.GetDefaultImage("backup")
	} else {
		backupImage = PSMDBMetadata().GetDefaultImage("backup")
	}

	return psmdbv1.BackupSpec{
		Enabled: true,
		Image:   backupImage,
		PITR: psmdbv1.PITRSpec{
			Enabled: false,
		},
		Configuration: psmdbv1.BackupConfig{
			BackupOptions: &psmdbv1.BackupOptions{
				Timeouts: &psmdbv1.BackupTimeouts{Starting: pointer.ToUint32(defaultBackupStartingTimeout)},
			},
		},

		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("1G"),
				corev1.ResourceCPU:    resource.MustParse("300m"),
			},
		},
	}
}

// configureExporter creates a sidecar container configuration for MongoDB Exporter.
// It exposes metrics on localhost:9216/metrics.
func configureExporter(c *sdk.Context, secretName string) *corev1.Container {
	if _, ok := c.DB().Spec.Components[ComponentMetrics]; !ok {
		return nil
	}

	// Use the image from the component spec if provided, otherwise use default
	var exporterImage string
	if metadata := c.Metadata(); metadata != nil {
		exporterImage = metadata.GetDefaultImage(ComponentTypeExporter)
	} else {
		exporterImage = PSMDBMetadata().GetDefaultImage(ComponentTypeExporter)
	}

	return &corev1.Container{
		Name:  c.Name() + "-metrics-exporter",
		Image: exporterImage,
		Args:  []string{"--discovering-mode", "--compatible-mode", "--collect-all", "--mongodb.uri=$(MONGODB_URI)"},
		Env: []corev1.EnvVar{
			{
				Name: "MONGODB_USER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
						Key: "MONGODB_CLUSTER_MONITOR_USER",
					},
				},
			},
			{
				Name: "MONGODB_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
						Key: "MONGODB_CLUSTER_MONITOR_PASSWORD",
					},
				},
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.name",
					},
				},
			},
			{
				Name:  "MONGODB_URI",
				Value: "mongodb://$(MONGODB_USER):$(MONGODB_PASSWORD)@$(POD_NAME)",
			},
		},
	}
}

// SyncPSMDB ensures all PSMDB resources exist and are configured correctly.
func SyncPSMDB(c *sdk.Context) error {
	fmt.Println("Syncing PSMDB cluster:", c.Name())
	psmdb := &psmdbv1.PerconaServerMongoDB{
		ObjectMeta: c.ObjectMeta(c.Name()),
		Spec:       defaultSpec(),
	}

	// Get the engine component spec
	engine := c.DB().Spec.Components[ComponentEngine]
	// No need to check if engine is nil, it is guaranteed to be present by the validator

	// Set the image: use the user-specified image if provided, otherwise use the default from metadata
	if engine.Image != "" {
		// User explicitly specified an image
		psmdb.Spec.Image = engine.Image
	} else if metadata := c.Metadata(); metadata != nil {
		// Look up the default image for the component type from the registered metadata
		psmdb.Spec.Image = metadata.GetDefaultImage("mongod")
	} else {
		// Fallback: metadata not available, use PSMDBMetadata() directly
		// This can happen in tests or when using NewContext instead of NewContextWithMetadata
		psmdb.Spec.Image = PSMDBMetadata().GetDefaultImage(engine.Type)
	}
	psmdb.Spec.ImagePullPolicy = corev1.PullIfNotPresent

	psmdb.Spec.Replsets = configureReplsets(c)
	if c.DB().Spec.Topology != nil && c.DB().Spec.Topology.Type == "sharded" {
		psmdb.Spec.Sharding.Enabled = true
		psmdb.Spec.Sharding.ConfigsvrReplSet = configureConfigServerReplset(c)
		psmdb.Spec.Sharding.Mongos = configureMongos(c)
	}

	psmdb.Spec.Backup = configureBackup(c)

	psmdb.Spec.Secrets = &psmdbv1.SecretsSpec{
		Users:         "everest-secrets-" + c.Name(),
		EncryptionKey: c.Name() + "-mongodb-encryption-key",
		SSLInternal:   c.Name() + "-ssl-internal",
	}

	// attaches the MongoDB exporter sidecar to replsets, if enabled.
	if sidecar := configureExporter(c, psmdb.Spec.Secrets.Users); sidecar != nil {
		for _, replset := range psmdb.Spec.Replsets {
			if replset == nil {
				continue
			}

			replset.MultiAZ.Sidecars, _ = replset.MultiAZ.WithSidecars(*sidecar)
		}

		// Also add exporter to config server replset in sharded topology
		if psmdb.Spec.Sharding.Enabled && psmdb.Spec.Sharding.ConfigsvrReplSet != nil {
			psmdb.Spec.Sharding.ConfigsvrReplSet.MultiAZ.Sidecars, _ = psmdb.Spec.Sharding.ConfigsvrReplSet.MultiAZ.WithSidecars(*sidecar)
		}
	}

	if err := c.Apply(psmdb); err != nil {
		return err
	}
	fmt.Println("PSMDB cluster synced:", c.Name())
	return nil
}

// StatusPSMDB computes the current status of the PSMDB cluster.
func StatusPSMDB(c *sdk.Context) (sdk.Status, error) {
	// TODO: We probably shouldn't be querying the PSMDB object directly here;
	// It can lead to a race condition where we are setting the status based on
	// new data whereas the sync used older data.
	// Should the SDK be responsible for fetching and caching the PSMDB object
	// to ensure we only get it once during the reconcile?
	psmdb := &psmdbv1.PerconaServerMongoDB{}
	if err := c.Get(psmdb, c.Name()); err != nil {
		return sdk.Creating("Waiting for PerconaServerMongoDB"), nil
	}
	switch psmdb.Status.State {
	case psmdbv1.AppStateReady:
		return sdk.Running(), nil
	case psmdbv1.AppStateError:
		return sdk.Failed(psmdb.Status.Message), nil
	default:
		return sdk.Creating("Cluster is being created"), nil
	}
}

// CleanupPSMDB handles deletion of the PSMDB cluster.
func CleanupPSMDB(c *sdk.Context) error {
	fmt.Println("Cleaning up PSMDB cluster:", c.Name())
	// TODO: Implemenent handling of finalizers
	psmdb := &psmdbv1.PerconaServerMongoDB{
		ObjectMeta: c.ObjectMeta(c.Name()),
	}
	if err := c.Delete(psmdb); err != nil {
		return err
	}
	fmt.Println("PSMDB cluster cleaned up:", c.Name())
	return nil
}

// =============================================================================
// PROVIDER METADATA
// =============================================================================

// PSMDBTopologyDefinitions returns the topology definitions for PSMDB.
// This is shared by all provider implementations to maintain a single source of truth.
func PSMDBTopologyDefinitions() map[string]sdk.TopologyDefinition {
	return map[string]sdk.TopologyDefinition{
		string(types.TopologyTypeReplicaSet): {
			Schema: &types.ReplicaSetTopologyConfig{},
			Components: map[string]sdk.TopologyComponentDefinition{
				ComponentEngine:      {Optional: false, Defaults: map[string]interface{}{"replicas": 3}},
				ComponentBackupAgent: {Optional: true},
				ComponentMonitoring:  {Optional: true},
				ComponentMetrics:     {Optional: true},
			},
		},
		string(types.TopologyTypeSharded): {
			Schema: &types.ShardedTopologyConfig{},
			Components: map[string]sdk.TopologyComponentDefinition{
				ComponentEngine:       {Optional: false, Defaults: map[string]interface{}{"replicas": 3}},
				ComponentProxy:        {Optional: false},
				ComponentConfigServer: {Optional: false},
				ComponentBackupAgent:  {Optional: true},
				ComponentMonitoring:   {Optional: true},
				ComponentMetrics:      {Optional: true},
			},
		},
	}
}

// PSMDBMetadata returns the metadata for the PSMDB provider.
// This defines the component types, versions, components, and topologies
// that the provider supports.
//
// This metadata is shared by all PSMDB provider examples and is used for:
// - CLI generation: `go run ./cmd/generate-manifest` -> provider.yaml (for Helm)
// - Runtime metadata access via c.Metadata()
//
// Note: The topologies are derived from the shared PSMDBTopologyDefinitions()
// to maintain a single source of truth across all provider implementations.
func PSMDBMetadata() *sdk.ProviderMetadata {
	// Define component types and logical components
	metadata := &sdk.ProviderMetadata{
		// ComponentTypes defines the available component types with their versions.
		// Each component type represents a different image/binary that can be deployed.
		ComponentTypes: map[string]sdk.ComponentTypeMeta{
			// mongod is the main MongoDB server component
			ComponentTypeMongod: {
				Versions: []sdk.ComponentVersionMeta{
					{Version: "6.0.19-16", Image: "percona/percona-server-mongodb:6.0.19-16-multi"},
					{Version: "6.0.21-18", Image: "percona/percona-server-mongodb:6.0.21-18"},
					{Version: "7.0.18-11", Image: "percona/percona-server-mongodb:7.0.18-11"},
					{Version: "8.0.4-1", Image: "percona/percona-server-mongodb:8.0.4-1-multi"},
					{Version: "8.0.8-3", Image: "percona/percona-server-mongodb:8.0.8-3", Default: true},
				},
			},
			// backup is the backup agent component
			ComponentTypeBackup: {
				Versions: []sdk.ComponentVersionMeta{
					{Version: "2.9.1", Image: "percona/percona-backup-mongodb:2.9.1", Default: true},
				},
			},
			// pmm is the Percona Monitoring and Management component
			ComponentTypePMM: {
				Versions: []sdk.ComponentVersionMeta{
					{Version: "2.44.1", Image: "percona/pmm-server:2.44.1", Default: true},
				},
			},
			ComponentTypeExporter: {
				Versions: []sdk.ComponentVersionMeta{
					{Version: "0.47.2", Image: "percona/mongodb_exporter:0.47.2", Default: true},
				},
			},
		},

		// Components defines the logical components that use the component types.
		// Multiple components can reference the same component type (e.g., engine and configServer both use mongod).
		Components: map[string]sdk.ComponentMeta{
			ComponentEngine:       {Type: ComponentTypeMongod},   // Main database engine
			ComponentConfigServer: {Type: ComponentTypeMongod},   // Config server for sharded clusters
			ComponentProxy:        {Type: ComponentTypeMongod},   // Proxy/mongos for sharded clusters
			ComponentBackupAgent:  {Type: ComponentTypeBackup},   // Backup agent
			ComponentMonitoring:   {Type: ComponentTypePMM},      // Monitoring agent
			ComponentMetrics:      {Type: ComponentTypeExporter}, // Metrics exporter
		},
	}

	// Derive topologies from the shared topology definitions
	metadata.Topologies = sdk.TopologiesFromSchemaProvider(PSMDBTopologyDefinitions())

	return metadata
}

// PSMDBProvider implements the sdk.ProviderInterface interface.
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

func (p *PSMDBProvider) Validate(c *sdk.Context) error {
	return ValidatePSMDB(c)
}

func (p *PSMDBProvider) Sync(c *sdk.Context) error {
	return SyncPSMDB(c)
}

func (p *PSMDBProvider) Status(c *sdk.Context) (sdk.Status, error) {
	return StatusPSMDB(c)
}

func (p *PSMDBProvider) Cleanup(c *sdk.Context) error {
	return CleanupPSMDB(c)
}

// Compile-time interface checks
var _ sdk.ProviderInterface = (*PSMDBProvider)(nil)
var _ sdk.MetadataProvider = (*PSMDBProvider)(nil)
var _ sdk.SchemaProvider = (*PSMDBProvider)(nil)

// SchemaProvider implementation for OpenAPI schema generation

func (p *PSMDBProvider) ComponentSchemas() map[string]interface{} {
	return map[string]interface{}{
		ComponentEngine:       &types.MongodCustomSpec{},
		ComponentConfigServer: &types.MongodCustomSpec{},
		ComponentProxy:        &types.MongosCustomSpec{},
		ComponentBackupAgent:  &types.BackupCustomSpec{},
		ComponentMonitoring:   &types.PMMCustomSpec{},
		ComponentMetrics:      &types.ExporterSpec{},
	}
}

func (p *PSMDBProvider) Topologies() map[string]sdk.TopologyDefinition {
	return PSMDBTopologyDefinitions()
}

func (p *PSMDBProvider) GlobalSchema() interface{} {
	return &types.GlobalConfig{}
}
