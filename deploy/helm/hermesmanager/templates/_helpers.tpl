{{/*
Expand the name of the chart.
*/}}
{{- define "hermesmanager.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "hermesmanager.fullname" -}}
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
{{- define "hermesmanager.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "hermesmanager.labels" -}}
helm.sh/chart: {{ include "hermesmanager.chart" . }}
{{ include "hermesmanager.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "hermesmanager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hermesmanager.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Service account name.
*/}}
{{- define "hermesmanager.serviceAccountName" -}}
{{- include "hermesmanager.fullname" . }}
{{- end }}

{{/*
Watch namespace — defaults to the release namespace if not set.
*/}}
{{- define "hermesmanager.watchNamespace" -}}
{{- if .Values.watchNamespace }}
{{- .Values.watchNamespace }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
PostgreSQL secret name — CloudNativePG generates a secret named <cluster>-app.
*/}}
{{- define "hermesmanager.pgSecretName" -}}
{{- printf "%s-postgres-app" (include "hermesmanager.fullname" .) }}
{{- end }}

{{/*
Database URL constructed from CloudNativePG secret fields.
The actual value is assembled at runtime from the secret; this helper produces the
secret key reference name.
*/}}
{{- define "hermesmanager.pgClusterName" -}}
{{- printf "%s-postgres" (include "hermesmanager.fullname" .) }}
{{- end }}
