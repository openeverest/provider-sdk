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
	"fmt"
	"go/types"
	"strings"
)

// componentSpecType is the struct a supportedFields declaration selects from.
const componentSpecType = "ComponentSpec"

// parametersSegment is governed by parametersSchema, not by supportedFields.
const parametersSegment = "parameters"

// componentPathPrefix is what a UI path looks like before the component name.
const componentPathPrefix = "spec.components."

// FieldIssue is a supportedFields declaration that does not describe a real
// selection from ComponentSpec, or a form control bound to a core field the
// component does not declare it honours.
type FieldIssue struct {
	Topology  string
	Component string
	Path      string
	Reason    string
}

func (i FieldIssue) String() string {
	return fmt.Sprintf("topology %s: component %q: %s: %s", i.Topology, i.Component, i.Path, i.Reason)
}

// ValidateSupportedFields checks that every supportedFields declaration selects
// from ComponentSpec: each property names a real field at that path, and none
// changes the type of the field it selects. A declaration may narrow a field --
// required, bounds, CEL -- but re-typing one would fork ComponentSpec per
// provider and take the codegen chain and the UI with it.
func ValidateSupportedFields(cfg *AssembledConfig, pkgPatterns []string) ([]FieldIssue, error) {
	return validateSupportedFields(cfg, append([]string{corePackage}, pkgPatterns...))
}

func validateSupportedFields(cfg *AssembledConfig, pkgPatterns []string) ([]FieldIssue, error) {
	declarations := allDeclarations(cfg)
	if len(declarations) == 0 {
		return nil, nil
	}

	pkgs, err := loadPackages(pkgPatterns)
	if err != nil {
		return nil, fmt.Errorf("loading packages to validate supportedFields: %w", err)
	}
	componentSpec, err := lookupType(pkgs, componentSpecType)
	if err != nil {
		return nil, err
	}

	var issues []FieldIssue
	for _, d := range declarations {
		for _, issue := range checkSelection(componentSpec, d.schema, "") {
			issue.Topology, issue.Component = d.topology, d.component
			issues = append(issues, issue)
		}
	}
	return issues, nil
}

// ValidateUIPathsAreDeclared checks that every core field a topology's form
// binds a control to is one the component declares it honours. A form offering
// a field the API rejects is worse than the silent no-op this declaration
// exists to remove, so the two are reconciled here rather than left to drift.
//
// Only `path` values count: a field merely referenced by a CEL expression is
// read by the form, not written by it.
func ValidateUIPathsAreDeclared(cfg *AssembledConfig) []FieldIssue {
	var issues []FieldIssue

	for _, topology := range sortedKeys(cfg.UISchema) {
		declared := declarationsFor(cfg, topology)
		for _, ref := range collectPathRefs(cfg.UISchema[topology]) {
			if ref.source != "path" {
				continue
			}
			component, segments, ok := splitComponentPath(ref.path)
			if !ok || segments[0] == parametersSegment {
				continue
			}
			// A component that declares nothing is unconstrained.
			schema, ok := declared[component]
			if !ok {
				continue
			}
			if !declaresPath(schema, segments) {
				issues = append(issues, FieldIssue{
					Topology:  topology,
					Component: component,
					Path:      ref.path,
					Reason:    "bound by the UI, but not declared in supportedFields",
				})
			}
		}
	}
	return issues
}

// splitComponentPath breaks spec.components.<name>.<field>... into the
// component and the segments below it.
func splitComponentPath(path string) (string, []string, bool) {
	rest, ok := strings.CutPrefix(path, componentPathPrefix)
	if !ok {
		return "", nil, false
	}
	component, field, ok := strings.Cut(rest, ".")
	if !ok {
		return "", nil, false
	}
	return component, strings.Split(field, "."), true
}

// declaresPath reports whether the schema covers the path. A property declared
// without nested properties covers everything beneath it, which is what makes
// `schedulingPolicy: {}` a promise about the whole struct.
func declaresPath(schema map[string]any, segments []string) bool {
	current := schema
	for _, segment := range segments {
		properties, ok := current["properties"].(map[string]any)
		if !ok || len(properties) == 0 {
			return true
		}
		next, ok := properties[segment].(map[string]any)
		if !ok {
			return false
		}
		current = next
	}
	return true
}

// checkSelection walks a declared schema against the Go type it selects from.
func checkSelection(t types.Type, schema map[string]any, at string) []FieldIssue {
	// Named through the pointer: typeName on a *T falls back to the
	// fully-qualified string, which reads badly in an error.
	owner := typeName(deref(t))

	structure, ok := deref(t).Underlying().(*types.Struct)
	if !ok {
		return []FieldIssue{{
			Path:   at,
			Reason: fmt.Sprintf("declares properties, but %s has none to select", owner),
		}}
	}

	properties, _ := schema["properties"].(map[string]any)

	var issues []FieldIssue
	for _, name := range sortedKeys(properties) {
		property, _ := properties[name].(map[string]any)
		path := at + "." + name

		field, found := fieldByJSONTag(structure, name)
		if !found {
			issues = append(issues, FieldIssue{
				Path:   path,
				Reason: fmt.Sprintf("no such field on %s", owner),
			})
			continue
		}
		if declared, _ := property["type"].(string); declared != "" {
			if want := jsonType(field.Type()); want != "" && declared != want {
				issues = append(issues, FieldIssue{
					Path: path,
					Reason: fmt.Sprintf(
						"declared as %q but the field is %q; a declaration may narrow a field, never re-type it",
						declared, want),
				})
				continue
			}
		}
		if nested, ok := property["properties"].(map[string]any); ok && len(nested) > 0 {
			issues = append(issues, checkSelection(field.Type(), property, path)...)
		}
	}

	for _, name := range requiredNames(schema) {
		if _, ok := properties[name]; !ok {
			issues = append(issues, FieldIssue{
				Path:   at,
				Reason: fmt.Sprintf("required lists %q, which the schema does not declare", name),
			})
		}
	}
	return issues
}

// jsonType reports the OpenAPI type a Go type serializes as, or "" when it has
// no single one. Quantity is the exception that matters: a struct in Go and a
// string on the wire, so the naive answer would reject an honest declaration.
func jsonType(t types.Type) string {
	t = deref(t)
	if typeName(t) == "Quantity" {
		return "string"
	}
	switch underlying := t.Underlying().(type) {
	case *types.Basic:
		info := underlying.Info()
		switch {
		case info&types.IsBoolean != 0:
			return "boolean"
		case info&types.IsInteger != 0:
			return "integer"
		case info&types.IsFloat != 0:
			return "number"
		case info&types.IsString != 0:
			return "string"
		}
	case *types.Struct, *types.Map:
		return "object"
	case *types.Slice, *types.Array:
		return "array"
	}
	return ""
}

// declaration is one component's supportedFields within one topology.
type declaration struct {
	topology  string
	component string
	schema    map[string]any
}

func allDeclarations(cfg *AssembledConfig) []declaration {
	var out []declaration
	for _, topology := range sortedKeys(cfg.Topologies) {
		for _, component := range sortedKeys(declarationsFor(cfg, topology)) {
			out = append(out, declaration{
				topology:  topology,
				component: component,
				schema:    declarationsFor(cfg, topology)[component],
			})
		}
	}
	return out
}

// declarationsFor returns each declared component schema in a topology,
// unwrapped from the ParametersSchema envelope.
func declarationsFor(cfg *AssembledConfig, topology string) map[string]map[string]any {
	out := map[string]map[string]any{}

	topo, ok := cfg.Topologies[topology].(map[string]any)
	if !ok {
		return out
	}
	components, ok := topo["components"].(map[string]any)
	if !ok {
		return out
	}
	for name, raw := range components {
		component, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		envelope, ok := component["supportedFields"].(map[string]any)
		if !ok {
			continue
		}
		if schema, ok := envelope["openAPIV3Schema"].(map[string]any); ok {
			out[name] = schema
		}
	}
	return out
}

func requiredNames(schema map[string]any) []string {
	raw, ok := schema["required"].([]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(raw))
	for _, entry := range raw {
		if name, ok := entry.(string); ok {
			names = append(names, name)
		}
	}
	return names
}
