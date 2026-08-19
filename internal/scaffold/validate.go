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

// nameRE matches valid names accepted by the scaffolder. Names follow the
// DNS-1123 label format used for Kubernetes resource names, with one added
// restriction: they cannot start with a digit, because the name is also used
// to derive Go identifiers (package and type names). A name must be lowercase,
// start with a letter, end with an alphanumeric character, and contain only
// letters, digits, and hyphens. Hyphens are stripped when deriving Go package
// names.
var nameRE = regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`)

// validateName rejects names that are unusable as a Kubernetes resource name
// or Go identifier.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}

	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid name %q: must be alphanumeric or '-', start with a letter, and end with an alphanumeric character", name)
	}

	if token.IsKeyword(name) {
		return fmt.Errorf("invalid name %q: must not be a Go keyword", name)
	}

	return nil
}
