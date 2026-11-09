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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/mod/module"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// The files a monitoring integration's contract module holds at its root:
// the OpenAPI v3 schemas of the parameters of the integration's destinations
// (MonitoringDestination.spec.parameters) and of an Instance's entries for
// them (Instance.spec.monitoring.destinations[].parameters), generated from
// the contract's Go types. github.com/openeverest/monitoring-pmm/api is such
// a module.
const (
	destinationParametersFile = "zz_generated.destination-parameters.yaml"
	instanceParametersFile    = "zz_generated.instance-parameters.yaml"
)

// contractSchemas returns the destination and instance parameter schemas of
// the contract module modulePath, in the version the Go module containing dir
// selects (its go.mod, a replace directive or a go.work), so a provider
// publishes the schemas of the contract it is built with. Each is closed
// (additionalProperties: false) at its root, so a parameter newer than that
// version is unsupported (spec 019 Rule E).
func contractSchemas(dir, modulePath string) (*apiextensionsv1.JSONSchemaProps, *apiextensionsv1.JSONSchemaProps, error) {
	if err := module.CheckPath(modulePath); err != nil {
		return nil, nil, fmt.Errorf("%q is not a module path: %w", modulePath, err)
	}
	moduleDir, err := goModuleDir(dir, modulePath)
	if err != nil {
		return nil, nil, err
	}
	destination, errD := readContractSchema(moduleDir, modulePath, destinationParametersFile)
	instance, errI := readContractSchema(moduleDir, modulePath, instanceParametersFile)
	if err := errors.Join(errD, errI); err != nil {
		return nil, nil, err
	}
	return destination, instance, nil
}

// readContractSchema reads one schema file of a contract module.
func readContractSchema(moduleDir, modulePath, file string) (*apiextensionsv1.JSONSchemaProps, error) {
	data, err := os.ReadFile(filepath.Join(moduleDir, file)) //nolint:gosec // a file of a module the provider requires
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s has no %s at its root: it is not a monitoring contract module, or predates this convention", modulePath, file)
	}
	if err != nil {
		return nil, err
	}
	schema := &apiextensionsv1.JSONSchemaProps{}
	if err := yaml.UnmarshalStrict(data, schema); err != nil {
		return nil, fmt.Errorf("%s: %s is not an OpenAPI v3 schema: %w", modulePath, file, err)
	}
	if schema.Type != "object" {
		return nil, fmt.Errorf("%s: %s: parameters are an object, the schema has type %q", modulePath, file, schema.Type)
	}
	if schema.AdditionalProperties == nil {
		schema.AdditionalProperties = &apiextensionsv1.JSONSchemaPropsOrBool{Allows: false}
	}
	return schema, nil
}

// goModule is the part of the output of go list -m -json and go mod
// download -json this file reads.
type goModule struct {
	Path    string
	Version string
	Dir     string
	Replace *goModule
	// Error is an object with an Err string in go list, a string in go mod
	// download.
	Error json.RawMessage
}

// errorText returns the message of Error, or "" without one.
func (m *goModule) errorText() string {
	if len(m.Error) == 0 {
		return ""
	}
	var listErr struct{ Err string }
	if json.Unmarshal(m.Error, &listErr) == nil && listErr.Err != "" {
		return listErr.Err
	}
	var downloadErr string
	if json.Unmarshal(m.Error, &downloadErr) == nil && downloadErr != "" {
		return downloadErr
	}
	return string(m.Error)
}

// goModuleDir returns the directory holding modulePath in the version the Go
// module containing dir selects, downloading it into the module cache if it
// is not there yet.
func goModuleDir(dir, modulePath string) (string, error) {
	listed, err := runGoJSON(dir, "list", "-m", "-e", "-json", modulePath)
	if err != nil {
		return "", err
	}
	if msg := listed.errorText(); msg != "" {
		return "", fmt.Errorf("%s: %s; the provider's go.mod must require the contract module (go get %s@<version>)", modulePath, msg, modulePath)
	}
	if listed.Dir != "" {
		return listed.Dir, nil
	}
	want := listed
	if listed.Replace != nil {
		want = listed.Replace
	}
	downloaded, err := runGoJSON(dir, "mod", "download", "-json", want.Path+"@"+want.Version)
	if err != nil {
		return "", err
	}
	if msg := downloaded.errorText(); msg != "" || downloaded.Dir == "" {
		return "", fmt.Errorf("downloading %s@%s: %s", want.Path, want.Version, msg)
	}
	return downloaded.Dir, nil
}

// runGoJSON runs the go command in dir and decodes its JSON output. A
// command that fails with a JSON error on stdout returns that error in the
// result instead.
func runGoJSON(dir string, args ...string) (*goModule, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	out := &goModule{}
	if err := json.Unmarshal(stdout.Bytes(), out); err != nil {
		if runErr != nil {
			return nil, fmt.Errorf("go %s: %w: %s", strings.Join(args, " "), runErr, strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}
	if runErr != nil && out.errorText() == "" {
		return nil, fmt.Errorf("go %s: %w: %s", strings.Join(args, " "), runErr, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
