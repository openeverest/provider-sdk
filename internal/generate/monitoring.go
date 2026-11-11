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

var authModePattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]{0,62}$`)

// assembleMonitoring reads the optional definition/monitoring.yaml and returns
// it as Provider.spec.monitoring. It returns nil when the file is absent or
// declares nothing, so a fully commented-out file publishes no contract.
//
// components is the components map from provider.yaml; every key under
// componentTypes must be the type of one of them. kinds is the kind each
// component type declares in provider.yaml; every type under componentTypes
// needs one, since provider-runtime publishes no endpoint of a type without
// a kind.
func assembleMonitoring(defDir string, components map[string]any, kinds map[string]string) (map[string]any, error) {
	data, err := os.ReadFile(filepath.Join(defDir, "monitoring.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading monitoring.yaml: %w", err)
	}
	if err := checkMonitoringDeclaresNoKind(data); err != nil {
		return nil, fmt.Errorf("invalid monitoring.yaml: %w", err)
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
	if err := validateMonitoring(&monitoring, declaredComponentTypes(components), kinds); err != nil {
		return nil, fmt.Errorf("invalid monitoring.yaml: %w", err)
	}

	spec, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&monitoring)
	if err != nil {
		return nil, fmt.Errorf("converting monitoring.yaml: %w", err)
	}
	return spec, nil
}

// checkMonitoringDeclaresNoKind rejects the kind of a component type in
// monitoring.yaml, where it was declared before it moved to provider.yaml,
// with a message that says where it goes now.
func checkMonitoringDeclaresNoKind(data []byte) error {
	var loose struct {
		ComponentTypes map[string]map[string]any `json:"componentTypes"`
	}
	if yaml.Unmarshal(data, &loose) != nil {
		return nil // the strict parse reports it
	}
	var errs []error
	for _, typeName := range sortedKeys(loose.ComponentTypes) {
		if kind, has := loose.ComponentTypes[typeName]["kind"]; has {
			errs = append(errs, fmt.Errorf("componentTypes[%q].kind: kind moved to provider.yaml; declare it there as componentTypes.%s.kind: %v and remove it from monitoring.yaml", typeName, typeName, kind))
		}
	}
	return errors.Join(errs...)
}

// validateMonitoring reports every problem in the declaration at once, in a
// stable order, naming the offending field.
func validateMonitoring(monitoring *corev1alpha1.ProviderMonitoring, declaredTypes map[string]bool, kinds map[string]string) error {
	var errs []error

	for _, name := range sortedKeys(monitoring.Integrations) {
		for _, msg := range validation.IsDNS1123Label(name) {
			errs = append(errs, fmt.Errorf("integrations[%q]: %s", name, msg))
		}

		integration := monitoring.Integrations[name]
		if len(integration.AuthModes) == 0 {
			errs = append(errs, fmt.Errorf("integrations[%q].authModes: required, list the authentication modes of the class the provider renders", name))
		}
		seen := make(map[string]bool, len(integration.AuthModes))
		for _, mode := range integration.AuthModes {
			switch {
			case !authModePattern.MatchString(mode):
				errs = append(errs, fmt.Errorf("integrations[%q].authModes: %q does not match %s", name, mode, authModePattern))
			case seen[mode]:
				errs = append(errs, fmt.Errorf("integrations[%q].authModes: %q is listed twice", name, mode))
			}
			seen[mode] = true
		}

		agentVersions := integration.AgentVersions
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

		if kinds[typeName] == "" {
			errs = append(errs, fmt.Errorf("componentTypes[%q]: provider.yaml declares no kind for component type %q; set componentTypes.%s.kind there, endpoints of a type without a kind are not published", typeName, typeName, typeName))
		}

		switch componentType.Metrics {
		case "", corev1alpha1.MetricsSourceNative, corev1alpha1.MetricsSourceExporter, corev1alpha1.MetricsSourceNone:
		default:
			errs = append(errs, fmt.Errorf("componentTypes[%q].metrics: %q is not one of %s, %s, %s", typeName, componentType.Metrics,
				corev1alpha1.MetricsSourceNative, corev1alpha1.MetricsSourceExporter, corev1alpha1.MetricsSourceNone))
		}

		switch componentType.Connection {
		case "", corev1alpha1.ConnectionSupported, corev1alpha1.ConnectionNone:
		default:
			errs = append(errs, fmt.Errorf("componentTypes[%q].connection: %q is not one of %s, %s", typeName, componentType.Connection,
				corev1alpha1.ConnectionSupported, corev1alpha1.ConnectionNone))
		}

		switch componentType.PodMetadata {
		case "", corev1alpha1.PodMetadataSupported, corev1alpha1.PodMetadataNone:
		default:
			errs = append(errs, fmt.Errorf("componentTypes[%q].podMetadata: %q is not one of %s, %s", typeName, componentType.PodMetadata,
				corev1alpha1.PodMetadataSupported, corev1alpha1.PodMetadataNone))
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
