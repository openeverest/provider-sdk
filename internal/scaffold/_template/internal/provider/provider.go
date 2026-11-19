package provider

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/openeverest/openeverest/v2/provider-runtime/controller"

	// TODO: Import your operator's API types package, e.g.:
	// operatorv1 "github.com/example/my-operator/api/v1"

	"[[ .ModulePath ]]/internal/common"
)

// Compile-time check that Provider implements the required interface.
var _ controller.ProviderInterface = (*Provider)(nil)

// Provider implements controller.ProviderInterface for the [[ .ProjectName ]] provider.
type Provider struct {
	controller.BaseProvider
}

// New creates a new Provider instance.
func New() *Provider {
	return &Provider{
		BaseProvider: controller.BaseProvider{
			ProviderName: common.ProviderName,
			SchemeFuncs:  []func(*runtime.Scheme) error{
				// TODO: Register your operator's scheme, e.g.:
				// operatorv1.SchemeBuilder.AddToScheme,
			},
			WatchConfigs: []controller.WatchConfig{
				// TODO: Watch your operator's primary resource, e.g.:
				// controller.WatchOwned(&operatorv1.MyDatabase{}),
			},
		},
	}
}

// Sync ensures all required resources exist and are configured correctly.
//
// This is the main reconciliation logic. Create or update your
// operator's custom resource(s) based on the Instance spec.
//
// +kubebuilder:rbac:groups=[[ .OperatorAPIGroup ]],resources=[[ .OperatorResource ]],verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=[[ .OperatorAPIGroup ]],resources=[[ .OperatorResource ]]/status,verbs=get
func (p *Provider) Sync(c *controller.Context) error {
	l := log.FromContext(c.Context())
	l.Info("Syncing instance", "name", c.Name())

	// TODO: Implement sync logic.
	// Typical pattern:
	//   1. Build the operator CR spec from the Instance spec
	//   2. Label each component's pods with c.PodLabels(<component name>),
	//      so the runtime can count them and report crash loops
	//   3. Use c.Apply() to create/update the operator resource
	//
	// Example:
	//   cr := &operatorv1.MyDatabase{
	//       ObjectMeta: metav1.ObjectMeta{
	//           Name:      c.Name(),
	//           Namespace: c.Namespace(),
	//       },
	//       Spec: buildSpec(c),
	//   }
	//   cr.Spec.PodLabels = c.PodLabels("engine")
	//   return c.Apply(cr)
	return nil
}

// Status reports what the provider observes about the engine.
//
// It reports facts, not a phase: whether the engine runs what the Instance
// asks for, whether clients can connect, what is wrong and which long
// operation is running. The runtime derives the Instance's phase and
// conditions from them, the same way for every provider.
func (p *Provider) Status(c *controller.Context) (controller.Status, error) {
	l := log.FromContext(c.Context())
	l.Info("Computing status", "name", c.Name())

	// TODO: Implement status logic. Report only what the operator tells you.
	//
	// Example:
	//   cr := &operatorv1.MyDatabase{}
	//   if err := c.Get(cr, c.Name()); err != nil {
	//       if apierrors.IsNotFound(err) {
	//           return controller.Progressing("Waiting for the MyDatabase to be created"), nil
	//       }
	//       return controller.Status{}, err // retried, and reported if it persists
	//   }
	//
	//   // Converged: the operator has acted on the latest spec and reports ready.
	//   status := controller.Progressing("Waiting for the cluster to be ready")
	//   if cr.Status.ObservedGeneration == cr.Generation && cr.Status.State == "ready" {
	//       status = controller.Ready()
	//   }
	//
	//   // Available: whether clients can connect now. Publish the connection
	//   // details as soon as they can, not only once converged.
	//   if cr.Status.ReadyReplicas > 0 {
	//       status = status.WithAvailable(controller.Available)
	//       status.ConnectionDetails = connectionDetails(cr)
	//   } else {
	//       status = status.WithUnavailable("NoReplicaReady", "No replica is ready")
	//   }
	//
	//   // Operations: name every long operation. A wait with no name is
	//   // reported as stalled after 30 minutes.
	//   if cr.Status.Version != cr.Spec.Version {
	//       status = status.WithOperation(v1alpha1.ConditionUpgrading, "RollingUpdate",
	//           "Upgrading to "+cr.Spec.Version)
	//   }
	//
	//   // Problems: WithProblem for what may clear by itself, WithTerminalProblem
	//   // only for what nothing but a person can fix.
	//   switch {
	//   case cr.Spec.Paused: // Sync never sets it
	//       status = status.WithTerminalProblem("PausedOutsideOpenEverest",
	//           "The cluster was paused outside OpenEverest")
	//   case cr.Status.Error != "":
	//       status = status.WithProblem("ReplicationBroken", cr.Status.Error)
	//   }
	//
	//   // Desired replicas: the pods each component has once the current spec
	//   // has converged, defaults included.
	//   return status.WithDesiredReplicas("engine", engineReplicas(c)), nil
	//
	// The runtime already reports crash loops, image pull back-offs and
	// unschedulable pods (of the pods labelled with c.PodLabels), the seeding
	// of a new Instance from spec.dataSource, and errors returned from Sync or
	// Status. Don't report them again.

	return controller.Progressing("Status is not implemented yet"), nil
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
