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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blang/semver/v4"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/yaml"

	corev1alpha1 "github.com/openeverest/openeverest/v2/api/core/v1alpha1"
)

var monitoringKindPattern = regexp.MustCompile(corev1alpha1.MonitoringKindPattern)

// assembleMonitoring reads the optional definition/monitoring.yaml and returns
// it as Provider.spec.monitoring. It returns nil when the file is absent or
// declares nothing, so a fully commented-out file publishes no contract.
//
// components is the components map from provider.yaml; every key under
// componentTypes must be the type of one of them.
func assembleMonitoring(defDir string, components map[string]any) (map[string]any, error) {
	data, err := os.ReadFile(filepath.Join(defDir, "monitoring.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading monitoring.yaml: %w", err)
	}

	var monitoring corev1alpha1.ProviderMonitoring
	if err := yaml.UnmarshalStrict(data, &monitoring); err != nil {
		var topLevel map[string]any
		if yaml.Unmarshal(data, &topLevel) == nil {
			if _, wrapped := topLevel["monitoring"]; wrapped {
				return nil, errors.New("parsing monitoring.yaml: this file is spec.monitoring itself; remove the top-level monitoring key")
			}
		}
		return nil, fmt.Errorf("parsing monitoring.yaml: %w", err)
	}
	if len(monitoring.Integrations) == 0 && len(monitoring.ComponentTypes) == 0 {
		return nil, nil
	}
	if err := validateMonitoring(&monitoring, declaredComponentTypes(components)); err != nil {
		return nil, fmt.Errorf("invalid monitoring.yaml: %w", err)
	}

	spec, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&monitoring)
	if err != nil {
		return nil, fmt.Errorf("converting monitoring.yaml: %w", err)
	}
	return spec, nil
}

// validateMonitoring reports every problem in the declaration at once, in a
// stable order, naming the offending field.
func validateMonitoring(monitoring *corev1alpha1.ProviderMonitoring, declaredTypes map[string]bool) error {
	var errs []error

	for _, name := range sortedKeys(monitoring.Integrations) {
		for _, msg := range validation.IsDNS1123Label(name) {
			errs = append(errs, fmt.Errorf("integrations[%q]: %s", name, msg))
		}

		agentVersions := monitoring.Integrations[name].AgentVersions
		if agentVersions == "" {
			continue
		}
		if _, err := semver.ParseRange(agentVersions); err != nil {
			errs = append(errs, fmt.Errorf("integrations[%q].agentVersions: %q is not a semver range: %w", name, agentVersions, err))
		}
	}

	declared := "none"
	if len(declaredTypes) > 0 {
		declared = strings.Join(sortedKeys(declaredTypes), ", ")
	}

	for _, typeName := range sortedKeys(monitoring.ComponentTypes) {
		componentType := monitoring.ComponentTypes[typeName]

		if !declaredTypes[typeName] {
			errs = append(errs, fmt.Errorf("componentTypes[%q]: no component in provider.yaml has type %q (declared types: %s)", typeName, typeName, declared))
		}

		switch {
		case componentType.Kind == "":
			errs = append(errs, fmt.Errorf("componentTypes[%q].kind: required", typeName))
		case !monitoringKindPattern.MatchString(componentType.Kind):
			errs = append(errs, fmt.Errorf("componentTypes[%q].kind: %q does not match %s", typeName, componentType.Kind, monitoringKindPattern))
		}

		switch componentType.Metrics {
		case "", corev1alpha1.MetricsSourceNative, corev1alpha1.MetricsSourceExporter, corev1alpha1.MetricsSourceNone:
		default:
			errs = append(errs, fmt.Errorf("componentTypes[%q].metrics: %q is not one of %s, %s, %s", typeName, componentType.Metrics,
				corev1alpha1.MetricsSourceNative, corev1alpha1.MetricsSourceExporter, corev1alpha1.MetricsSourceNone))
		}
	}

	return errors.Join(errs...)
}

// declaredComponentTypes returns the set of component types the components
// map from provider.yaml uses.
func declaredComponentTypes(components map[string]any) map[string]bool {
	declared := make(map[string]bool)
	for _, raw := range components {
		if component, ok := raw.(map[string]any); ok {
			if typeName, ok := component["type"].(string); ok {
				declared[typeName] = true
			}
		}
	}
	return declared
}
