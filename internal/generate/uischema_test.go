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

	"github.com/openeverest/openeverest/v2/provider-runtime/uischema"
	"github.com/stretchr/testify/assert"
)

// A toggleable group holding only a podSchedulingPolicy widget is valid when
// the topology declares a component accepting affinity.
func TestValidateUISchemaResolvesWidgetPaths(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		schedulingPolicy map[string]any
		want             []uischema.Rule
	}{
		"component accepts affinity": {schedulingPolicy: map[string]any{"properties": map[string]any{"affinity": map[string]any{}}}},
		"no component accepts it":    {want: []uischema.Rule{uischema.RuleToggleableNoFields}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			properties := map[string]any{}
			if tc.schedulingPolicy != nil {
				properties["schedulingPolicy"] = tc.schedulingPolicy
			}
			cfg := configWithUISchema(map[string]any{"sections": map[string]any{"advanced": map[string]any{"components": map[string]any{
				"scheduling": map[string]any{"uiType": "group", "groupType": "toggleable", "components": map[string]any{
					"policy": map[string]any{"uiType": "widget", "widgetType": "podSchedulingPolicy"},
				}},
			}}}})
			cfg.Topologies["simple"] = buildTopologySpec(map[string]any{"components": map[string]any{
				"engine": map[string]any{"supportedFields": map[string]any{"properties": properties}},
			}}, nil)

			var got []uischema.Rule
			for _, issue := range validateUISchema(cfg) {
				got = append(got, issue.Rule)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}
