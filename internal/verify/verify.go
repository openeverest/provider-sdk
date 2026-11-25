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

// Package verify checks that a provider's chart carries the stamp of the
// OpenEverest it is built against, and grants the provider what the
// provider-runtime of that OpenEverest needs to start, and that the
// provider's image is built against that same OpenEverest.
package verify

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"

	"github.com/openeverest/openeverest/v2/contract"
	"github.com/openeverest/provider-sdk/internal/chartrender"
	"github.com/openeverest/provider-sdk/internal/compat"
	"github.com/openeverest/provider-sdk/internal/generate"
	"github.com/openeverest/provider-sdk/internal/imagebin"
)

// Options configures Run.
type Options struct {
	// ChartDir is the provider's chart directory.
	ChartDir string

	// ContractVersion is the stamp every object must carry: the contract
	// version of the OpenEverest core this build links.
	ContractVersion string

	// Release also rejects what must not be released: a chart built against
	// an unreleased OpenEverest.
	Release bool

	// Core is the OpenEverest core module the provider's go.mod requires,
	// checked with Release and Image.
	Core compat.Module

	// Image, when set, is the provider's image, which the chart must deploy
	// and whose binary must link Core on every platform.
	Image *Image
}

// Image is a provider's image.
type Image struct {
	// Reference names the image, such as ghcr.io/openeverest/provider-x:1.2.3.
	Reference string

	// Binaries are the binaries the image runs, one per platform.
	Binaries []imagebin.Binary
}

// Result is the outcome of Run.
type Result struct {
	// Stamped lists what carries the expected stamp: Chart.yaml and the
	// objects OpenEverest interprets.
	Stamped []string

	// Linked lists the binaries of Options.Image that link the core the
	// chart is stamped for, as platform and path.
	Linked []string

	// Problems lists what is wrong. The chart passes when it is empty.
	Problems []string
}

// Run checks that the chart in opts.ChartDir carries the stamp
// opts.ContractVersion in Chart.yaml and on every object OpenEverest
// interprets that it renders with its default values, and that its
// ClusterRoles allow what provider-runtime's startup gate requests. It
// returns an error only when it cannot check, e.g. when the chart does not
// render.
func Run(opts Options) (Result, error) {
	var res Result

	chartFile := filepath.Join(opts.ChartDir, "Chart.yaml")
	meta, err := chartutil.LoadChartfile(chartFile)
	if err != nil {
		return res, fmt.Errorf("loading %s: %w", chartFile, err)
	}
	switch stamp, ok := meta.Annotations[contract.VersionAnnotation]; {
	case !ok:
		res.Problems = append(res.Problems, fmt.Sprintf("Chart.yaml has no %s annotation; run `make generate`", contract.VersionAnnotation))
	case stamp != opts.ContractVersion:
		res.Problems = append(res.Problems, fmt.Sprintf("Chart.yaml is stamped %s, but this provider-sdk builds against %s; run `make generate`", stamp, opts.ContractVersion))
	default:
		res.Stamped = append(res.Stamped, "Chart.yaml")
	}

	objects, err := chartrender.Render(opts.ChartDir)
	if err != nil {
		return res, err
	}
	var providers []string
	for _, o := range objects {
		// The objects of subcharts belong to other charts, with stamps of
		// their own.
		if !o.Own() || !o.CoreInterpreted() {
			continue
		}
		if o.GroupKind == providerKind {
			providers = append(providers, o.Name)
		}
		what := fmt.Sprintf("%s %s (%s)", o.GroupKind.Kind, o.Name, o.Source)
		hint := fmt.Sprintf("include %q, from templates/%s, in its metadata.annotations",
			generate.ContractAnnotationsHelper, generate.ContractHelperFile)
		if o.GroupKind.Kind == "BackupClass" {
			hint = "run `make generate`, which stamps the BackupClasses it generates"
		}
		switch stamp, ok := o.Annotations[contract.VersionAnnotation]; {
		case !ok:
			res.Problems = append(res.Problems, fmt.Sprintf("%s has no %s annotation; %s", what, contract.VersionAnnotation, hint))
		case stamp != opts.ContractVersion:
			res.Problems = append(res.Problems, fmt.Sprintf("%s is stamped %s, but this provider-sdk builds against %s; run `make generate`", what, stamp, opts.ContractVersion))
		default:
			res.Stamped = append(res.Stamped, what)
		}
	}
	if len(providers) == 0 {
		res.Problems = append(res.Problems, "the chart renders no Provider with its default values")
	}
	gateProblems, err := startupGateProblems(objects, providers)
	if err != nil {
		return res, err
	}
	res.Problems = append(res.Problems, gateProblems...)

	// The templates generate writes must be those of this provider-sdk, and
	// the check must read the CRDs of what the chart applies.
	for _, generated := range []struct{ file, want string }{
		{generate.ContractHelperFile, generate.ContractHelper},
		{generate.CompatCheckFile, generate.CompatCheck(generate.CheckedCRDs(objects))},
	} {
		file := generated.file
		got, err := os.ReadFile(filepath.Join(opts.ChartDir, "templates", file))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			res.Problems = append(res.Problems, fmt.Sprintf("templates/%s is missing; run `make generate`", file))
		case err != nil:
			return res, fmt.Errorf("reading templates/%s: %w", file, err)
		case string(got) != generated.want:
			res.Problems = append(res.Problems, fmt.Sprintf("templates/%s is out of date; run `make generate`", file))
		}
	}

	// OpenEverest's admission reads an acknowledgement on the object it
	// admits: a stamped object without it would be refused even after the
	// user acknowledged the incompatibility and the chart's check let the
	// install through.
	acknowledged, err := chartrender.RenderValues(opts.ChartDir, map[string]any{
		"compatibility": map[string]any{"acknowledge": map[string]any{"coreVersion": acknowledgeProbe}},
	})
	if err != nil {
		return res, fmt.Errorf("rendering the chart with %s set: %w", contract.AcknowledgeValuePath, err)
	}
	for _, o := range acknowledged {
		if !o.Own() || !o.CoreInterpreted() {
			continue
		}
		// An object without the stamp is reported above.
		if _, ok := o.Annotations[contract.VersionAnnotation]; !ok || o.Annotations[contract.AcknowledgedAnnotation] == acknowledgeProbe {
			continue
		}
		hint := fmt.Sprintf("include %q, from templates/%s, in its metadata.annotations",
			generate.ContractAnnotationsHelper, generate.ContractHelperFile)
		if o.GroupKind.Kind == "BackupClass" {
			hint = fmt.Sprintf("render the generated BackupClasses with {{ include %q (dict \"root\" $ \"path\" $path) }}, as the scaffolded templates/backupclasses.yaml does",
				generate.GeneratedObjectHelper)
		}
		res.Problems = append(res.Problems, fmt.Sprintf(
			"%s %s (%s) has no %s annotation when %s is set, so OpenEverest would refuse it even once the user acknowledged it; %s",
			o.GroupKind.Kind, o.Name, o.Source, contract.AcknowledgedAnnotation, contract.AcknowledgeValuePath, hint))
	}

	if opts.Release {
		res.Problems = append(res.Problems, compat.ReleaseProblems(opts.Core, opts.ContractVersion)...)
	}
	if opts.Image != nil {
		checkImage(&res, objects, *opts.Image, opts.Core)
	}
	return res, nil
}

// checkImage checks that the chart deploys image, and that the binary image
// runs on every platform links core. The stamp is the contract version of
// core, and provider-runtime's startup gate checks the contract version of
// the core the binary links: they must be the same, or the gate and what
// reads the stamp, such as the chart's own check, would decide on different
// versions.
func checkImage(res *Result, objects []chartrender.Object, image Image, core compat.Module) {
	deployed := containerImages(objects)
	if !slices.Contains(deployed, image.Reference) {
		what := "no image"
		if len(deployed) > 0 {
			what = strings.Join(deployed, ", ")
		}
		res.Problems = append(res.Problems, fmt.Sprintf("the chart deploys %s, not %s, with its default values", what, image.Reference))
	}
	for _, b := range image.Binaries {
		linked, err := compat.CheckBinary(b.BuildInfo, core)
		if err != nil {
			res.Problems = append(res.Problems, fmt.Sprintf("%s (%s): %s %v", image.Reference, b.Platform, b.Path, err))
			continue
		}
		res.Linked = append(res.Linked, fmt.Sprintf("%s %s, which links %s", b.Platform, b.Path, linked))
	}
}

// containerImages returns the images of the containers of the chart's own
// workloads.
func containerImages(objects []chartrender.Object) []string {
	var images []string
	for _, o := range objects {
		if !o.Own() {
			continue
		}
		var obj map[string]any
		if err := yaml.Unmarshal([]byte(o.Manifest), &obj); err != nil {
			continue
		}
		images = appendImages(images, obj)
	}
	slices.Sort(images)
	return slices.Compact(images)
}

// appendImages appends the images of the containers in v, at any depth, to
// images: the pod template of a Deployment, a Job or a CronJob alike.
func appendImages(images []string, v any) []string {
	switch v := v.(type) {
	case map[string]any:
		for key, value := range v {
			if containers, ok := value.([]any); ok && (key == "containers" || key == "initContainers") {
				for _, c := range containers {
					if c, ok := c.(map[string]any); ok {
						if image, ok := c["image"].(string); ok {
							images = append(images, image)
						}
					}
				}
				continue
			}
			images = appendImages(images, value)
		}
	case []any:
		for _, value := range v {
			images = appendImages(images, value)
		}
	}
	return images
}

// acknowledgeProbe is the release verify acknowledges to check that the
// acknowledgement reaches every stamped object. Any release does: only
// whether it reaches them matters.
const acknowledgeProbe = "1.0.0"
