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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeDefinition(t *testing.T, versionsYAML string) string {
	t.Helper()
	dir := t.TempDir()
	providerYAML := "name: test-provider\ncomponents:\n  engine:\n    type: mongod\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "provider.yaml"), []byte(providerYAML), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "versions.yaml"), []byte(versionsYAML), 0o600))
	return dir
}

func TestAssembleParsesReleaseBlock(t *testing.T) {
	t.Parallel()

	cfg, err := Assemble(writeDefinition(t, `
componentTypes:
  mongod:
    versions:
      - version: "8.0.12-4"
        image: percona/psmdb:8.0.12-4
        default: true
release:
  minUpgradableFrom: "0.2"
`))

	require.NoError(t, err)
	assert.Equal(t, "0.2", cfg.MinUpgradableFrom)
}

func TestAssembleWithoutReleaseBlock(t *testing.T) {
	t.Parallel()

	cfg, err := Assemble(writeDefinition(t, `
componentTypes:
  mongod:
    versions:
      - version: "8.0.12-4"
        image: percona/psmdb:8.0.12-4
`))

	require.NoError(t, err)
	assert.Empty(t, cfg.MinUpgradableFrom)
}

func TestBuildReleaseSpec(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		map[string]any{"version": "0.3", "minUpgradableFrom": "0.2"},
		buildReleaseSpec("0.3", "0.2"))
	assert.Equal(t,
		map[string]any{"version": "0.3"},
		buildReleaseSpec("0.3", ""))
	assert.Nil(t, buildReleaseSpec("", ""))
}

func TestChartAppVersion(t *testing.T) {
	t.Parallel()

	chartDir := t.TempDir()
	chartYAML := "apiVersion: v2\nname: provider-test\nversion: 0.3.1\nappVersion: \"0.3\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYAML), 0o600))

	appVersion, err := chartAppVersion(chartDir)

	require.NoError(t, err)
	assert.Equal(t, "0.3", appVersion)
}

func TestBuildSpecMapPassesThroughDeprecationFlags(t *testing.T) {
	t.Parallel()

	cfg, err := Assemble(writeDefinition(t, `
componentTypes:
  mongod:
    versions:
      - version: "6.0.19-16"
        image: percona/psmdb:6.0.19-16
        deprecated: true
        removedInVersion: "0.3"
      - version: "8.0.12-4"
        image: percona/psmdb:8.0.12-4
        default: true
`))
	require.NoError(t, err)

	spec := buildSpecMap(cfg, nil, nil, nil)

	versions := spec["componentTypes"].(map[string]any)["mongod"].(map[string]any)["versions"].([]any)
	legacy := versions[0].(map[string]any)
	assert.Equal(t, true, legacy["deprecated"])
	assert.Equal(t, "0.3", legacy["removedInVersion"])
}
