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

import "strings"

// InjectDefaultValuePaths adds optionsPathConfig.defaultValuePath to version
// pickers so the UI can pre-select the provider's declared default
// (openeverest/openeverest#3265) without the renderer knowing the Provider CR
// shape. A picker qualifies when its optionsPath addresses a version list
// whose owner declares a default:
//
//	spec.versions                       → spec.defaultVersion
//	spec.componentTypes.<type>.versions → spec.componentTypes.<type>.defaultVersion
//
// An author-set defaultValuePath is never overwritten, and nothing is injected
// when the corresponding defaultVersion is not declared in versions.yaml.
func InjectDefaultValuePaths(cfg *AssembledConfig) {
	for _, schema := range cfg.UISchema {
		walkMaps(schema, func(m map[string]any) {
			optionsPath, _ := m["optionsPath"].(string)
			defaultPath := defaultValuePathFor(cfg, optionsPath)
			if defaultPath == "" {
				return
			}
			config, ok := m["optionsPathConfig"].(map[string]any)
			if !ok {
				config = map[string]any{}
				m["optionsPathConfig"] = config
			}
			if _, set := config["defaultValuePath"]; !set {
				config["defaultValuePath"] = defaultPath
			}
		})
	}
}

// defaultValuePathFor maps an optionsPath to the defaultVersion field of the
// list it addresses, or "" when it is not a version list or no default is
// declared.
func defaultValuePathFor(cfg *AssembledConfig, optionsPath string) string {
	if optionsPath == "spec.versions" {
		if cfg.DefaultVersion == "" {
			return ""
		}
		return "spec.defaultVersion"
	}

	typeName, found := strings.CutPrefix(optionsPath, "spec.componentTypes.")
	if !found {
		return ""
	}
	typeName, found = strings.CutSuffix(typeName, ".versions")
	if !found || typeName == "" || strings.Contains(typeName, ".") {
		return ""
	}
	if nestedString(cfg.ComponentTypes[typeName], "defaultVersion") == "" {
		return ""
	}
	return "spec.componentTypes." + typeName + ".defaultVersion"
}

func walkMaps(node any, visit func(map[string]any)) {
	switch t := node.(type) {
	case map[string]any:
		visit(t)
		for _, v := range t {
			walkMaps(v, visit)
		}
	case []any:
		for _, v := range t {
			walkMaps(v, visit)
		}
	}
}
