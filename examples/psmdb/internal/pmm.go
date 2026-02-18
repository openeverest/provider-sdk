package provider

import (
	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/openeverest/provider-sdk/pkg/apis/v2alpha1"
	sdk "github.com/openeverest/provider-sdk/pkg/controller"
)

const (
	// Consts used for calculating resources.

	// Kibibyte represents 1 KiB.
	Kibibyte = 1024
	// Mebibyte represents 1 MiB.
	Mebibyte = 1024 * Kibibyte
	// pmmClientRequestCPUSmall are the default CPU requests for PMM client in small clusters.
	pmmClientRequestCPUSmall = 95
	// pmmClientRequestCPUMedium are the default CPU requests for PMM client in medium clusters.
	pmmClientRequestCPUMedium = 228
	// pmmClientRequestCPULarge are the default CPU requests for PMM client in large clusters.
	pmmClientRequestCPULarge = 228
)

// Prefefined database engine sizes based on memory.
var (
	MemorySmallSize  = resource.MustParse("2G")
	MemoryMediumSize = resource.MustParse("8G")
	MemoryLargeSize  = resource.MustParse("32G")
)

var (
	// NOTE: provided below values were taken from the tool https://github.com/Tusamarco/mysqloperatorcalculator

	// PmmResourceRequirementsSmall is the resource requirements for PMM for small clusters.
	PmmResourceRequirementsSmall = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			// 97.27Mi = 97 MiB + 276 KiB = 99604 KiB
			corev1.ResourceMemory: *resource.NewQuantity(97*Mebibyte+276*Kibibyte, resource.BinarySI),
			corev1.ResourceCPU:    *resource.NewScaledQuantity(pmmClientRequestCPUSmall, resource.Milli),
		},
	}

	// PmmResourceRequirementsMedium is the resource requirements for PMM for medium clusters.
	PmmResourceRequirementsMedium = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			// 194.5Mi = 194 MiB + 512 KiB = 199168 KiB
			corev1.ResourceMemory: *resource.NewQuantity(194*Mebibyte+512*Kibibyte, resource.BinarySI),
			corev1.ResourceCPU:    *resource.NewScaledQuantity(pmmClientRequestCPUMedium, resource.Milli),
		},
	}

	// PmmResourceRequirementsLarge is the resource requirements for PMM for large clusters.
	PmmResourceRequirementsLarge = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			// 778.23Mi = 778 MiB + 235 KiB = 796907 KiB
			corev1.ResourceMemory: *resource.NewQuantity(778*Mebibyte+235*Kibibyte, resource.BinarySI),
			corev1.ResourceCPU:    *resource.NewScaledQuantity(pmmClientRequestCPULarge, resource.Milli),
		},
	}
)

// getPMMResources returns the PMM resources to be used for the DB cluster.
// The logic is as follows:
//  1. If this is a new DB cluster, use the resources specified in the DB spec, if any.
//     Otherwise, use the default resources based on the DB size.
//  2. If this is an existing DB cluster and the DB size has changed, use the resources specified in the DB spec, if any.
//     Otherwise, use the default resources based on the new DB size.
//  3. If this is an existing DB cluster and PMM was enabled before, use the resources specified in the DB spec, if any.
//     Otherwise, use the current PMM resources.
//  4. If this is an existing DB cluster and PMM was not enabled before, use the resources specified in the DB spec, if any.
//     Otherwise, use the default resources based on the DB size.
func getPMMResources(c *sdk.Context, curPsmdbSpec *psmdbv1.PerconaServerMongoDBSpec,
) corev1.ResourceRequirements {
	monitoring := c.DB().Spec.Components[ComponentMonitoring]

	requestedResources := corev1.ResourceRequirements{}
	if monitoring.Resources != nil {
		requestedResources = *monitoring.Resources
	}

	engine := c.DB().Spec.Components[ComponentEngine]
	engineLimitsMemory := resource.Quantity{}
	if engine.Resources != nil {
		engineLimitsMemory = engine.Resources.Limits[corev1.ResourceMemory]
	}

	if c.DB().Status.Phase == v2alpha1.DataStorePhaseCreating {
		// This is new DB cluster.
		// DB spec may contain custom PMM resources -> merge them with defaults.
		// If none are specified, default resources will be used.

		return mergeResources(requestedResources, calculatePMMResources(engineLimitsMemory))
	}

	// Prepare current DB cluster size
	var currentReplSet psmdbv1.ReplsetSpec
	for _, replset := range curPsmdbSpec.Replsets {
		if replset.Name == rsName(0) {
			currentReplSet = *replset
			break
		}
	}

	if !equalSize(engineLimitsMemory, *currentReplSet.Resources.Requests.Memory()) {
		// DB cluster size has changed -> need to update PMM resources.
		// DB spec may contain custom PMM resources -> merge them with defaults.
		return mergeResources(requestedResources, calculatePMMResources(engineLimitsMemory))
	}

	if curPsmdbSpec.PMM.Enabled {
		// DB cluster is not new and PMM was enabled before.
		// DB spec may contain new custom PMM resources -> merge them with previously used PMM resources.
		return mergeResources(requestedResources, curPsmdbSpec.PMM.Resources)
	}

	// DB cluster is not new and PMM was not enabled before. Now it is being enabled.
	// DB spec may contain custom PMM resources -> merge them with defaults.
	return mergeResources(requestedResources, calculatePMMResources(engineLimitsMemory))
}

// equalSize checks if two memory sizes fall into the same predefined size category.
func equalSize(a, b resource.Quantity) bool {
	switch {
	case a.Cmp(MemoryLargeSize) >= 0:
		// a is large size -> b must be large size
		return b.Cmp(MemoryLargeSize) >= 0
	case a.Cmp(MemoryMediumSize) >= 0:
		// a is medium size -> b must be medium size
		return b.Cmp(MemoryMediumSize) >= 0 && b.Cmp(MemoryLargeSize) == -1
	default:
		// a is small size -> b must be small size (less than medium)
		return b.Cmp(MemoryMediumSize) == -1
	}
}

// calculatePMMResources returns the resource requirements for PMM based on memoery size.
func calculatePMMResources(m resource.Quantity) corev1.ResourceRequirements {
	if m.Cmp(MemoryLargeSize) >= 0 {
		return PmmResourceRequirementsLarge
	}

	if m.Cmp(MemoryMediumSize) >= 0 {
		return PmmResourceRequirementsMedium
	}

	return PmmResourceRequirementsSmall
}

// mergeResources merges highPriorityResources and lowPriorityResources.
// If a resource is specified in both, the value from highPriorityResources is used.
// If a resource is only specified in one, that value is used.
func mergeResources(highPriorityResources, lowPriorityResources corev1.ResourceRequirements) corev1.ResourceRequirements {
	mergedResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}

	// CPU Requests
	if highPriorityResources.Requests != nil &&
		highPriorityResources.Requests.Cpu() != nil &&
		!highPriorityResources.Requests.Cpu().IsZero() {
		mergedResources.Requests[corev1.ResourceCPU] = *highPriorityResources.Requests.Cpu()
	} else if lowPriorityResources.Requests != nil &&
		lowPriorityResources.Requests.Cpu() != nil &&
		!lowPriorityResources.Requests.Cpu().IsZero() {
		mergedResources.Requests[corev1.ResourceCPU] = *lowPriorityResources.Requests.Cpu()
	}

	// Memory Requests
	if highPriorityResources.Requests != nil &&
		highPriorityResources.Requests.Memory() != nil &&
		!highPriorityResources.Requests.Memory().IsZero() {
		mergedResources.Requests[corev1.ResourceMemory] = *highPriorityResources.Requests.Memory()
	} else if lowPriorityResources.Requests != nil &&
		lowPriorityResources.Requests.Memory() != nil &&
		!lowPriorityResources.Requests.Memory().IsZero() {
		mergedResources.Requests[corev1.ResourceMemory] = *lowPriorityResources.Requests.Memory()
	}

	// CPU Limits
	if highPriorityResources.Limits != nil &&
		highPriorityResources.Limits.Cpu() != nil &&
		!highPriorityResources.Limits.Cpu().IsZero() {
		mergedResources.Limits[corev1.ResourceCPU] = *highPriorityResources.Limits.Cpu()
	} else if lowPriorityResources.Limits != nil &&
		lowPriorityResources.Limits.Cpu() != nil &&
		!lowPriorityResources.Limits.Cpu().IsZero() {
		mergedResources.Limits[corev1.ResourceCPU] = *lowPriorityResources.Limits.Cpu()
	}

	// Memory Limits
	if highPriorityResources.Limits != nil &&
		highPriorityResources.Limits.Memory() != nil &&
		!highPriorityResources.Limits.Memory().IsZero() {
		mergedResources.Limits[corev1.ResourceMemory] = *highPriorityResources.Limits.Memory()
	} else if lowPriorityResources.Limits != nil &&
		lowPriorityResources.Limits.Memory() != nil &&
		!lowPriorityResources.Limits.Memory().IsZero() {
		mergedResources.Limits[corev1.ResourceMemory] = *lowPriorityResources.Limits.Memory()
	}

	return mergedResources
}
