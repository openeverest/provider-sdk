package provider

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

// CalculatePMMResources returns the resource requirements for PMM based on memoery size.
func CalculatePMMResources(m resource.Quantity) corev1.ResourceRequirements {
	if m.Cmp(MemoryLargeSize) >= 0 {
		return PmmResourceRequirementsLarge
	}

	if m.Cmp(MemoryMediumSize) >= 0 {
		return PmmResourceRequirementsMedium
	}

	return PmmResourceRequirementsSmall
}

// MergeResources merges highPriorityResources and lowPriorityResources.
// If a resource is specified in both, the value from highPriorityResources is used.
// If a resource is only specified in one, that value is used.
func MergeResources(highPriorityResources, lowPriorityResources corev1.ResourceRequirements) corev1.ResourceRequirements {
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
