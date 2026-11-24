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

package compat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	alpha1 = "v2.0.0-alpha.1"
	pseudo = "v2.0.0-dev.2.0.20261123230825-5a38fe56add1"
)

func core(version string) Module {
	return Module{Path: CoreModule, Version: version}
}

func replaced(version, path, replacementVersion string) Module {
	m := core(version)
	m.Replace = &Module{Path: path, Version: replacementVersion}
	return m
}

func TestCheckCore(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		linked, required Module
		wantWarning      bool
		wantErr          string
	}{
		"same release": {
			linked: core(alpha1), required: core(alpha1),
		},
		"same pseudo-version": {
			linked: core(pseudo), required: core(pseudo),
		},
		"another release": {
			linked: core("v2.0.0-alpha.2"), required: core(alpha1),
			wantErr: "this provider-sdk links " + CoreModule + "@v2.0.0-alpha.2, but this module requires " + CoreModule + "@" + alpha1,
		},
		"same fork": {
			linked:   replaced(alpha1, "github.com/fork/openeverest/v2", "v2.0.0-alpha.1.1"),
			required: replaced(alpha1, "github.com/fork/openeverest/v2", "v2.0.0-alpha.1.1"),
		},
		"fork against upstream": {
			linked:   replaced(alpha1, "github.com/fork/openeverest/v2", "v2.0.0-alpha.1.1"),
			required: core(alpha1),
			wantErr:  "would stamp the chart for another OpenEverest",
		},
		"both from a local directory": {
			linked:      replaced(alpha1, "../openeverest", "(devel)"),
			required:    replaced(alpha1, "../openeverest", ""),
			wantWarning: true,
		},
		"both from a workspace": {
			linked:      core("(devel)"),
			required:    core(""),
			wantWarning: true,
		},
		"only the module replaces it with a directory": {
			linked:   core(alpha1),
			required: replaced(alpha1, "../openeverest", ""),
			wantErr:  "would stamp the chart for another OpenEverest",
		},
		"only the binary was built from a directory": {
			linked:   replaced(alpha1, "/src/openeverest", "(devel)"),
			required: core(alpha1),
			wantErr:  "run it as `go tool provider-sdk`",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			warning, err := CheckCore(tt.linked, tt.required)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.wantWarning {
				assert.Contains(t, warning, "local directory")
			} else {
				assert.Empty(t, warning)
			}
		})
	}
}

// TestLinkedCoreIsRequiredCore checks the guard on this module, whose test
// binary is built with the core its go.mod requires, as `go tool` builds
// provider-sdk with the core of the provider's module.
func TestLinkedCoreIsRequiredCore(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}
	t.Parallel()

	linked, err := LinkedCore()
	require.NoError(t, err)
	required, err := RequiredCore(".")
	require.NoError(t, err)

	assert.Equal(t, required.Version, linked.Version)
	warning, err := CheckCore(linked, required)
	require.NoError(t, err)
	assert.Empty(t, warning)
}

func TestReleaseProblems(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		required Module
		version  string
		want     []string
	}{
		"released core": {
			required: core(alpha1), version: "2.0.0-alpha.1",
		},
		"development contract version": {
			required: core(alpha1), version: "2.0.0-alpha.1.next",
			want: []string{"built against an unreleased OpenEverest (2.0.0-alpha.1.next)"},
		},
		"development contract version after a GA release": {
			required: core("v2.0.0"), version: "2.1.0-alpha.0",
			want: []string{"built against an unreleased OpenEverest (2.1.0-alpha.0)"},
		},
		"pseudo-version, contract changed since the last release": {
			required: core(pseudo), version: "2.0.0-alpha.1.next",
			want: []string{"built against an unreleased OpenEverest (2.0.0-alpha.1.next)"},
		},
		"core dev build": {
			required: core("v2.0.0-dev.4"), version: "2.0.0-dev.4",
		},
		"pseudo-version, contract of a release": {
			required: core(pseudo), version: "2.0.0-alpha.1",
		},
		"replaced by a fork release": {
			required: replaced(alpha1, "github.com/fork/openeverest/v2", "v2.0.0-alpha.1"), version: "2.0.0-alpha.1",
			want: []string{"go.mod replaces " + CoreModule},
		},
		"replaced by a directory": {
			required: replaced(alpha1, "../openeverest", ""), version: "2.0.0-alpha.1",
			want: []string{"go.mod replaces " + CoreModule},
		},
		"workspace module": {
			required: core(""), version: "2.0.0-alpha.1",
			want: []string{CoreModule + " comes from a local directory"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := ReleaseProblems(tt.required, tt.version)

			require.Len(t, got, len(tt.want))
			for i, want := range tt.want {
				assert.Contains(t, got[i], want)
			}
		})
	}
}

func TestFloor(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"2.0.0-alpha.1":      "This build works with OpenEverest 2.0.0-alpha.1 and later.",
		"2.0.0":              "This build works with OpenEverest 2.0.0 and later.",
		"2.0.0-dev.4":        "This build works with OpenEverest 2.0.0-dev.4 and later.",
		"2.0.0-alpha.1.next": "Built against an unreleased OpenEverest (2.0.0-alpha.1.next); it cannot be published until that OpenEverest is released.",
		"2.1.0-alpha.0":      "Built against an unreleased OpenEverest (2.1.0-alpha.0); it cannot be published until that OpenEverest is released.",
	}
	for version, want := range tests {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, want, Floor(version))
		})
	}
}
