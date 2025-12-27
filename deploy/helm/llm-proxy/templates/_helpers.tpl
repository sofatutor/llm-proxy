{{/*
Expand the name of the chart.
*/}}
{{- define "llm-proxy.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "llm-proxy.fullname" -}}
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
{{- define "llm-proxy.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "llm-proxy.labels" -}}
helm.sh/chart: {{ include "llm-proxy.chart" . }}
{{ include "llm-proxy.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "llm-proxy.selectorLabels" -}}
app.kubernetes.io/name: {{ include "llm-proxy.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Get the name of the secret containing MANAGEMENT_TOKEN
*/}}
{{- define "llm-proxy.managementTokenSecretName" -}}
{{- if and .Values.secrets.create .Values.secrets.data.managementToken }}
{{- include "llm-proxy.fullname" . }}
{{- else if .Values.secrets.managementToken.existingSecret.name }}
{{- .Values.secrets.managementToken.existingSecret.name }}
{{- end }}
{{- end }}

{{/*
Get the key within the secret for MANAGEMENT_TOKEN
*/}}
{{- define "llm-proxy.managementTokenSecretKey" -}}
{{- if .Values.secrets.create }}
{{- printf "MANAGEMENT_TOKEN" }}
{{- else }}
{{- .Values.secrets.managementToken.existingSecret.key | default "MANAGEMENT_TOKEN" }}
{{- end }}
{{- end }}

{{/*
Get the name of the secret containing DATABASE_URL
*/}}
{{- define "llm-proxy.databaseUrlSecretName" -}}
{{- if and .Values.secrets.create .Values.secrets.data.databaseUrl }}
{{- include "llm-proxy.fullname" . }}
{{- else if .Values.secrets.databaseUrl.existingSecret.name }}
{{- .Values.secrets.databaseUrl.existingSecret.name }}
{{- end }}
{{- end }}

{{/*
Get the key within the secret for DATABASE_URL
*/}}
{{- define "llm-proxy.databaseUrlSecretKey" -}}
{{- if .Values.secrets.create }}
{{- printf "DATABASE_URL" }}
{{- else }}
{{- .Values.secrets.databaseUrl.existingSecret.key | default "DATABASE_URL" }}
{{- end }}
{{- end }}
