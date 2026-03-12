package provider

// Run `make manifests` to regenerate config/rbac/role.yaml from these markers.
// This file contains kubebuilder RBAC markers for controller-gen.
// See: https://book.kubebuilder.io/reference/markers/rbac

// Base RBAC (required by all providers):
// +kubebuilder:rbac:groups=core.openeverest.io,resources=instances,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core.openeverest.io,resources=instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.openeverest.io,resources=instances/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.openeverest.io,resources=providers,verbs=get;list;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// =============================================================================
// PROVIDER-SPECIFIC RBAC — Add markers for your upstream operator's resources.
// =============================================================================
// Examples:
//
//   - Watch/manage upstream operator CRs:
//   // +kubebuilder:rbac:groups=[[ .UpstreamAPIGroup ]],resources=[[ .UpstreamResource ]],verbs=get;list;watch;create;update;patch;delete
//   // +kubebuilder:rbac:groups=[[ .UpstreamAPIGroup ]],resources=[[ .UpstreamResource ]]/status,verbs=get
//   // +kubebuilder:rbac:groups=[[ .UpstreamAPIGroup ]],resources=[[ .UpstreamResource ]]/finalizers,verbs=update
//
//   - Access Kubernetes core resources:
//   // +kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get;list;watch
//   // +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//
//   - Access PVCs (if managing storage):
//   // +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
