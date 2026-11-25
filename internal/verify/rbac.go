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

package verify

import (
	"fmt"
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	corev1alpha1 "github.com/openeverest/openeverest/v2/api/core/v1alpha1"
	"github.com/openeverest/provider-sdk/internal/chartrender"
)

var (
	providerKind    = schema.GroupKind{Group: corev1alpha1.GroupVersion.Group, Kind: "Provider"}
	clusterRoleKind = schema.GroupKind{Group: rbacv1.GroupName, Kind: "ClusterRole"}
)

// request is a request the provider makes to the API server: verb on the
// object name of resource, in group.
type request struct {
	group, resource, name, verb string

	// why is what the provider needs it for, and what happens without it.
	why string
}

// startupGateRequests returns the requests provider-runtime's startup gate
// makes for the Providers named providers. Every provider must grant them
// itself, as controller-gen does not read the RBAC markers of the
// provider-runtime it depends on.
func startupGateRequests(providers []string) []request {
	requests := []request{{
		group:    apiextensionsv1.GroupName,
		resource: "customresourcedefinitions",
		name:     chartrender.CoreInterpreted[providerKind],
		verb:     "get",
		why:      "to read the OpenEverest versions the cluster supports; without it, a new build of the provider never becomes ready",
	}}
	for _, provider := range providers {
		requests = append(requests, request{
			group:    corev1alpha1.GroupVersion.Group,
			resource: "providers/status",
			name:     provider,
			verb:     "patch",
			why:      "to record on its Provider the version that started; without it, every restart waits for the check like a new build",
		})
	}
	return requests
}

// marker returns the kubebuilder RBAC marker that grants r.
func (r request) marker() string {
	return fmt.Sprintf("// +kubebuilder:rbac:groups=%s,resources=%s,resourceNames=%s,verbs=%s", r.group, r.resource, r.name, r.verb)
}

// allowedBy reports whether rule allows r, the way the API server's RBAC
// authorizer matches it.
func (r request) allowedBy(rule rbacv1.PolicyRule) bool {
	return matches(rule.APIGroups, r.group) &&
		resourceMatches(rule.Resources, r.resource) &&
		(len(rule.ResourceNames) == 0 || slices.Contains(rule.ResourceNames, r.name)) &&
		matches(rule.Verbs, r.verb)
}

func matches(values []string, value string) bool {
	return slices.Contains(values, rbacv1.ResourceAll) || slices.Contains(values, value)
}

// resourceMatches also matches a subresource with */<subresource>.
func resourceMatches(resources []string, resource string) bool {
	if matches(resources, resource) {
		return true
	}
	_, subresource, ok := strings.Cut(resource, "/")
	return ok && slices.Contains(resources, "*/"+subresource)
}

// startupGateProblems reports the requests of provider-runtime's startup
// gate for the Providers named providers that no ClusterRole of the chart
// allows. A chart that renders no ClusterRole of its own leaves RBAC to
// whoever installs it: there is nothing to check.
func startupGateProblems(objects []chartrender.Object, providers []string) ([]string, error) {
	var rules []rbacv1.PolicyRule
	roles := 0
	for _, o := range objects {
		if !o.Own() || o.GroupKind != clusterRoleKind {
			continue
		}
		roles++
		var role rbacv1.ClusterRole
		if err := yaml.Unmarshal([]byte(o.Manifest), &role); err != nil {
			return nil, fmt.Errorf("decoding ClusterRole %s (%s): %w", o.Name, o.Source, err)
		}
		rules = append(rules, role.Rules...)
	}
	if roles == 0 {
		return nil, nil
	}

	var problems []string
	for _, r := range startupGateRequests(providers) {
		if slices.ContainsFunc(rules, r.allowedBy) {
			continue
		}
		problems = append(problems, fmt.Sprintf(
			"no ClusterRole of the chart allows %s on %s %s (%s), which provider-runtime needs %s; add `%s` to internal/provider/rbac.go and run `make generate`",
			r.verb, r.resource, r.name, r.group, r.why, r.marker()))
	}
	return problems, nil
}
