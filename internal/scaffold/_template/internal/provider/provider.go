package provider

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/openeverest/openeverest/v2/provider-runtime/controller"

	// TODO: Import your upstream operator's API types package, e.g.:
	// operatorv1 "github.com/example/my-operator/api/v1"

	"[[ .ModulePath ]]/internal/common"
)

// Compile-time check that Provider implements the required interface.
var _ controller.ProviderInterface = (*Provider)(nil)

// Provider implements controller.ProviderInterface for the [[ .ProviderName ]] provider.
type Provider struct {
	controller.BaseProvider
}

// New creates a new Provider instance.
func New() *Provider {
	return &Provider{
		BaseProvider: controller.BaseProvider{
			ProviderName: common.ProviderName,
			SchemeFuncs:  []func(*runtime.Scheme) error{
				// TODO: Register your upstream operator's scheme, e.g.:
				// operatorv1.SchemeBuilder.AddToScheme,
			},
			WatchConfigs: []controller.WatchConfig{
				// TODO: Watch your upstream operator's primary resource, e.g.:
				// controller.WatchOwned(&operatorv1.MyDatabase{}),
			},
		},
	}
}

// Validate checks if the Instance spec is valid.
//
// Add your provider-specific validation logic here.
// Return an error if the spec is invalid.
//
// +kubebuilder:rbac:groups=[[ .UpstreamAPIGroup ]],resources=[[ .UpstreamResource ]],verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=[[ .UpstreamAPIGroup ]],resources=[[ .UpstreamResource ]]/status,verbs=get
func (p *Provider) Validate(c *controller.Context) error {
	l := log.FromContext(c.Context())
	l.Info("Validating instance", "name", c.Name())

	// TODO: Implement validation logic.
	// Examples:
	//   - Check that required components are present
	//   - Validate storage sizes, replica counts
	//   - Ensure version compatibility
	return nil
}

// Sync ensures all required resources exist and are configured correctly.
//
// This is the main reconciliation logic. Create or update your upstream
// operator's custom resource(s) based on the Instance spec.
func (p *Provider) Sync(c *controller.Context) error {
	l := log.FromContext(c.Context())
	l.Info("Syncing instance", "name", c.Name())

	// TODO: Implement sync logic.
	// Typical pattern:
	//   1. Build the upstream CR spec from the Instance spec
	//   2. Use c.Apply() to create/update the upstream resource
	//
	// Example:
	//   cr := &operatorv1.MyDatabase{
	//       ObjectMeta: metav1.ObjectMeta{
	//           Name:      c.Name(),
	//           Namespace: c.Namespace(),
	//       },
	//       Spec: buildSpec(c),
	//   }
	//   return c.Apply(cr)
	return nil
}

// Status computes the current status of the database instance.
//
// Query the upstream operator's resource(s) and translate their status
// into the provider-runtime's Status type.
func (p *Provider) Status(c *controller.Context) (controller.Status, error) {
	l := log.FromContext(c.Context())
	l.Info("Computing status", "name", c.Name())

	// TODO: Implement status logic.
	// Typical pattern:
	//   1. Get the upstream CR using c.Get()
	//   2. Translate its status to a controller.Status
	//
	// Example:
	//   cr := &operatorv1.MyDatabase{}
	//   if err := c.Get(c.NamespacedName(), cr); err != nil {
	//       return controller.Status{}, err
	//   }
	//   if cr.Status.Ready {
	//       return controller.Running(), nil
	//   }
	//   return controller.Creating("waiting for database to be ready"), nil

	return controller.Creating("initializing"), nil
}

// Cleanup handles deletion of provider-managed resources.
//
// Called when the Instance has a deletion timestamp set.
// Delete any resources that are not automatically cleaned up
// via owner references.
func (p *Provider) Cleanup(c *controller.Context) error {
	l := log.FromContext(c.Context())
	l.Info("Cleaning up instance", "name", c.Name())

	// TODO: Implement cleanup logic if needed.
	// Resources with owner references set via c.Apply() are automatically
	// garbage collected. Only implement this if you need custom cleanup.
	return nil
}
