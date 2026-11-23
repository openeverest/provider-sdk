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

// Command gen-compat-tests writes the files generated from this module's
// code that are committed outside a provider's chart: the copies of the
// templates provider-sdk generate writes into a chart, in the scaffold and in
// the helm-unittest fixture chart test/helm/openeverest-compat, and the
// fixture's suite of the vectors OpenEverest publishes for its compatibility
// check, so the Helm implementation decides every case exactly as core does.
//
// Run it from the module root: go run ./hack/gen-compat-tests
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/openeverest/openeverest/v2/contract"
	"github.com/openeverest/provider-sdk/internal/generate"
)

const (
	fixtureChart  = "test/helm/openeverest-compat"
	scaffoldChart = "internal/scaffold/_template/charts/__PROJECT_NAME__"
	providersCRD  = "providers.core.openeverest.io"
	backupCRD     = "backupclasses.backup.openeverest.io"
)

func main() {
	files, err := Files()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.FromSlash(path), content, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

// Files returns the files to write, by path from the module root.
func Files() (map[string][]byte, error) {
	vectors, err := vectorsSuite()
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		scaffoldChart + "/templates/" + generate.ContractHelperFile: []byte(generate.ContractHelper),
		scaffoldChart + "/templates/" + generate.CompatCheckFile:    []byte(generate.CompatCheck([]string{providersCRD})),
		fixtureChart + "/templates/" + generate.ContractHelperFile:  []byte(generate.ContractHelper),
		fixtureChart + "/templates/" + generate.CompatCheckFile:     []byte(generate.CompatCheck([]string{providersCRD, backupCRD})),
		fixtureChart + "/tests/vectors_test.yaml":                   vectors,
	}, nil
}

type vectors struct {
	Order   []string `json:"order"`
	Invalid []string `json:"invalid"`
	Check   []struct {
		Ext    string `json:"ext"`
		Min    string `json:"min"`
		Cur    string `json:"cur"`
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"check"`
}

type suite struct {
	Suite              string    `yaml:"suite"`
	Templates          []string  `yaml:"templates"`
	KubernetesProvider *provider `yaml:"kubernetesProvider"`
	Tests              []test    `yaml:"tests"`
}

type provider struct {
	Scheme  map[string]any `yaml:"scheme,omitempty"`
	Objects []any          `yaml:"objects"`
}

type test struct {
	It                 string            `yaml:"it"`
	Set                map[string]string `yaml:"set"`
	KubernetesProvider *provider         `yaml:"kubernetesProvider"`
	Asserts            []any             `yaml:"asserts"`
}

// vectorsSuite returns the helm-unittest suite that runs the verdict of
// _openeverest_contract.tpl on every case of contract.OrderVectors, with the
// window published on a mocked providers CRD:
//   - each "check" row, expecting its reason;
//   - each pair of consecutive "order" versions, both ways, and each version
//     against itself, so the order is the same as core's;
//   - each "invalid" version as the stamp and as the window's minimum.
//
// Every expectation is also what contract.Check returns, or generation fails.
func vectorsSuite() ([]byte, error) {
	var v vectors
	if err := json.Unmarshal(contract.OrderVectors, &v); err != nil {
		return nil, fmt.Errorf("decoding contract.OrderVectors: %w", err)
	}

	var tests []test
	add := func(name, ext, minimum, current, reason string) error {
		if got := contract.Check(ext, minimum, current).Reason; got != reason {
			return fmt.Errorf("%s: contract.Check(%q, %q, %q) is %s, not %s", name, ext, minimum, current, got, reason)
		}
		development := ""
		if contract.IsDevelopment(ext) {
			development = "true"
		}
		tests = append(tests, test{
			It:  fmt.Sprintf("%s: %q in [%q, %q] is %s", name, ext, minimum, current, reason),
			Set: map[string]string{"ext": ext},
			KubernetesProvider: &provider{Objects: []any{map[string]any{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "CustomResourceDefinition",
				"metadata": map[string]any{
					"name": providersCRD,
					"annotations": map[string]string{
						contract.MinVersionAnnotation: minimum,
						contract.VersionAnnotation:    current,
					},
				},
			}}},
			Asserts: []any{
				equal("data.found", "1"),
				equal("data.reason", reason),
				equal("data.development", development),
			},
		})
		return nil
	}

	for _, c := range v.Check {
		if err := add("check", c.Ext, c.Min, c.Cur, c.Reason); err != nil {
			return nil, err
		}
	}
	for i, ver := range v.Order {
		if err := add("order", ver, ver, ver, contract.ReasonSupported); err != nil {
			return nil, err
		}
		if i == 0 {
			continue
		}
		prev := v.Order[i-1]
		if err := add("order", ver, prev, prev, contract.ReasonCoreTooOld); err != nil {
			return nil, err
		}
		if err := add("order", prev, ver, ver, contract.ReasonExtensionTooOld); err != nil {
			return nil, err
		}
	}
	const valid = "2.0.0"
	for _, inv := range v.Invalid {
		reason := contract.ReasonInvalidVersion
		if inv == "" {
			reason = contract.ReasonUnversioned
		}
		if err := add("invalid stamp", inv, valid, valid, reason); err != nil {
			return nil, err
		}
		if err := add("invalid window", valid, inv, valid, contract.ReasonInvalidVersion); err != nil {
			return nil, err
		}
	}

	s := suite{
		Suite:     "contract vectors",
		Templates: []string{"templates/verdict.yaml"},
		KubernetesProvider: &provider{
			Scheme: map[string]any{
				"apiextensions.k8s.io/v1/CustomResourceDefinition": map[string]any{
					"gvr":        map[string]string{"group": "apiextensions.k8s.io", "version": "v1", "resource": "customresourcedefinitions"},
					"namespaced": false,
				},
			},
			Objects: []any{},
		},
		Tests: tests,
	}
	var buf bytes.Buffer
	buf.WriteString("# Code generated by hack/gen-compat-tests from contract.OrderVectors. DO NOT EDIT.\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(s); err != nil {
		return nil, fmt.Errorf("encoding the vectors suite: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encoding the vectors suite: %w", err)
	}
	return []byte(strings.TrimRight(buf.String(), "\n") + "\n"), nil
}

func equal(path, value string) map[string]any {
	return map[string]any{"equal": map[string]string{"path": path, "value": value}}
}
