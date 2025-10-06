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
Dispatcher labels
*/}}
{{- define "llm-proxy.dispatcherLabels" -}}
helm.sh/chart: {{ include "llm-proxy.chart" . }}
{{ include "llm-proxy.dispatcherSelectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Dispatcher selector labels
*/}}
{{- define "llm-proxy.dispatcherSelectorLabels" -}}
app.kubernetes.io/name: {{ include "llm-proxy.name" . }}-dispatcher
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: dispatcher
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
Create the image name
*/}}
{{- define "llm-proxy.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" }}
{{- $repository := .Values.image.repository }}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
Create the dispatcher image name
*/}}
{{- define "llm-proxy.dispatcherImage" -}}
{{- $registry := .Values.global.imageRegistry | default "" }}
{{- $repository := .Values.dispatcher.image.repository | default .Values.image.repository }}
{{- $tag := .Values.dispatcher.image.tag | default (.Values.image.tag | default .Chart.AppVersion) }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
Redis connection settings
*/}}
{{- define "llm-proxy.redisAddr" -}}
{{- if .Values.redis.enabled }}
{{- printf "%s-redis-master:6379" .Release.Name }}
{{- else }}
{{- printf "%s:%d" .Values.redis.external.host (.Values.redis.external.port | int) }}
{{- end }}
{{- end }}

{{/*
Database connection string
*/}}
{{- define "llm-proxy.databaseUrl" -}}
{{- if eq .Values.config.database.type "postgresql" }}
{{- printf "postgres://%s:%s@%s:%d/%s?sslmode=%s" .Values.config.database.postgresql.user .Values.config.database.postgresql.password .Values.config.database.postgresql.host (.Values.config.database.postgresql.port | int) .Values.config.database.postgresql.database .Values.config.database.postgresql.sslmode }}
{{- else }}
{{- .Values.config.database.sqlite.path }}
{{- end }}
{{- end }}

{{/*
Secret name for management token
*/}}
{{- define "llm-proxy.managementTokenSecret" -}}
{{- if .Values.secrets.external }}
{{- .Values.secrets.externalSecrets.managementToken }}
{{- else }}
{{- printf "%s-secrets" (include "llm-proxy.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Secret name for OpenAI API key
*/}}
{{- define "llm-proxy.openaiApiKeySecret" -}}
{{- if .Values.secrets.external }}
{{- .Values.secrets.externalSecrets.openaiApiKey }}
{{- else }}
{{- printf "%s-secrets" (include "llm-proxy.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Secret name for Redis password
*/}}
{{- define "llm-proxy.redisPasswordSecret" -}}
{{- if .Values.secrets.external }}
{{- .Values.secrets.externalSecrets.redisPassword }}
{{- else if .Values.redis.enabled }}
{{- printf "%s-redis" .Release.Name }}
{{- else }}
{{- printf "%s-secrets" (include "llm-proxy.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Common environment variables
*/}}
{{- define "llm-proxy.env" -}}
- name: LISTEN_ADDR
  value: {{ .Values.config.listenAddr | quote }}
- name: LOG_LEVEL
  value: {{ .Values.config.logLevel | quote }}
- name: LOG_FORMAT
  value: {{ .Values.config.logFormat | quote }}
- name: MANAGEMENT_TOKEN
  valueFrom:
    secretKeyRef:
      name: {{ include "llm-proxy.managementTokenSecret" . }}
      key: management-token
- name: DATABASE_PATH
  {{- if eq .Values.config.database.type "postgresql" }}
  value: {{ include "llm-proxy.databaseUrl" . | quote }}
  {{- else }}
  value: {{ .Values.config.database.sqlite.path | quote }}
  {{- end }}
- name: OPENAI_API_URL
  value: {{ .Values.config.openai.apiUrl | quote }}
- name: REQUEST_TIMEOUT
  value: {{ .Values.config.openai.requestTimeout | quote }}
- name: MAX_REQUEST_SIZE
  value: {{ .Values.config.openai.maxRequestSize | quote }}
- name: ENABLE_STREAMING
  value: {{ .Values.config.openai.enableStreaming | quote }}
- name: CORS_ALLOWED_ORIGINS
  value: {{ .Values.config.security.corsAllowedOrigins | quote }}
- name: CORS_ALLOWED_METHODS
  value: {{ .Values.config.security.corsAllowedMethods | quote }}
- name: CORS_ALLOWED_HEADERS
  value: {{ .Values.config.security.corsAllowedHeaders | quote }}
- name: CORS_MAX_AGE
  value: {{ .Values.config.security.corsMaxAge | quote }}
- name: MASK_API_KEYS
  value: {{ .Values.config.security.maskApiKeys | quote }}
- name: VALIDATE_API_KEY_FORMAT
  value: {{ .Values.config.security.validateApiKeyFormat | quote }}
- name: DEFAULT_TOKEN_LIFETIME
  value: {{ .Values.config.security.defaultTokenLifetime | quote }}
- name: DEFAULT_TOKEN_REQUEST_LIMIT
  value: {{ .Values.config.security.defaultTokenRequestLimit | quote }}
- name: GLOBAL_RATE_LIMIT
  value: {{ .Values.config.rateLimiting.globalRateLimit | quote }}
- name: IP_RATE_LIMIT
  value: {{ .Values.config.rateLimiting.ipRateLimit | quote }}
- name: MAX_CONCURRENT_REQUESTS
  value: {{ .Values.config.performance.maxConcurrentRequests | quote }}
- name: WORKER_POOL_SIZE
  value: {{ .Values.config.performance.workerPoolSize | quote }}
- name: ENABLE_METRICS
  value: {{ .Values.config.monitoring.enableMetrics | quote }}
- name: METRICS_PATH
  value: {{ .Values.config.monitoring.metricsPath | quote }}
- name: TOKEN_CLEANUP_INTERVAL
  value: {{ .Values.config.tokenCleanupInterval | quote }}
- name: OBSERVABILITY_ENABLED
  value: {{ .Values.config.observability.enabled | quote }}
- name: OBSERVABILITY_BUFFER_SIZE
  value: {{ .Values.config.observability.bufferSize | quote }}
- name: LLM_PROXY_EVENT_BUS
  value: "redis"
- name: REDIS_ADDR
  value: {{ include "llm-proxy.redisAddr" . | quote }}
{{- if .Values.adminUI.enabled }}
- name: ADMIN_UI_ENABLED
  value: "true"
- name: ADMIN_UI_PATH
  value: {{ .Values.adminUI.path | quote }}
{{- if .Values.adminUI.apiBaseUrl }}
- name: ADMIN_UI_API_BASE_URL
  value: {{ .Values.adminUI.apiBaseUrl | quote }}
{{- end }}
{{- end }}
{{- range .Values.env }}
- name: {{ .name }}
  {{- if .value }}
  value: {{ .value | quote }}
  {{- else if .valueFrom }}
  valueFrom:
    {{- toYaml .valueFrom | nindent 4 }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
Volume mounts
*/}}
{{- define "llm-proxy.volumeMounts" -}}
{{- if .Values.persistence.enabled }}
- name: data
  mountPath: {{ .Values.persistence.dataPath }}
- name: logs
  mountPath: {{ .Values.persistence.logsPath }}
{{- end }}
- name: tmp
  mountPath: /tmp
{{- range .Values.volumeMounts }}
- name: {{ .name }}
  mountPath: {{ .mountPath }}
  {{- if .readOnly }}
  readOnly: {{ .readOnly }}
  {{- end }}
  {{- if .subPath }}
  subPath: {{ .subPath }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
Volumes
*/}}
{{- define "llm-proxy.volumes" -}}
{{- if .Values.persistence.enabled }}
- name: data
  persistentVolumeClaim:
    claimName: {{ include "llm-proxy.fullname" . }}-data
- name: logs
  persistentVolumeClaim:
    claimName: {{ include "llm-proxy.fullname" . }}-logs
{{- end }}
- name: tmp
  emptyDir: {}
{{- range .Values.volumes }}
- name: {{ .name }}
  {{- if .configMap }}
  configMap:
    name: {{ .configMap.name }}
    {{- if .configMap.items }}
    items:
      {{- toYaml .configMap.items | nindent 6 }}
    {{- end }}
  {{- else if .secret }}
  secret:
    secretName: {{ .secret.secretName }}
    {{- if .secret.items }}
    items:
      {{- toYaml .secret.items | nindent 6 }}
    {{- end }}
  {{- else if .emptyDir }}
  emptyDir: {}
  {{- end }}
{{- end }}
{{- end }}