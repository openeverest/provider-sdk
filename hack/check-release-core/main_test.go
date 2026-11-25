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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openeverest/provider-sdk/internal/compat"
)

const (
	tag    = "v2.0.0-alpha.2"
	pseudo = "v2.0.0-dev.2.0.20261125185706-a1ddf4732eeb"
)

func core(version string) compat.Module {
	return compat.Module{Path: compat.CoreModule, Version: version}
}

func TestCheck(t *testing.T) {
	t.Parallel()

	const refusal = "provider-sdk releases must require a released OpenEverest; pin core to its tag first"
	directory := core(tag)
	directory.Replace = &compat.Module{Path: "../openeverest"}

	tests := map[string]struct {
		sdk, scaffolded compat.Module
		want            []string
	}{
		"both require a tag": {
			sdk: core(tag), scaffolded: core(tag),
		},
		"go.mod requires a pseudo-version": {
			sdk: core(pseudo), scaffolded: core(tag),
			want: []string{refusal, "go.mod requires " + compat.CoreModule + "@" + pseudo},
		},
		"the scaffold requires a pseudo-version": {
			sdk: core(tag), scaffolded: core(pseudo),
			want: []string{refusal, scaffoldGoMod + " requires " + compat.CoreModule + "@" + pseudo},
		},
		"both require a pseudo-version": {
			sdk: core(pseudo), scaffolded: core(pseudo),
			want: []string{refusal, "\n  - go.mod requires", "\n  - " + scaffoldGoMod + " requires"},
		},
		"go.mod replaces core": {
			sdk: directory, scaffolded: core(tag),
			want: []string{refusal, "go.mod replaces " + compat.CoreModule + " with ../openeverest"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := check(tt.sdk, tt.scaffolded)

			if len(tt.want) == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, want := range tt.want {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}
