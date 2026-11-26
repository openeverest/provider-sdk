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

import "github.com/openeverest/provider-sdk/internal/chartrender"

// renderHelp describes, for generate and verify, the cluster the chart is
// rendered for.
const renderHelp = `The chart is rendered without a cluster, as a fresh install with its default
values would be, for a cluster that serves the APIs of Kubernetes, those of
the CRDs in the crds/ directories of the chart and of the subcharts it
enables, which Helm installs before it renders, and the API versions the
Chart.yaml annotation ` + chartrender.APIVersionsAnnotation + ` lists, separated by commas:
those of the chart's prerequisites, such as cert-manager.io/v1/Certificate
for a bundled operator chart that requires cert-manager. --api-versions adds
more, as with helm template.`

const apiVersionsUsage = "API versions the cluster serves when the chart is rendered, on top of those Chart.yaml's " +
	chartrender.APIVersionsAnnotation + " annotation lists (as helm template --api-versions)"
