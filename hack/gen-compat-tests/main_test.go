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

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/openeverest/openeverest/v2/contract"
)

// TestFilesAreFresh fails when a committed file differs from what this
// command writes: run `make gen` and commit the result.
func TestFilesAreFresh(t *testing.T) {
	t.Parallel()

	files, err := Files()
	require.NoError(t, err)
	for path, want := range files {
		got, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
		require.NoError(t, err)
		assert.Equal(t, string(want), string(got), "%s is stale: run `make gen`", path)
	}
}

func TestVectorsSuite(t *testing.T) {
	t.Parallel()

	data, err := vectorsSuite()
	require.NoError(t, err)
	var got suite
	require.NoError(t, yaml.Unmarshal(data, &got))
	var v vectors
	require.NoError(t, json.Unmarshal(contract.OrderVectors, &v))

	assert.Equal(t, []string{"templates/verdict.yaml"}, got.Templates)
	wantTests := len(v.Check) + 3*len(v.Order) - 2 + 2*len(v.Invalid)
	require.Len(t, got.Tests, wantTests, "a test per check row, order step and invalid value")

	counts := map[string]int{}
	for _, tt := range got.Tests {
		reason := tt.Asserts[1].(map[string]any)["equal"].(map[string]any)["value"].(string)
		counts[reason]++
		name, _, _ := strings.Cut(tt.It, ":")
		ext := tt.Set["ext"]
		annotations := tt.KubernetesProvider.Objects[0].(map[string]any)["metadata"].(map[string]any)["annotations"].(map[string]any)
		minimum, current := annotations[contract.MinVersionAnnotation].(string), annotations[contract.VersionAnnotation].(string)
		assert.Equal(t, contract.Check(ext, minimum, current).Reason, reason, "%s: %q in [%q, %q]", name, ext, minimum, current)
	}
	for _, reason := range []string{
		contract.ReasonSupported,
		contract.ReasonCoreTooOld,
		contract.ReasonExtensionTooOld,
		contract.ReasonUnversioned,
		contract.ReasonInvalidVersion,
		contract.ReasonCoreVersionSkew,
	} {
		assert.Positive(t, counts[reason], "no case for %s", reason)
	}
}
