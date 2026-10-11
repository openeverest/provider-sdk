package provider

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openeverest/openeverest/v2/provider-runtime/controller"
)

// Validate checks the rules about the Instance alone and returns every error
// with its field path. It gets no client, so it cannot read the cluster.
//
// The runtime calls it before every Sync, once the Instance matches the
// topologies, components, versions, parameter schemas and supportedFields the
// Provider declares, so it can rely on those. A rule on a single field is
// better declared: a +kubebuilder:validation marker on a parameters type, or
// a bound in supportedFields. Write the rules across fields here.
//
// A rule about another object's contents, such as the keys of a Secret, goes
// in an optional ValidateReferences(c *controller.Context) error method.
func (p *Provider) Validate(in controller.ValidationInput) controller.ValidationResult {
	var errs field.ErrorList

	// TODO: Add the rules the Provider CR cannot declare, e.g.:
	//
	//   engine := in.Instance.Spec.Components[common.ComponentEngine]
	//   if engine.Replicas != nil && *engine.Replicas%2 == 0 {
	//       errs = append(errs, field.Invalid(
	//           field.NewPath("spec", "components").Key(common.ComponentEngine).Child("replicas"),
	//           *engine.Replicas, "must be an odd number",
	//       ))
	//   }

	return controller.ValidationResult{Errors: errs}
}
