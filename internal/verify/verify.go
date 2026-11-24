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
// OpenEverest it is built against.
package verify

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/openeverest/openeverest/v2/contract"
	"github.com/openeverest/provider-sdk/internal/chartrender"
	"github.com/openeverest/provider-sdk/internal/compat"
	"github.com/openeverest/provider-sdk/internal/generate"
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
	// checked with Release.
	Core compat.Module
}

// Result is the outcome of Run.
type Result struct {
	// Stamped lists what carries the expected stamp: Chart.yaml and the
	// objects OpenEverest interprets.
	Stamped []string

	// Problems lists what is wrong. The chart passes when it is empty.
	Problems []string
}

// Run checks that the chart in opts.ChartDir carries the stamp
// opts.ContractVersion in Chart.yaml and on every object OpenEverest
// interprets that it renders with its default values, and that Argo CD
// applies those objects before any workload: the stamped objects at sync
// wave generate.StampSyncWave, and no workload, a subchart's included, at
// that wave or before it. It returns an error only when it cannot check,
// e.g. when the chart does not render.
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
	providers := 0
	for _, o := range objects {
		what := fmt.Sprintf("%s %s (%s)", o.GroupKind.Kind, o.Name, o.Source)
		if o.Workload() && syncPhase(o.Annotations) {
			if wave := syncWave(o.Annotations); wave <= stampWave {
				res.Problems = append(res.Problems, fmt.Sprintf(
					"%s is at Argo CD sync wave %d; put it after wave %s, that of the objects OpenEverest checks, so that Argo CD stops the sync before it runs when OpenEverest refuses one of them",
					what, wave, generate.StampSyncWave))
			}
		}
		// The objects of subcharts belong to other charts, with stamps of
		// their own.
		if !o.Own() || !o.CoreInterpreted() {
			continue
		}
		if o.GroupKind.Kind == "Provider" {
			providers++
		}
		hint := fmt.Sprintf("include %q, from templates/%s, in its metadata.annotations",
			generate.ContractAnnotationsHelper, generate.ContractHelperFile)
		if o.GroupKind.Kind == "BackupClass" {
			hint = "run `make generate`, which stamps the BackupClasses it generates"
		}
		stamp, ok := o.Annotations[contract.VersionAnnotation]
		if !ok {
			res.Problems = append(res.Problems, fmt.Sprintf("%s has no %s annotation; %s", what, contract.VersionAnnotation, hint))
			continue
		}
		stamped := true
		if stamp != opts.ContractVersion {
			res.Problems = append(res.Problems, fmt.Sprintf("%s is stamped %s, but this provider-sdk builds against %s; run `make generate`", what, stamp, opts.ContractVersion))
			stamped = false
		}
		if wave := syncWave(o.Annotations); wave != stampWave {
			res.Problems = append(res.Problems, fmt.Sprintf("%s is at Argo CD sync wave %d, not %s; %s", what, wave, generate.StampSyncWave, hint))
			stamped = false
		}
		if stamped {
			res.Stamped = append(res.Stamped, what)
		}
	}
	if providers == 0 {
		res.Problems = append(res.Problems, "the chart renders no Provider with its default values")
	}

	if opts.Release {
		res.Problems = append(res.Problems, compat.ReleaseProblems(opts.Core, opts.ContractVersion)...)
	}
	return res, nil
}

// Annotations Argo CD reads to order a sync, besides
// generate.SyncWaveAnnotation.
const (
	argoCDHookAnnotation     = "argocd.argoproj.io/hook"
	helmHookAnnotation       = "helm.sh/hook"
	helmHookWeightAnnotation = "helm.sh/hook-weight"
)

// stampWave is the sync wave of the stamped objects.
var stampWave = syncWave(map[string]string{generate.SyncWaveAnnotation: generate.StampSyncWave})

// syncWave returns the Argo CD sync wave of an object with annotations, as
// Argo CD reads it: its sync-wave annotation, else its Helm hook weight,
// else 0.
func syncWave(annotations map[string]string) int {
	for _, key := range []string{generate.SyncWaveAnnotation, helmHookWeightAnnotation} {
		if wave, err := strconv.Atoi(annotations[key]); err == nil {
			return wave
		}
	}
	return 0
}

// syncPhase reports whether Argo CD applies an object with annotations in
// the sync phase, ordered by sync wave with the stamped objects. Hooks run in
// phases of their own, such as PreSync and PostSync, except Argo CD Sync
// hooks; Argo CD maps Helm hooks to those phases, and ignores them on an
// object with Argo CD hooks.
func syncPhase(annotations map[string]string) bool {
	if hooks, ok := annotations[argoCDHookAnnotation]; ok {
		for hook := range strings.SplitSeq(hooks, ",") {
			if strings.TrimSpace(hook) == "Sync" {
				return true
			}
		}
		return false
	}
	_, ok := annotations[helmHookAnnotation]
	return !ok
}
