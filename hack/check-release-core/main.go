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

// Command check-release-core refuses a provider-sdk release that does not
// require a released OpenEverest core: the core provider-sdk's go.mod
// requires, as `go list -m` selects it, must be a tag, not a pseudo-version,
// and must not be replaced, and so must the core the go.mod of a scaffolded
// project requires. See compat.LibraryReleaseProblems for why.
//
// Between releases, provider-sdk's main branch may require a commit of
// OpenEverest's main branch, so only the release workflow runs this check.
//
// Run it from the module root: go run ./hack/check-release-core
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/openeverest/provider-sdk/internal/compat"
	"github.com/openeverest/provider-sdk/internal/scaffold"
)

const scaffoldGoMod = "internal/scaffold/_template/go.mod.tmpl"

func main() {
	sdk, err := compat.RequiredCore(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	scaffolded, err := scaffold.RequiredCore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := check(sdk, scaffolded); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("go.mod requires %s\n%s requires %s\n", sdk, scaffoldGoMod, scaffolded)
}

// check returns an error listing every reason why sdk, the core provider-sdk
// requires, and scaffolded, the core a scaffolded project requires, keep
// provider-sdk from being released.
func check(sdk, scaffolded compat.Module) error {
	var problems []string
	for _, p := range compat.LibraryReleaseProblems(sdk) {
		problems = append(problems, "go.mod "+p)
	}
	for _, p := range compat.LibraryReleaseProblems(scaffolded) {
		problems = append(problems, scaffoldGoMod+" "+p)
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("provider-sdk releases must require a released OpenEverest; pin core to its tag first:\n  - %s",
		strings.Join(problems, "\n  - "))
}
