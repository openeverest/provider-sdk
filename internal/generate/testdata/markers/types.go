// Copyright (C) 2026 The OpenEverest Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package markers holds parameter types annotated with kubebuilder markers.
package markers

// EngineParameters exercises the markers a parameters type can carry.
// +kubebuilder:validation:XValidation:rule="!has(self.cacheSizeGB) || self.cacheSizeGB <= self.maxCacheSizeGB",message="cacheSizeGB must not exceed maxCacheSizeGB"
type EngineParameters struct {
	// CacheSizeGB is the cache size in gigabytes.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=64
	// +optional
	CacheSizeGB *int32 `json:"cacheSizeGB,omitempty"`

	// MaxCacheSizeGB bounds cacheSizeGB.
	MaxCacheSizeGB int32 `json:"maxCacheSizeGB"`

	// Compression is the block compressor.
	// +kubebuilder:validation:Enum=snappy;zstd
	// +kubebuilder:default=snappy
	// +optional
	Compression string `json:"compression,omitempty"`

	// Tags are free-form labels.
	// +kubebuilder:validation:MaxItems=4
	// +optional
	Tags []Tag `json:"tags,omitempty"`
}

// Tag is a label.
type Tag struct {
	// Name of the tag.
	// +kubebuilder:validation:Pattern=`^[a-z]+$`
	Name string `json:"name"`
}
