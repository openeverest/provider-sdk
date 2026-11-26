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

// Package chartrender renders a provider's Helm chart the way an install
// with the chart's default values would, without a cluster, and returns the
// objects it applies.
package chartrender

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"
	"unicode"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	backupv1alpha1 "github.com/openeverest/openeverest/v2/api/backup/v1alpha1"
	corev1alpha1 "github.com/openeverest/openeverest/v2/api/core/v1alpha1"
	monitoringv1alpha1 "github.com/openeverest/openeverest/v2/api/monitoring/v1alpha1"
)

// CoreInterpreted lists the kinds of objects an extension applies that
// OpenEverest core interprets, with the CRDs that serve them. Each must
// carry the stamp of the core the extension was built against. A provider's
// ClusterInstancePresets are written for its parameter schemas, so they are
// stamped too: those of a newer provider must not land next to an older
// Provider.
var CoreInterpreted = map[schema.GroupKind]string{
	{Group: corev1alpha1.GroupVersion.Group, Kind: "Provider"}:              "providers." + corev1alpha1.GroupVersion.Group,
	{Group: corev1alpha1.GroupVersion.Group, Kind: "ClusterInstancePreset"}: "clusterinstancepresets." + corev1alpha1.GroupVersion.Group,
	{Group: backupv1alpha1.GroupVersion.Group, Kind: "BackupClass"}:         "backupclasses." + backupv1alpha1.GroupVersion.Group,
	{Group: backupv1alpha1.GroupVersion.Group, Kind: "BackupStorageClass"}:  "backupstorageclasses." + backupv1alpha1.GroupVersion.Group,
	{Group: monitoringv1alpha1.GroupVersion.Group, Kind: "MonitoringClass"}: "monitoringclasses." + monitoringv1alpha1.GroupVersion.Group,
}

// Object is an object a chart renders.
type Object struct {
	// Source is the template that rendered the object, relative to the
	// chart, e.g. templates/provider.yaml, or
	// charts/operator/templates/deployment.yaml for a subchart's.
	Source      string
	GroupKind   schema.GroupKind
	Name        string
	Annotations map[string]string

	// Manifest is the object as the chart renders it, in YAML.
	Manifest string
}

// CoreInterpreted reports whether core interprets o.
func (o Object) CoreInterpreted() bool {
	_, ok := CoreInterpreted[o.GroupKind]
	return ok
}

// Own reports whether one of the chart's own templates rendered o, rather
// than one of its subcharts', such as a bundled operator's.
func (o Object) Own() bool {
	return strings.HasPrefix(o.Source, "templates/")
}

// APIVersionsAnnotation is the Chart.yaml annotation that lists the API
// versions the chart needs the cluster to serve before it is installed,
// beyond Kubernetes' built-in APIs and the CRDs it ships in crds/: those of
// prerequisites, such as cert-manager's for a bundled operator chart that
// checks for it. Entries are separated by commas or whitespace, each in a
// form .Capabilities.APIVersions.Has checks, group/version or
// group/version/Kind, as `helm template --api-versions` takes them.
const APIVersionsAnnotation = "openeverest.io/api-versions"

// Options configures Render.
type Options struct {
	// Values are set over the chart's default values, as
	// `helm install --values` sets them.
	Values map[string]any

	// Skip lists templates, relative to the chart (e.g.
	// templates/check.yaml), that are not rendered.
	Skip []string

	// APIVersions are API versions the cluster serves on top of those
	// Render already renders with, as `helm template --api-versions` adds
	// them.
	APIVersions []string
}

// Render renders the chart in dir with its default values, as a fresh
// install would, and returns the objects it applies, its subcharts'
// included, in template order. Nothing is read from a cluster: lookup
// returns nothing, and the cluster serves the APIs of Kubernetes, those of
// the CRDs in the crds/ directories of the chart and of the subcharts it
// enables, which Helm installs before it renders, and those the chart lists
// in its APIVersionsAnnotation.
func Render(dir string, opts Options) ([]Object, error) {
	values := opts.Values
	if values == nil {
		values = map[string]any{}
	}
	chrt, err := loader.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("loading chart %s: %w", dir, err)
	}
	if len(opts.Skip) > 0 {
		kept := chrt.Templates[:0]
		for _, t := range chrt.Templates {
			if !slices.Contains(opts.Skip, t.Name) {
				kept = append(kept, t)
			}
		}
		chrt.Templates = kept
	}
	if err := chartutil.ProcessDependenciesWithMerge(chrt, values); err != nil {
		return nil, fmt.Errorf("processing chart dependencies: %w", err)
	}
	crdAPIs, err := crdAPIVersions(chrt)
	if err != nil {
		return nil, err
	}
	caps := chartutil.DefaultCapabilities.Copy()
	caps.APIVersions = slices.Concat(caps.APIVersions, crdAPIs,
		splitAPIVersions(chrt.Metadata.Annotations[APIVersionsAnnotation]), opts.APIVersions)
	vals, err := chartutil.ToRenderValues(chrt, values, chartutil.ReleaseOptions{
		Name:      chrt.Name(),
		Namespace: "default",
		Revision:  1,
		IsInstall: true,
	}, caps)
	if err != nil {
		return nil, fmt.Errorf("computing chart values: %w", err)
	}
	files, err := engine.Render(chrt, vals)
	if err != nil {
		return nil, fmt.Errorf("rendering chart %s, for a cluster serving the APIs of Kubernetes, of its CRDs in crds/ and those listed in Chart.yaml's %s annotation: %w",
			dir, APIVersionsAnnotation, err)
	}

	names := make([]string, 0, len(files))
	for name := range files {
		if path.Base(name) == "NOTES.txt" || strings.HasPrefix(path.Base(name), "_") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	var objects []Object
	for _, name := range names {
		docs := releaseutil.SplitManifests(files[name])
		keys := make([]string, 0, len(docs))
		for k := range docs {
			keys = append(keys, k)
		}
		sort.Sort(releaseutil.BySplitManifestsOrder(keys))
		// Engine keys are prefixed with the chart name, e.g.
		// provider-x/templates/provider.yaml.
		source := strings.TrimPrefix(name, chrt.Name()+"/")
		for _, k := range keys {
			o, ok, err := decode(docs[k])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", source, err)
			}
			if !ok {
				continue
			}
			o.Source = source
			objects = append(objects, o)
		}
	}
	return objects, nil
}

// crdAPIVersions returns the API versions the CRDs in the crds/ directories
// of chrt and of the subcharts it enables serve, in the forms Helm's
// discovery lists them: group/version and group/version/Kind. Helm installs
// these CRDs before it renders, so a fresh install renders with them; the
// CRDs templates render are applied with the rest of the release, after it.
func crdAPIVersions(chrt *chart.Chart) ([]string, error) {
	var apis []string
	for _, f := range chrt.CRDObjects() {
		docs := releaseutil.SplitManifests(string(f.File.Data))
		keys := make([]string, 0, len(docs))
		for k := range docs {
			keys = append(keys, k)
		}
		sort.Sort(releaseutil.BySplitManifestsOrder(keys))
		for _, k := range keys {
			var crd struct {
				Kind string `json:"kind"`
				Spec struct {
					Group string `json:"group"`
					Names struct {
						Kind string `json:"kind"`
					} `json:"names"`
					Versions []struct {
						Name   string `json:"name"`
						Served bool   `json:"served"`
					} `json:"versions"`
				} `json:"spec"`
			}
			if err := yaml.Unmarshal([]byte(docs[k]), &crd); err != nil {
				return nil, fmt.Errorf("%s: decoding CRD: %w", f.Filename, err)
			}
			if crd.Kind != "CustomResourceDefinition" {
				continue
			}
			for _, v := range crd.Spec.Versions {
				if v.Served {
					gv := crd.Spec.Group + "/" + v.Name
					apis = append(apis, gv, gv+"/"+crd.Spec.Names.Kind)
				}
			}
		}
	}
	return apis, nil
}

// splitAPIVersions splits the value of APIVersionsAnnotation into its
// entries.
func splitAPIVersions(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
}

func decode(doc string) (Object, bool, error) {
	var head struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Name        string            `json:"name"`
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	}
	if err := yaml.Unmarshal([]byte(doc), &head); err != nil {
		return Object{}, false, fmt.Errorf("decoding rendered object: %w", err)
	}
	if head.Kind == "" {
		return Object{}, false, nil
	}
	gv, err := schema.ParseGroupVersion(head.APIVersion)
	if err != nil {
		return Object{}, false, fmt.Errorf("%s %s: %w", head.Kind, head.Metadata.Name, err)
	}
	return Object{
		GroupKind:   schema.GroupKind{Group: gv.Group, Kind: head.Kind},
		Name:        head.Metadata.Name,
		Annotations: head.Metadata.Annotations,
		Manifest:    doc,
	}, true, nil
}
