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
	"errors"
	"fmt"
	"regexp"
	"strings"

	corev1alpha1 "github.com/openeverest/openeverest/v2/api/core/v1alpha1"
)

var kindPattern = regexp.MustCompile(corev1alpha1.KindPattern)

// componentKinds reads the optional componentTypes block of provider.yaml,
// which declares the kind of each component type (e.g. mongod: {kind:
// mongodb}), and returns the kind per type. Every key must be the type of a
// component in provider.yaml and every entry must set a kind matching the
// core pattern; every problem is reported at once.
func componentKinds(provider map[string]any, components map[string]any) (map[string]string, error) {
	raw, ok := provider["componentTypes"]
	if !ok || raw == nil {
		return nil, nil
	}
	types, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("componentTypes: must be a map of component type to {kind}")
	}

	declaredTypes := declaredComponentTypes(components)
	declared := "none"
	if len(declaredTypes) > 0 {
		declared = strings.Join(sortedKeys(declaredTypes), ", ")
	}

	kinds := make(map[string]string, len(types))
	var errs []error
	for _, typeName := range sortedKeys(types) {
		if !declaredTypes[typeName] {
			errs = append(errs, fmt.Errorf("componentTypes[%q]: no component in provider.yaml has type %q (declared types: %s)", typeName, typeName, declared))
		}

		entry, ok := types[typeName].(map[string]any)
		if !ok {
			errs = append(errs, fmt.Errorf("componentTypes[%q]: must be a map with a kind", typeName))
			continue
		}
		for _, field := range sortedKeys(entry) {
			switch field {
			case "kind":
			case "versions", "defaultVersion":
				errs = append(errs, fmt.Errorf("componentTypes[%q].%s: unknown field; version catalogs belong in versions.yaml", typeName, field))
			default:
				errs = append(errs, fmt.Errorf("componentTypes[%q].%s: unknown field; provider.yaml componentTypes declare only kind", typeName, field))
			}
		}

		kind, _ := entry["kind"].(string)
		switch {
		case kind == "":
			errs = append(errs, fmt.Errorf("componentTypes[%q].kind: required, a string such as mongodb, mysql or haproxy", typeName))
		case !kindPattern.MatchString(kind):
			errs = append(errs, fmt.Errorf("componentTypes[%q].kind: %q does not match %s", typeName, kind, kindPattern))
		default:
			kinds[typeName] = kind
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return kinds, nil
}

// checkVersionsDeclareNoKind rejects kind in the version catalogs of
// versions.yaml: provider.yaml is its only source.
func checkVersionsDeclareNoKind(componentTypes map[string]any) error {
	var errs []error
	for _, typeName := range sortedKeys(componentTypes) {
		if entry, ok := componentTypes[typeName].(map[string]any); ok {
			if _, has := entry["kind"]; has {
				errs = append(errs, fmt.Errorf("componentTypes[%q].kind: declare the kind in provider.yaml (componentTypes.%s.kind), not in versions.yaml", typeName, typeName))
			}
		}
	}
	return errors.Join(errs...)
}

// withKinds returns componentTypes, the version catalogs of versions.yaml,
// with each type's kind from provider.yaml added, so the generator emits
// Provider.spec.componentTypes.<type>.kind next to its versions.
func withKinds(componentTypes map[string]any, kinds map[string]string) map[string]any {
	if len(kinds) == 0 {
		return componentTypes
	}
	if componentTypes == nil {
		componentTypes = make(map[string]any, len(kinds))
	}
	for typeName, kind := range kinds {
		entry, ok := componentTypes[typeName].(map[string]any)
		if !ok {
			entry = make(map[string]any, 1)
			componentTypes[typeName] = entry
		}
		entry["kind"] = kind
	}
	return componentTypes
}
