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
Create the name of the service account to use
*/}}
{{- define "llm-proxy.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "llm-proxy.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Get the secret name for management token
*/}}
{{- define "llm-proxy.managementTokenSecretName" -}}
{{- if .Values.secrets.create }}
{{- include "llm-proxy.fullname" . }}
{{- else }}
{{- .Values.secrets.managementToken.existingSecret.name }}
{{- end }}
{{- end }}

{{/*
Get the secret key for management token
*/}}
{{- define "llm-proxy.managementTokenSecretKey" -}}
{{- if .Values.secrets.create -}}
MANAGEMENT_TOKEN
{{- else -}}
{{- .Values.secrets.managementToken.existingSecret.key -}}
{{- end -}}
{{- end -}}

{{/*
Get the secret name for database URL
*/}}
{{- define "llm-proxy.databaseUrlSecretName" -}}
{{- if .Values.secrets.create }}
{{- include "llm-proxy.fullname" . }}
{{- else }}
{{- .Values.secrets.databaseUrl.existingSecret.name }}
{{- end }}
{{- end }}

{{/*
Get the secret key for database URL
*/}}
{{- define "llm-proxy.databaseUrlSecretKey" -}}
{{- if .Values.secrets.create -}}
DATABASE_URL
{{- else -}}
{{- .Values.secrets.databaseUrl.existingSecret.key -}}
{{- end -}}
{{- end -}}
