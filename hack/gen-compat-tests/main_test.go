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

func TestDevelopmentSuite(t *testing.T) {
	t.Parallel()

	data, err := developmentSuite()
	require.NoError(t, err)
	var got suite
	require.NoError(t, yaml.Unmarshal(data, &got))
	var v vectors
	require.NoError(t, json.Unmarshal(contract.OrderVectors, &v))
	require.NotEmpty(t, v.Development)

	helper := map[string]string{}
	stamps := map[string]string{}
	minimums := map[string]string{}
	for _, tt := range got.Tests {
		require.Len(t, tt.Asserts, 1, tt.It)
		switch tt.Template {
		case "templates/development.yaml":
			helper[tt.Set["version"]] = tt.Asserts[0].(map[string]any)["equal"].(map[string]any)["value"].(string)
		case "templates/check.yaml":
			pattern := tt.Asserts[0].(map[string]any)["failedTemplate"].(map[string]any)["errorPattern"].(string)
			annotations := tt.KubernetesProvider.Objects[0].(map[string]any)["metadata"].(map[string]any)["annotations"].(map[string]any)
			if minimum := annotations[contract.MinVersionAnnotation].(string); minimum == oldestCore {
				stamps[tt.Set["stamp"]] = pattern
			} else {
				assert.Equal(t, oldestCore, tt.Set["stamp"], tt.It)
				minimums[minimum] = pattern
			}
		default:
			t.Fatalf("%s: unexpected template %q", tt.It, tt.Template)
		}
	}

	var development, release int
	for _, c := range v.Development {
		isDevelopment := contract.IsDevelopment(c.Version)
		want := ""
		if isDevelopment {
			want = "true"
			development++
		} else {
			release++
		}
		assert.Equal(t, want, helper[c.Version], "development %q", c.Version)
		if !contract.Valid(c.Version) {
			assert.NotContains(t, stamps, c.Version)
			assert.NotContains(t, minimums, c.Version)
			continue
		}
		require.Contains(t, stamps, c.Version)
		require.Contains(t, minimums, c.Version)
		assert.Equal(t, isDevelopment, strings.Contains(stamps[c.Version], "unreleased"), "message for the stamp %q: %q", c.Version, stamps[c.Version])
		assert.Equal(t, isDevelopment, strings.Contains(minimums[c.Version], "unreleased"), "message for the minimum %q: %q", c.Version, minimums[c.Version])
	}
	assert.Len(t, helper, len(v.Development), "a helper case per row")
	assert.Positive(t, development, "no development value")
	assert.Positive(t, release, "no value that is not a development value")
}

func TestAcknowledgeSuite(t *testing.T) {
	t.Parallel()

	data, err := acknowledgeSuite()
	require.NoError(t, err)
	var got suite
	require.NoError(t, yaml.Unmarshal(data, &got))
	var v vectors
	require.NoError(t, json.Unmarshal(contract.OrderVectors, &v))
	require.NotEmpty(t, v.Acknowledge)

	helper := map[[3]string]string{}
	inValues := map[[3]string]bool{}
	listed := map[[3]string]bool{}
	for _, tt := range got.Tests {
		require.Len(t, tt.Asserts, 1, tt.It)
		switch tt.Template {
		case "templates/acknowledgement.yaml":
			key := [3]string{tt.Set["ack"], tt.Set["contractVersion"], tt.Set["coreVersion"]}
			helper[key] = tt.Asserts[0].(map[string]any)["equal"].(map[string]any)["value"].(string)
		case "templates/check.yaml":
			assert.Equal(t, newestStamp, tt.Set["stamp"], tt.It)
			obj := tt.KubernetesProvider.Objects[0].(map[string]any)["metadata"].(map[string]any)
			annotations, labels := obj["annotations"].(map[string]any), obj["labels"].(map[string]any)
			contractVersion, coreVersion := annotations[contract.VersionAnnotation].(string), labels[contract.CoreVersionKey].(string)
			_, passes := tt.Asserts[0].(map[string]any)["containsDocument"]
			if list, ok := annotations[contract.AcknowledgedExtensionsAnnotation].(string); ok {
				listed[[3]string{strings.TrimPrefix(list, fixtureProvider+"@"), contractVersion, coreVersion}] = passes
			} else {
				inValues[[3]string{tt.Set[contract.AcknowledgeValuePath], contractVersion, coreVersion}] = passes
			}
		default:
			t.Fatalf("%s: unexpected template %q", tt.It, tt.Template)
		}
	}

	var holding, notHolding int
	for _, c := range v.Acknowledge {
		key := [3]string{c.Ack, c.ContractVersion, c.CoreVersion}
		holds := contract.AcknowledgementHolds(c.Ack, c.ContractVersion, c.CoreVersion)
		want := ""
		if holds {
			want = "true"
			holding++
		} else {
			notHolding++
		}
		assert.Equal(t, want, helper[key], "acknowledgementHolds%q", key)
		if !contract.Valid(c.ContractVersion) {
			continue
		}
		assert.Equal(t, holds, inValues[key], "check with the acknowledgement in values %q", key)
		if strings.TrimSpace(c.Ack) == c.Ack {
			assert.Equal(t, holds, listed[key], "check with the acknowledgement listed %q", key)
		}
	}
	assert.Len(t, helper, len(v.Acknowledge), "a helper case per row")
	assert.Positive(t, holding, "no case that holds")
	assert.Positive(t, notHolding, "no case that does not hold")
}
