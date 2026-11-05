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

package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

const markersPackage = "./testdata/markers"

func TestResolveSchemasEmitsMarkers(t *testing.T) {
	schemas, err := ResolveSchemas([]string{markersPackage}, []string{"EngineParameters"})
	require.NoError(t, err)

	want := `
description: EngineParameters exercises the markers a parameters type can carry.
properties:
  cacheSizeGB:
    description: CacheSizeGB is the cache size in gigabytes.
    format: int32
    maximum: 64
    minimum: 1
    type: integer
  compression:
    default: snappy
    description: Compression is the block compressor.
    enum:
    - snappy
    - zstd
    type: string
  maxCacheSizeGB:
    description: MaxCacheSizeGB bounds cacheSizeGB.
    format: int32
    type: integer
  tags:
    description: Tags are free-form labels.
    items:
      description: Tag is a label.
      properties:
        name:
          description: Name of the tag.
          pattern: ^[a-z]+$
          type: string
      required:
      - name
      type: object
    maxItems: 4
    type: array
required:
- maxCacheSizeGB
type: object
x-kubernetes-validations:
- message: cacheSizeGB must not exceed maxCacheSizeGB
  rule: '!has(self.cacheSizeGB) || self.cacheSizeGB <= self.maxCacheSizeGB'
`
	var expected any
	require.NoError(t, yaml.Unmarshal([]byte(want), &expected))
	assert.Equal(t, expected, schemas["EngineParameters"])
}

func TestResolveSchemasUnknownType(t *testing.T) {
	_, err := ResolveSchemas([]string{markersPackage}, []string{"MissingParameters"})
	require.ErrorContains(t, err, `type "MissingParameters" not found`)
}
