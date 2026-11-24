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

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openeverest/openeverest/v2/contract"
	"github.com/openeverest/provider-sdk/internal/compat"
	"github.com/openeverest/provider-sdk/internal/generate"
	"github.com/openeverest/provider-sdk/internal/verify"
)

var verifyOpts struct {
	chartDir string
	release  bool
}

func init() {
	f := verifyCmd.Flags()
	f.StringVar(&verifyOpts.chartDir, "chart-dir", "", "Path to the Helm chart directory (default: auto-detect from charts/)")
	f.BoolVar(&verifyOpts.release, "release", false, "Also fail when the provider is built against an unreleased OpenEverest")

	rootCmd.AddCommand(verifyCmd)
}

var verifyCmd = &cobra.Command{
	Use:          "verify",
	Short:        "Check the Helm chart is stamped with the OpenEverest it is built against",
	SilenceUsage: true,
	Long: `Render the Helm chart with its default values, without a cluster, and
check that Chart.yaml and every object OpenEverest interprets (Provider,
BackupClass, BackupStorageClass, MonitoringClass, ClusterInstancePreset) carry
the annotation openeverest.io/contract-version with the contract version of the
OpenEverest this provider is built against, the core its go.mod requires.

Those objects must also be at Argo CD sync wave -1, and no workload
(Deployment, StatefulSet, DaemonSet, Job, CronJob, Pod), a subchart's
included, at -1 or before: Argo CD then applies the objects OpenEverest checks
before anything runs, and stops the sync when OpenEverest refuses one.

provider-sdk generate writes the stamp into Chart.yaml, and the stamp and the
wave on the generated BackupClasses; chart templates render both on other
objects with {{ include "openeverest.contract.annotations" . }}.

With --release, also fail when the stamp is a development contract version
(OpenEverest's contract changed since its last release) or when go.mod replaces
OpenEverest. A release names the oldest OpenEverest it works with, which users
must be able to install. Requiring a commit of OpenEverest's main branch is
fine while main's contract is still that of a release.

Run it as ` + "`go tool provider-sdk verify`" + ` from the provider's module.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		required, err := checkLinkedCore()
		if err != nil {
			return err
		}
		chartDir := verifyOpts.chartDir
		if chartDir == "" {
			if chartDir, err = generate.DetectChartDir(); err != nil {
				return fmt.Errorf("auto-detecting chart directory: %w; specify --chart-dir explicitly", err)
			}
		}

		res, err := verify.Run(verify.Options{
			ChartDir:        chartDir,
			ContractVersion: contract.Version,
			Release:         verifyOpts.release,
			Core:            required,
		})
		if err != nil {
			return err
		}
		if len(res.Stamped) > 0 {
			fmt.Fprintf(os.Stderr, "Stamped with %s=%s:\n", contract.VersionAnnotation, contract.Version)
			for _, s := range res.Stamped {
				fmt.Fprintf(os.Stderr, "  %s\n", s)
			}
		}
		if len(res.Problems) > 0 {
			var b strings.Builder
			fmt.Fprintf(&b, "%d problem(s) in %s:", len(res.Problems), chartDir)
			for _, p := range res.Problems {
				fmt.Fprintf(&b, "\n  - %s", p)
			}
			return errors.New(b.String())
		}
		fmt.Fprintln(os.Stderr, compat.Floor(contract.Version))
		return nil
	},
}

// checkLinkedCore returns the OpenEverest core the module in the working
// directory requires, after checking it is the one this binary links: the
// stamp is the contract version of the linked core, so it is only right for
// a provider built against that same core.
func checkLinkedCore() (compat.Module, error) {
	linked, err := compat.LinkedCore()
	if err != nil {
		return compat.Module{}, err
	}
	required, err := compat.RequiredCore(".")
	if err != nil {
		return compat.Module{}, err
	}
	warning, err := compat.CheckCore(linked, required)
	if err != nil {
		return compat.Module{}, err
	}
	if warning != "" {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
	}
	return required, nil
}
