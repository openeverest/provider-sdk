{{/*
Expand the name of the chart.
*/}}
{{- define "[[ .ProjectName ]].name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "[[ .ProjectName ]].fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "[[ .ProjectName ]].chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "[[ .ProjectName ]].labels" -}}
helm.sh/chart: {{ include "[[ .ProjectName ]].chart" . }}
{{ include "[[ .ProjectName ]].selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "[[ .ProjectName ]].selectorLabels" -}}
app.kubernetes.io/name: {{ include "[[ .ProjectName ]].name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "[[ .ProjectName ]].serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "[[ .ProjectName ]].fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Provider short name
*/}}
{{- define "[[ .ProjectName ]].shortName" -}}
{{- .Values.provider.shortName -}}
{{- end }}

{{/*
Container image
*/}}
{{- define "[[ .ProjectName ]].image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end }}

{{/*
Name of the Secret holding the admission webhook's serving certificate
*/}}
{{- define "[[ .ProjectName ]].admissionSecretName" -}}
{{- printf "%s-admission-cert" (include "[[ .ProjectName ]].fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
The admission webhook's serving certificate: from values, else the Secret the
previous release created, else a new CA and certificate.
*/}}
{{- define "[[ .ProjectName ]].admissionCerts" -}}
{{- $certs := .Values.admission.certs }}
{{- $current := lookup "v1" "Secret" .Release.Namespace (include "[[ .ProjectName ]].admissionSecretName" .) }}
{{- if and (index $certs "tls.crt") (index $certs "tls.key") (index $certs "ca.crt") }}
tls.crt: {{ index $certs "tls.crt" | b64enc }}
tls.key: {{ index $certs "tls.key" | b64enc }}
ca.crt: {{ index $certs "ca.crt" | b64enc }}
{{- else if and $current (index $current.data "ca.crt") }}
tls.crt: {{ index $current.data "tls.crt" }}
tls.key: {{ index $current.data "tls.key" }}
ca.crt: {{ index $current.data "ca.crt" }}
{{- else }}
{{- $service := include "[[ .ProjectName ]].fullname" . }}
{{- $dnsName := printf "%s.%s.svc" $service .Release.Namespace }}
{{- $altNames := list $service (printf "%s.%s" $service .Release.Namespace) $dnsName (printf "%s.cluster.local" $dnsName) }}
{{- $ca := genCA (printf "%s-admission-ca" $service) 3650 }}
{{- $cert := genSignedCert $dnsName nil $altNames 3650 $ca }}
tls.crt: {{ $cert.Cert | b64enc }}
tls.key: {{ $cert.Key | b64enc }}
ca.crt: {{ $ca.Cert | b64enc }}
{{- end }}
{{- end }}
