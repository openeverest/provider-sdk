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

	"github.com/openeverest/provider-sdk/internal/compat"
)

// RequiredCore returns the OpenEverest core the go.mod of a scaffolded project
// requires, with the module that replaces it, if any.
func RequiredCore() (compat.Module, error) {
	const name = "go.mod.tmpl"
	tmpl, err := templateFS.ReadFile(templateRoot + "/" + name)
	if err != nil {
		return compat.Module{}, fmt.Errorf("reading %s: %w", name, err)
	}
	rendered, err := renderTemplate(name, string(tmpl), &Config{ModulePath: "example.com/provider"})
	if err != nil {
		return compat.Module{}, fmt.Errorf("rendering %s: %w", name, err)
	}
	return compat.RequiredCoreIn(name, []byte(rendered))
}
