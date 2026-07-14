package testpkg

type PxcPITRConfig struct {
	// TimeBetweenUploads controls binlog upload interval in seconds.
	// +kubebuilder:default=60
	// +kubebuilder:validation:Minimum=1
	TimeBetweenUploads *float64 `json:"timeBetweenUploads,omitempty"`
	// TimeoutSeconds controls timeout for each PITR upload in seconds.
	// +kubebuilder:default=3600
	// +kubebuilder:validation:Maximum=7200
	TimeoutSeconds *float64 `json:"timeoutSeconds,omitempty"`
}
