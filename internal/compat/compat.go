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

	// Sum is the checksum of the module's code, as go.sum records it, when
	// known.
	Sum string
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

// same reports whether m and o are the same code: the same module version,
// replaced by the same module version, if at all.
func (m Module) same(o Module) bool {
	if m.Path != o.Path || m.Version != o.Version || (m.Replace == nil) != (o.Replace == nil) {
		return false
	}
	return m.Replace == nil || (m.Replace.Path == o.Replace.Path && m.Replace.Version == o.Replace.Version)
}

// LinkedCore returns the core module this binary was built with.
func LinkedCore() (Module, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Module{}, errors.New("this provider-sdk binary carries no build information")
	}
	m, ok := coreIn(info)
	if !ok {
		return Module{}, fmt.Errorf("this provider-sdk binary does not link %s", CoreModule)
	}
	return m, nil
}

// coreIn returns the core module of the binary whose build information is
// info, and whether it links one.
func coreIn(info *debug.BuildInfo) (Module, bool) {
	for _, dep := range info.Deps {
		if dep.Path != CoreModule {
			continue
		}
		m := Module{Path: dep.Path, Version: dep.Version, Sum: dep.Sum}
		if dep.Replace != nil {
			m.Replace = &Module{Path: dep.Replace.Path, Version: dep.Replace.Version, Sum: dep.Replace.Sum}
		}
		return m, true
	}
	return Module{}, false
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

// RequiredCoreIn returns the core module the go.mod file data, named name,
// requires, with the module its replace directives substitute, if any. It
// reads the file alone, without the minimal version selection of
// RequiredCore.
func RequiredCoreIn(name string, data []byte) (Module, error) {
	f, err := modfile.Parse(name, data, nil)
	if err != nil {
		return Module{}, fmt.Errorf("parsing %s: %w", name, err)
	}
	var core *Module
	for _, r := range f.Require {
		if r.Mod.Path == CoreModule {
			core = &Module{Path: r.Mod.Path, Version: r.Mod.Version}
		}
	}
	if core == nil {
		return Module{}, fmt.Errorf("%s does not require %s", name, CoreModule)
	}
	for _, r := range f.Replace {
		if r.Old.Path == CoreModule && (r.Old.Version == "" || r.Old.Version == core.Version) {
			core.Replace = &Module{Path: r.New.Path, Version: r.New.Version}
		}
	}
	return *core, nil
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

// CheckBinary checks that a binary of the provider, whose build information
// is info, links the core the provider's module requires, required, at the
// same version and with the same checksum, and returns the core it links.
//
// The chart is stamped with the contract version of required, which is in
// the core's tree, and go.sum and the checksum database tie each module
// version, pseudo-versions included, to one tree. A binary that links
// required therefore carries the contract the chart is stamped with, which
// takes neither the core's tree nor anything from the binary but its build
// information. A binary that links another version may carry the same
// contract, but it is not built from the provider's module as released,
// which a release must not ship either. A core from a local directory has no
// version that says which tree it is.
func CheckBinary(info *debug.BuildInfo, required Module) (Module, error) {
	linked, ok := coreIn(info)
	switch {
	case !ok:
		return linked, fmt.Errorf("does not link %s", CoreModule)
	case linked.local():
		return linked, fmt.Errorf("links %s from a local directory, %s, whose contract version nothing records",
			CoreModule, linked.effective().Path)
	case !linked.same(required):
		return linked, fmt.Errorf("links %s, but go.mod requires %s, whose contract version the chart is stamped with; build the image from the commit the chart is released from",
			linked, required)
	}
	if l, r := linked.effective().Sum, required.effective().Sum; l != "" && r != "" && l != r {
		return linked, fmt.Errorf("links %s with the checksum %s, but go.sum records %s", linked, l, r)
	}
	return linked, nil
}

// ReleaseProblems returns why a provider built against required, stamped
// with version, must not be released. A release is installed by users, so
// the OpenEverest it names must be one they can install: its stamp must be a
// released contract version. The core commit itself may be unreleased: a
// provider can take a fix from OpenEverest's main branch while main's
// contract is still that of a release, because the stamp then names that
// release. A provider is a leaf module, so pinning a commit does not leak
// into anyone else's build; libraries such as this SDK require released
// tags instead.
func ReleaseProblems(required Module, version string) []string {
	var problems []string
	if contract.IsDevelopment(version) {
		problems = append(problems, fmt.Sprintf("built against an unreleased OpenEverest (%s): its contract has changed since the last release; require an OpenEverest whose contract is a released one", version))
	}
	switch {
	case required.Replace != nil:
		problems = append(problems, fmt.Sprintf("go.mod replaces %s (%s); require it directly instead", CoreModule, required))
	case required.local():
		problems = append(problems, fmt.Sprintf("%s comes from a local directory; require it in go.mod instead", CoreModule))
	}
	return problems
}

// LibraryReleaseProblems returns why a release of provider-sdk requiring
// required must not be published. provider-sdk is a library: minimal version
// selection gives every provider that requires a release of it at least the
// core that release requires, and a pseudo-version of OpenEverest's main
// branch, such as v2.0.0-dev.2.0.20261125185706-a1ddf4732eeb, sorts above
// every alpha and beta of that version, so a provider asking for one would
// silently build against that commit instead. Consumers also ignore its
// replace directives, so a release built with one is not what they get. A
// release must therefore require a tag of OpenEverest.
func LibraryReleaseProblems(required Module) []string {
	var problems []string
	switch {
	case required.Replace != nil:
		problems = append(problems, fmt.Sprintf("replaces %s with %s", CoreModule, required.Replace))
	case required.local():
		problems = append(problems, fmt.Sprintf("builds %s from a local directory, such as a go.work workspace", CoreModule))
	}
	if module.IsPseudoVersion(required.Version) {
		problems = append(problems, fmt.Sprintf("requires %s@%s, a pseudo-version, not a tag", CoreModule, required.Version))
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
