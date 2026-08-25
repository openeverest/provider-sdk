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
	"reflect"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// corePackage is the package holding the Instance CRD types that UI schema
// paths address. It is loaded from the provider's own module, so paths are
// checked against the core version that provider is pinned to.
const corePackage = "github.com/openeverest/openeverest/v2/api/core/v1alpha1"

// rawExtensionType marks the boundary between the Instance schema and a
// provider-declared parameters payload. It is matched by name because the SDK
// does not depend on apimachinery.
const rawExtensionType = "RawExtension"

// celPathRef matches `spec.a.b.c` inside a CEL expression, including the
// `original.` prefix used to address the pre-update object.
var celPathRef = regexp.MustCompile(`\b(?:original\.)?(spec(?:\.[a-zA-Z0-9_]+)+)`)

// PathIssue is a UI schema reference that does not address a real API field.
type PathIssue struct {
	Topology string
	Source   string // "path" or "celExpr"
	Path     string
	Reason   string
}

func (i PathIssue) String() string {
	return fmt.Sprintf("topology %s: %s %q: %s", i.Topology, i.Source, i.Path, i.Reason)
}

// pathRef is one addressable reference found in a UI schema.
type pathRef struct {
	source string
	path   string
}

// ValidateUISchemaPaths checks that every field reference in the UI schemas
// addresses a real field of InstanceSpec. The UI schemas are copied verbatim
// into the generated Provider CR, so an unresolvable path would otherwise be
// published to clusters and silently break a form.
func ValidateUISchemaPaths(cfg *AssembledConfig, pkgPatterns []string) ([]PathIssue, error) {
	return validateUISchemaPaths(cfg, append([]string{corePackage}, pkgPatterns...))
}

func validateUISchemaPaths(cfg *AssembledConfig, pkgPatterns []string) ([]PathIssue, error) {
	if len(cfg.UISchema) == 0 {
		return nil, nil
	}

	pkgs, err := loadPackages(pkgPatterns)
	if err != nil {
		return nil, fmt.Errorf("loading packages to validate UI schema paths: %w", err)
	}

	instanceSpec, err := lookupType(pkgs, "InstanceSpec")
	if err != nil {
		return nil, err
	}

	var issues []PathIssue
	for _, topology := range sortedKeys(cfg.UISchema) {
		r := &resolver{
			pkgs:       pkgs,
			cfg:        cfg,
			root:       instanceSpec,
			topology:   topology,
			components: topologyComponentNames(cfg, topology),
		}
		for _, ref := range collectPathRefs(cfg.UISchema[topology]) {
			if reason := r.resolve(ref); reason != "" {
				issues = append(issues, PathIssue{
					Topology: topology,
					Source:   ref.source,
					Path:     ref.path,
					Reason:   reason,
				})
			}
		}
	}
	return issues, nil
}

// collectPathRefs walks a UI schema and returns every field reference in it:
// `path` values, and `spec.`-rooted identifiers inside CEL expressions.
func collectPathRefs(node any) []pathRef {
	var refs []pathRef
	var walk func(any)
	walk = func(n any) {
		switch t := n.(type) {
		case map[string]any:
			for _, key := range sortedKeys(t) {
				v := t[key]
				switch {
				case key == "path":
					if s, ok := v.(string); ok {
						refs = append(refs, pathRef{source: "path", path: s})
					}
				case key == "celExpr":
					if s, ok := v.(string); ok {
						for _, m := range celPathRef.FindAllStringSubmatch(s, -1) {
							refs = append(refs, pathRef{source: "celExpr", path: m[1]})
						}
					}
				default:
					walk(v)
				}
			}
		case []any:
			for _, v := range t {
				walk(v)
			}
		}
	}
	walk(node)
	return refs
}

type resolver struct {
	pkgs       []*packages.Package
	cfg        *AssembledConfig
	root       types.Type
	topology   string
	components map[string]bool
}

// resolve returns an empty string when the reference addresses a real field,
// or a human-readable reason why it does not.
func (r *resolver) resolve(ref pathRef) string {
	rest, ok := strings.CutPrefix(ref.path, "spec.")
	if !ok {
		return ""
	}

	cur := r.root
	var walked []string
	for _, seg := range strings.Split(rest, ".") {
		cur = deref(cur)

		if named, ok := cur.(*types.Named); ok && named.Obj().Name() == rawExtensionType {
			// Everything below is a provider-declared parameters payload.
			schema := r.parametersSchemaFor(walked)
			if schema == "" {
				return fmt.Sprintf("no parametersSchema declared for %q, so %q cannot be verified",
					strings.Join(walked, "."), seg)
			}
			t, err := lookupType(r.pkgs, schema)
			if err != nil {
				return fmt.Sprintf("parametersSchema %q not found: %v", schema, err)
			}
			cur = deref(t)
		}

		switch u := cur.Underlying().(type) {
		case *types.Map:
			if strings.Join(walked, ".") == "components" && !r.components[seg] {
				return fmt.Sprintf("component %q is not part of topology %q (declared: %s)",
					seg, r.topology, strings.Join(sortedKeys(r.components), ", "))
			}
			cur = u.Elem()
		case *types.Struct:
			f, ok := fieldByJSONTag(u, seg)
			if !ok {
				return fmt.Sprintf("no field %q on %s", seg, typeName(cur))
			}
			cur = f.Type()
		default:
			return fmt.Sprintf("%q is a %s, cannot address %q inside it",
				strings.Join(walked, "."), u.String(), seg)
		}
		walked = append(walked, seg)
	}

	return ""
}

// parametersSchemaFor returns the Go type declared for the parameters payload
// at the given position in the Instance spec.
func (r *resolver) parametersSchemaFor(walked []string) string {
	switch {
	case len(walked) == 3 && walked[0] == "components":
		return nestedString(r.cfg.Components[walked[1]], "parametersSchema")
	case len(walked) == 2 && walked[0] == "topology":
		return nestedString(r.cfg.Topologies[r.topology], "parametersSchema")
	case len(walked) == 1:
		return r.cfg.ParametersSchema
	}
	return ""
}

func topologyComponentNames(cfg *AssembledConfig, topology string) map[string]bool {
	names := map[string]bool{}
	topo, ok := cfg.Topologies[topology].(map[string]any)
	if !ok {
		return names
	}
	comps, ok := topo["components"].(map[string]any)
	if !ok {
		return names
	}
	for name := range comps {
		names[name] = true
	}
	return names
}

func nestedString(node any, key string) string {
	m, ok := node.(map[string]any)
	if !ok {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func lookupType(pkgs []*packages.Package, name string) (types.Type, error) {
	for _, pkg := range pkgs {
		if obj := pkg.Types.Scope().Lookup(name); obj != nil {
			return obj.Type(), nil
		}
	}
	return nil, fmt.Errorf("type %q not found in loaded packages", name)
}

func deref(t types.Type) types.Type {
	for {
		p, ok := t.Underlying().(*types.Pointer)
		if !ok {
			return t
		}
		t = p.Elem()
	}
}

// fieldByJSONTag finds the field a JSON key addresses, descending into embedded
// structs whose tag has no name — core inlines those, and their fields are
// addressable as if they were declared on the outer struct.
func fieldByJSONTag(s *types.Struct, name string) (*types.Var, bool) {
	for i := range s.NumFields() {
		f := s.Field(i)
		tag, _, _ := strings.Cut(reflect.StructTag(s.Tag(i)).Get("json"), ",")
		if tag == name {
			return f, true
		}
		if tag != "" || !f.Embedded() {
			continue
		}
		if embedded, ok := deref(f.Type()).Underlying().(*types.Struct); ok {
			if promoted, found := fieldByJSONTag(embedded, name); found {
				return promoted, true
			}
		}
	}
	return nil, false
}

func typeName(t types.Type) string {
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name()
	}
	return t.String()
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
