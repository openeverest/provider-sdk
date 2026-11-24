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

// Workloads lists the kinds of objects that run code in the cluster.
var Workloads = map[schema.GroupKind]bool{
	{Group: "apps", Kind: "Deployment"}:  true,
	{Group: "apps", Kind: "StatefulSet"}: true,
	{Group: "apps", Kind: "DaemonSet"}:   true,
	{Group: "batch", Kind: "Job"}:        true,
	{Group: "batch", Kind: "CronJob"}:    true,
	{Group: "", Kind: "Pod"}:             true,
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
}

// CoreInterpreted reports whether core interprets o.
func (o Object) CoreInterpreted() bool {
	_, ok := CoreInterpreted[o.GroupKind]
	return ok
}

// Workload reports whether o runs code in the cluster.
func (o Object) Workload() bool {
	return Workloads[o.GroupKind]
}

// Own reports whether one of the chart's own templates rendered o, rather
// than one of its subcharts', such as a bundled operator's.
func (o Object) Own() bool {
	return strings.HasPrefix(o.Source, "templates/")
}

// Render renders the chart in dir with its default values, as a fresh
// install would, and returns the objects it applies, its subcharts'
// included, in template order. Nothing is read from a cluster: lookup
// returns nothing. Templates named in skip, relative to the chart (e.g.
// templates/check.yaml), are not rendered.
func Render(dir string, skip ...string) ([]Object, error) {
	chrt, err := loader.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("loading chart %s: %w", dir, err)
	}
	if len(skip) > 0 {
		kept := chrt.Templates[:0]
		for _, t := range chrt.Templates {
			if !slices.Contains(skip, t.Name) {
				kept = append(kept, t)
			}
		}
		chrt.Templates = kept
	}
	if err := chartutil.ProcessDependenciesWithMerge(chrt, map[string]any{}); err != nil {
		return nil, fmt.Errorf("processing chart dependencies: %w", err)
	}
	vals, err := chartutil.ToRenderValues(chrt, map[string]any{}, chartutil.ReleaseOptions{
		Name:      chrt.Name(),
		Namespace: "default",
		Revision:  1,
		IsInstall: true,
	}, chartutil.DefaultCapabilities)
	if err != nil {
		return nil, fmt.Errorf("computing chart values: %w", err)
	}
	files, err := engine.Render(chrt, vals)
	if err != nil {
		return nil, fmt.Errorf("rendering chart %s: %w", dir, err)
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
	}, true, nil
}
