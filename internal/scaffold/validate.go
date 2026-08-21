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

package scaffold

import (
	"fmt"
	"go/token"
	"regexp"
)

// resourceNameRE matches names in the DNS-1123 label format used for
// Kubernetes resource names. A name must be 1-63 characters, start and end
// with a lowercase alphanumeric character, and contain only lowercase
// alphanumeric characters and hyphens.
var resourceNameRE = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

// identifierNameRE matches valid Go identifiers. They cannot start with a
// digit, and contain only letters, digits, and underscores.
var identifierNameRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validateResourceName applies to names that become Kubernetes object names or
// keys addressed as such: secrets, configmaps, backup classes.
func validateResourceName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}

	if !resourceNameRE.MatchString(name) {
		return fmt.Errorf(
			"invalid name %q: must be 1-63 characters, contain only lowercase alphanumeric or '-', and start and end with an alphanumeric",
			name,
		)
	}

	return nil
}

// validateIdentifierName applies to names that only have to yield a Go
// identifier: topologies and components. The name is converted to PascalCase
// before checking against the Go identifier regex, and normalized to a
// lowercase, symbol-free identifier before checking against Go keywords.
func validateIdentifierName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}

	pascal := toPascalCase(name)
	if !identifierNameRE.MatchString(pascal) {
		return fmt.Errorf("invalid name %q: %q is not a valid Go identifier", name, pascal)
	}

	ident := toGoIdent(name)
	if token.IsKeyword(ident) {
		return fmt.Errorf("invalid name %q: %q is a Go keyword", name, ident)
	}

	return nil
}
