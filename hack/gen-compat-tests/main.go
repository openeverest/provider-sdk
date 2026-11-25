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
// fixture's suites of the vectors OpenEverest publishes for its compatibility
// check, its development values and its acknowledgements, so the Helm
// implementation decides every case exactly as core does.
//
// Run it from the module root: go run ./hack/gen-compat-tests
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

	// fixtureProvider is the name of the Provider of the fixture chart, its
	// provider.name value, under which OpenEverest lists its acknowledgement.
	fixtureProvider = "compat-test"
	// newestStamp is a stamp newer than every version in the vectors, so the
	// check refuses it on any window of theirs, and only an acknowledgement
	// lets it through.
	newestStamp = "99.0.0"
	// oldestCore is a version older than every version in the vectors.
	oldestCore = "1.0.0"
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
	acknowledge, err := acknowledgeSuite()
	if err != nil {
		return nil, err
	}
	development, err := developmentSuite()
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		scaffoldChart + "/templates/" + generate.ContractHelperFile: []byte(generate.ContractHelper),
		scaffoldChart + "/templates/" + generate.CompatCheckFile:    []byte(generate.CompatCheck([]string{providersCRD})),
		fixtureChart + "/templates/" + generate.ContractHelperFile:  []byte(generate.ContractHelper),
		fixtureChart + "/templates/" + generate.CompatCheckFile:     []byte(generate.CompatCheck([]string{providersCRD, backupCRD})),
		fixtureChart + "/tests/vectors_test.yaml":                   vectors,
		fixtureChart + "/tests/acknowledge_vectors_test.yaml":       acknowledge,
		fixtureChart + "/tests/development_vectors_test.yaml":       development,
	}, nil
}

type vectors struct {
	Order       []string `json:"order"`
	Invalid     []string `json:"invalid"`
	Development []struct {
		Version     string `json:"version"`
		Development bool   `json:"development"`
	} `json:"development"`
	Check []struct {
		Ext    string `json:"ext"`
		Min    string `json:"min"`
		Cur    string `json:"cur"`
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"check"`
	Acknowledge []struct {
		Ack             string `json:"ack"`
		ContractVersion string `json:"contractVersion"`
		CoreVersion     string `json:"coreVersion"`
		Holds           bool   `json:"holds"`
	} `json:"acknowledge"`
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
	Template           string            `yaml:"template,omitempty"`
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
		Suite:              "contract vectors",
		Templates:          []string{"templates/verdict.yaml"},
		KubernetesProvider: crdScheme(),
		Tests:              tests,
	}
	return encode(s)
}

// developmentSuite returns the helm-unittest suite that runs every
// "development" case of contract.OrderVectors:
//   - on openeverest.contract.development itself, expecting the case's
//     result;
//   - when the case's version is valid, through the check, as the stamp on a
//     core older than it and as the window of a core newer than a chart
//     stamped oldestCore. The message calls a development value an
//     unreleased OpenEverest build, and names any other version.
//
// Every expectation is also what contract.IsDevelopment and contract.Check
// return, or generation fails.
func developmentSuite() ([]byte, error) {
	var v vectors
	if err := json.Unmarshal(contract.OrderVectors, &v); err != nil {
		return nil, fmt.Errorf("decoding contract.OrderVectors: %w", err)
	}

	var tests []test
	for _, c := range v.Development {
		if got := contract.IsDevelopment(c.Version); got != c.Development {
			return nil, fmt.Errorf("contract.IsDevelopment(%q) is %t, not %t", c.Version, got, c.Development)
		}
		development := ""
		if c.Development {
			development = "true"
		}
		tests = append(tests, test{
			It:       fmt.Sprintf("development: %q is a development value: %t", c.Version, c.Development),
			Template: "templates/development.yaml",
			Set:      map[string]string{"version": c.Version},
			Asserts:  []any{equal("data.development", development)},
		})

		if !contract.Valid(c.Version) {
			continue
		}
		if got := contract.Check(c.Version, oldestCore, oldestCore).Reason; got != contract.ReasonCoreTooOld {
			return nil, fmt.Errorf("contract.Check(%q, %q, %q) is %s, not %s", c.Version, oldestCore, oldestCore, got, contract.ReasonCoreTooOld)
		}
		coreTooOld := "needs OpenEverest " + regexp.QuoteMeta(c.Version) + " or later;"
		if c.Development {
			coreTooOld = "needs a newer, unreleased OpenEverest build;"
		}
		tests = append(tests, test{
			It:                 fmt.Sprintf("development stamp: a chart stamped %q on OpenEverest %s", c.Version, oldestCore),
			Template:           "templates/check.yaml",
			Set:                map[string]string{"stamp": c.Version},
			KubernetesProvider: &provider{Objects: []any{crd(oldestCore, oldestCore, nil)}},
			Asserts:            []any{failedTemplate(coreTooOld)},
		})

		if got := contract.Check(oldestCore, c.Version, c.Version).Reason; got != contract.ReasonExtensionTooOld {
			return nil, fmt.Errorf("contract.Check(%q, %q, %q) is %s, not %s", oldestCore, c.Version, c.Version, got, contract.ReasonExtensionTooOld)
		}
		extensionTooOld := "built for OpenEverest " + regexp.QuoteMeta(c.Version) + " or later;"
		if c.Development {
			extensionTooOld = "built for this unreleased OpenEverest build;"
		}
		tests = append(tests, test{
			It:                 fmt.Sprintf("development minimum: a chart stamped %q on OpenEverest %s serving %q and later", oldestCore, newestStamp, c.Version),
			Template:           "templates/check.yaml",
			Set:                map[string]string{"stamp": oldestCore},
			KubernetesProvider: &provider{Objects: []any{crd(c.Version, newestStamp, nil)}},
			Asserts:            []any{failedTemplate(extensionTooOld)},
		})
	}

	return encode(suite{
		Suite:              "development vectors",
		Templates:          []string{"templates/development.yaml", "templates/check.yaml"},
		KubernetesProvider: crdScheme(),
		Tests:              tests,
	})
}

// acknowledgeSuite returns the helm-unittest suite that runs every
// "acknowledge" case of contract.OrderVectors:
//   - on openeverest.contract.acknowledgementHolds itself, expecting the
//     case's result;
//   - when the case's contract version is valid, through the check, on a
//     chart stamped newestStamp in a cluster whose providers CRD publishes
//     the case's contract version and core version, with the case's
//     acknowledgement as compatibility.acknowledge.coreVersion, and as the
//     chart's entry in the CRD's acknowledged-extensions list when it reads
//     back as written. The check passes exactly when the acknowledgement
//     holds.
//
// Every expectation is also what contract.AcknowledgementHolds and
// contract.ParseAcknowledgedExtensions return, or generation fails.
func acknowledgeSuite() ([]byte, error) {
	var v vectors
	if err := json.Unmarshal(contract.OrderVectors, &v); err != nil {
		return nil, fmt.Errorf("decoding contract.OrderVectors: %w", err)
	}

	var tests []test
	for _, c := range v.Acknowledge {
		if got := contract.AcknowledgementHolds(c.Ack, c.ContractVersion, c.CoreVersion); got != c.Holds {
			return nil, fmt.Errorf("contract.AcknowledgementHolds(%q, %q, %q) is %t, not %t", c.Ack, c.ContractVersion, c.CoreVersion, got, c.Holds)
		}
		holds := ""
		if c.Holds {
			holds = "true"
		}
		tests = append(tests, test{
			It:       fmt.Sprintf("acknowledge: %q with contract version %q on %q holds: %t", c.Ack, c.ContractVersion, c.CoreVersion, c.Holds),
			Template: "templates/acknowledgement.yaml",
			Set:      map[string]string{"ack": c.Ack, "contractVersion": c.ContractVersion, "coreVersion": c.CoreVersion},
			Asserts:  []any{equal("data.holds", holds)},
		})

		if !contract.Valid(c.ContractVersion) {
			continue
		}
		if got := contract.Check(newestStamp, c.ContractVersion, c.ContractVersion).Reason; got != contract.ReasonCoreTooOld {
			return nil, fmt.Errorf("contract.Check(%q, %q, %q) is %s, not %s", newestStamp, c.ContractVersion, c.ContractVersion, got, contract.ReasonCoreTooOld)
		}
		tests = append(tests, test{
			It: fmt.Sprintf("acknowledge in values: %q with contract version %q on %q %s",
				c.Ack, c.ContractVersion, c.CoreVersion, passesOrFails(c.Holds)),
			Template:           "templates/check.yaml",
			Set:                map[string]string{"stamp": newestStamp, contract.AcknowledgeValuePath: c.Ack},
			KubernetesProvider: &provider{Objects: []any{crd(c.ContractVersion, c.CoreVersion, nil)}},
			Asserts:            []any{checkResult(c.Holds)},
		})

		// The list trims its entries and splits them on commas, so only
		// values that read back as written are the same case.
		if strings.TrimSpace(c.Ack) != c.Ack || strings.Contains(c.Ack, ",") {
			continue
		}
		list := fixtureProvider + "@" + c.Ack
		acks, err := contract.ParseAcknowledgedExtensions(list)
		listed := err == nil && contract.AcknowledgementHolds(acks[fixtureProvider], c.ContractVersion, c.CoreVersion)
		if listed != c.Holds {
			return nil, fmt.Errorf("acknowledged-extensions %q with contract version %q on %q holds: %t, not %t", list, c.ContractVersion, c.CoreVersion, listed, c.Holds)
		}
		tests = append(tests, test{
			It: fmt.Sprintf("acknowledged-extensions: %q with contract version %q on %q %s",
				list, c.ContractVersion, c.CoreVersion, passesOrFails(c.Holds)),
			Template: "templates/check.yaml",
			Set:      map[string]string{"stamp": newestStamp},
			KubernetesProvider: &provider{Objects: []any{
				crd(c.ContractVersion, c.CoreVersion, map[string]string{contract.AcknowledgedExtensionsAnnotation: list}),
			}},
			Asserts: []any{checkResult(c.Holds)},
		})
	}

	return encode(suite{
		Suite:              "acknowledge vectors",
		Templates:          []string{"templates/acknowledgement.yaml", "templates/check.yaml"},
		KubernetesProvider: crdScheme(),
		Tests:              tests,
	})
}

// crdScheme returns the kubernetesProvider of a suite whose tests mock CRDs.
func crdScheme() *provider {
	return &provider{
		Scheme: map[string]any{
			"apiextensions.k8s.io/v1/CustomResourceDefinition": map[string]any{
				"gvr":        map[string]string{"group": "apiextensions.k8s.io", "version": "v1", "resource": "customresourcedefinitions"},
				"namespaced": false,
			},
		},
		Objects: []any{},
	}
}

// crd returns a providers CRD that publishes the window [contractVersion,
// contractVersion] of a core whose version is coreVersion, with the extra
// annotations.
func crd(contractVersion, coreVersion string, annotations map[string]string) map[string]any {
	all := map[string]string{
		contract.MinVersionAnnotation: contractVersion,
		contract.VersionAnnotation:    contractVersion,
	}
	for k, v := range annotations {
		all[k] = v
	}
	return map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata": map[string]any{
			"name":        providersCRD,
			"labels":      map[string]string{contract.CoreVersionKey: coreVersion},
			"annotations": all,
		},
	}
}

func passesOrFails(passes bool) string {
	if passes {
		return "passes"
	}
	return "fails"
}

// checkResult asserts templates/check.yaml passes or fails.
func checkResult(passes bool) map[string]any {
	if passes {
		return map[string]any{"containsDocument": map[string]string{"kind": "ConfigMap", "apiVersion": "v1", "name": "passed"}}
	}
	return failedTemplate("needs OpenEverest " + strings.ReplaceAll(newestStamp, ".", "[.]") + " or later")
}

// failedTemplate asserts the template fails with a message matching pattern.
func failedTemplate(pattern string) map[string]any {
	return map[string]any{"failedTemplate": map[string]string{"errorPattern": pattern}}
}

func encode(s suite) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("# Code generated by hack/gen-compat-tests from contract.OrderVectors. DO NOT EDIT.\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(s); err != nil {
		return nil, fmt.Errorf("encoding the %s suite: %w", s.Suite, err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encoding the %s suite: %w", s.Suite, err)
	}
	return []byte(strings.TrimRight(buf.String(), "\n") + "\n"), nil
}

func equal(path, value string) map[string]any {
	return map[string]any{"equal": map[string]string{"path": path, "value": value}}
}
