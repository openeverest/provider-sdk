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
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/tools/go/packages"
	"sigs.k8s.io/controller-tools/pkg/crd"
	crdmarkers "sigs.k8s.io/controller-tools/pkg/crd/markers"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

// ResolveSchemas generates the OpenAPI schema of each named Go type with
// controller-tools, the parser controller-gen uses for CRDs, so every
// +kubebuilder:validation marker and every doc comment ends up in the schema
// as it would on a CRD.
//
// Types are searched across packages in declaration order; the first match wins.
// The returned map keys are type names and values are map[string]any representations
// of OpenAPI schemas (suitable for YAML marshaling).
func ResolveSchemas(pkgPatterns []string, typeNames []string) (map[string]any, error) {
	if len(typeNames) == 0 {
		return nil, nil
	}

	roots, err := loader.LoadRoots(pkgPatterns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load type packages: %w", err)
	}

	registry := &markers.Registry{}
	if err := crdmarkers.Register(registry); err != nil {
		return nil, fmt.Errorf("registering kubebuilder markers: %w", err)
	}
	parser := &crd.Parser{
		Collector:           &markers.Collector{Registry: registry},
		Checker:             &loader.TypeChecker{},
		AllowDangerousTypes: true,
	}
	crd.AddKnownTypes(parser)
	for _, root := range roots {
		parser.NeedPackage(root)
	}

	schemas := make(map[string]any, len(typeNames))
	for _, name := range typeNames {
		ident, ok := lookupIdent(parser, roots, name)
		if !ok {
			return nil, fmt.Errorf("type %q not found in packages: %s", name, strings.Join(pkgPatterns, ", "))
		}
		parser.NeedFlattenedSchemaFor(ident)
		schema := parser.FlattenedSchemata[ident]
		// Convert to map[string]any via JSON round-trip for clean YAML output.
		jsonBytes, err := json.Marshal(schema)
		if err != nil {
			return nil, fmt.Errorf("marshaling schema for type %q: %w", name, err)
		}
		var result any
		if err := json.Unmarshal(jsonBytes, &result); err != nil {
			return nil, fmt.Errorf("unmarshaling schema for type %q: %w", name, err)
		}
		schemas[name] = result
	}

	if err := packageErrors(roots); err != nil {
		return nil, err
	}
	return schemas, nil
}

// lookupIdent finds the first root package that declares name.
func lookupIdent(parser *crd.Parser, roots []*loader.Package, name string) (crd.TypeIdent, bool) {
	for _, root := range roots {
		ident := crd.TypeIdent{Package: root, Name: name}
		if parser.LookupType(root, name) != nil {
			return ident, true
		}
	}
	return crd.TypeIdent{}, false
}

// packageErrors returns every error recorded while generating schemas: any
// error in the root packages, and anything but type-checking errors in the
// packages they import, as controller-gen does.
func packageErrors(roots []*loader.Package) error {
	isRoot := make(map[*packages.Package]bool, len(roots))
	raw := make([]*packages.Package, 0, len(roots))
	for _, root := range roots {
		isRoot[root.Package] = true
		raw = append(raw, root.Package)
	}

	var errs []string
	packages.Visit(raw, nil, func(pkg *packages.Package) {
		for _, err := range pkg.Errors {
			if err.Kind == packages.TypeError && !isRoot[pkg] {
				continue
			}
			errs = append(errs, err.Error())
		}
	})
	if len(errs) > 0 {
		return fmt.Errorf("generating schemas: %s", strings.Join(errs, "; "))
	}
	return nil
}

// loadPackages loads one or more Go packages for type inspection using go/packages.
// Each pkgPattern can be a relative path (e.g., "./definition/...") or a full import path.
func loadPackages(pkgPatterns []string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedImports | packages.NeedDeps,
	}
	var result []*packages.Package
	for _, pattern := range pkgPatterns {
		pkgs, err := packages.Load(cfg, pattern)
		if err != nil {
			return nil, fmt.Errorf("loading package %s: %w", pattern, err)
		}
		if len(pkgs) == 0 {
			return nil, fmt.Errorf("package %s not found", pattern)
		}
		for _, pkg := range pkgs {
			if len(pkg.Errors) > 0 {
				var errs []string
				for _, e := range pkg.Errors {
					errs = append(errs, e.Error())
				}
				return nil, fmt.Errorf("errors loading package %s: %s", pattern, strings.Join(errs, "; "))
			}
			result = append(result, pkg)
		}
	}
	return result, nil
}
