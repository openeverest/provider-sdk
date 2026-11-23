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

// Package compat ties what provider-sdk stamps to the OpenEverest core it
// links.
//
// provider-sdk stamps a provider's chart with the contract version of the
// OpenEverest core it is built against: contract.Version of the core module
// in its own build. Run as `go tool provider-sdk` from the provider's module,
// that is the core the provider requires. A binary built elsewhere, such as a
// globally installed one, may link another core and would stamp the wrong
// value, so the commands that stamp or check stamps first compare the two.
package compat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime/debug"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"

	"github.com/openeverest/openeverest/v2/contract"
)

// CoreModule is the module path of OpenEverest core.
const CoreModule = "github.com/openeverest/openeverest/v2"

// Module is a module version, with the module that replaces it, if any.
type Module struct {
	Path    string
	Version string
	Replace *Module
}

func (m Module) String() string {
	s := m.Path
	if m.Version != "" {
		s += "@" + m.Version
	}
	if m.Replace != nil {
		s += " => " + m.Replace.String()
	}
	return s
}

// local reports whether the code of m comes from a directory, as with a
// replace directive or a go.work workspace, so its version says nothing
// about the code.
func (m Module) local() bool {
	if m.Replace != nil {
		return modfile.IsDirectoryPath(m.Replace.Path)
	}
	return m.Version == "" || m.Version == "(devel)"
}

// effective returns the module whose code is built for m.
func (m Module) effective() Module {
	if m.Replace != nil {
		return *m.Replace
	}
	return m
}

// LinkedCore returns the core module this binary was built with.
func LinkedCore() (Module, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Module{}, errors.New("this provider-sdk binary carries no build information")
	}
	for _, dep := range info.Deps {
		if dep.Path != CoreModule {
			continue
		}
		m := Module{Path: dep.Path, Version: dep.Version}
		if dep.Replace != nil {
			m.Replace = &Module{Path: dep.Replace.Path, Version: dep.Replace.Version}
		}
		return m, nil
	}
	return Module{}, fmt.Errorf("this provider-sdk binary does not link %s", CoreModule)
}

// RequiredCore returns the core module the Go module in dir builds with, as
// `go list -m` selects it.
func RequiredCore(dir string) (Module, error) {
	cmd := exec.Command("go", "list", "-m", "-json", CoreModule)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return Module{}, fmt.Errorf("go list -m %s: %w: %s", CoreModule, err, strings.TrimSpace(stderr.String()))
	}
	var m Module
	if err := json.Unmarshal(out, &m); err != nil {
		return Module{}, fmt.Errorf("decoding go list -m %s: %w", CoreModule, err)
	}
	return m, nil
}

// CheckCore compares the core this binary links with the one the provider's
// module requires. It returns an error when they differ, as the stamp would
// then name a core the provider is not built against. When both come from a
// local directory, their versions cannot be compared, so it returns a
// warning instead: under `go tool` both are the same directory.
func CheckCore(linked, required Module) (warning string, err error) {
	switch {
	case linked.local() && required.local():
		return fmt.Sprintf("%s comes from a local directory (%s), so the stamp is the contract version of that tree, %s",
			CoreModule, required.effective().Path, contract.Version), nil
	case !linked.local() && !required.local():
		l, r := linked.effective(), required.effective()
		if l.Path == r.Path && l.Version == r.Version {
			return "", nil
		}
	}
	return "", fmt.Errorf("this provider-sdk links %s, but this module requires %s, so it would stamp the chart for another OpenEverest; "+
		"run it as `go tool provider-sdk` from the module, which builds it against the core the module requires", linked, required)
}

// ReleaseProblems returns why a provider built against required, stamped
// with version, must not be released. A release is installed by users, so
// the OpenEverest it names must be one they can install: a released core.
func ReleaseProblems(required Module, version string) []string {
	var problems []string
	if contract.IsDevelopment(version) {
		problems = append(problems, fmt.Sprintf("built against an unreleased OpenEverest (%s); require a released OpenEverest in go.mod", version))
	}
	switch {
	case required.Replace != nil:
		problems = append(problems, fmt.Sprintf("go.mod replaces %s (%s); require a released OpenEverest instead", CoreModule, required))
	case required.local():
		problems = append(problems, fmt.Sprintf("%s comes from a local directory; require a released OpenEverest in go.mod", CoreModule))
	case module.IsPseudoVersion(required.Version):
		problems = append(problems, fmt.Sprintf("go.mod requires %s at the pseudo-version %s, an unreleased commit; require a released OpenEverest", CoreModule, required.Version))
	}
	return problems
}

// Floor describes the oldest OpenEverest a provider stamped with version
// works with.
func Floor(version string) string {
	if contract.IsDevelopment(version) {
		return fmt.Sprintf("Built against an unreleased OpenEverest (%s); it cannot be published until that OpenEverest is released.", version)
	}
	return fmt.Sprintf("This build works with OpenEverest %s and later.", version)
}
