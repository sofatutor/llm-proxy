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

{{/*
Get the name of the secret containing REDIS_PASSWORD
*/}}
{{- define "llm-proxy.redisPasswordSecretName" -}}
{{- if .Values.redis.external.password.existingSecret.name }}
{{- .Values.redis.external.password.existingSecret.name }}
{{- end }}
{{- end }}

{{/*
Get the key within the secret for REDIS_PASSWORD
*/}}
{{- define "llm-proxy.redisPasswordSecretKey" -}}
{{- .Values.redis.external.password.existingSecret.key | default "REDIS_PASSWORD" }}
{{- end }}

{{/*
Get the name of the secret containing ENCRYPTION_KEY
*/}}
{{- define "llm-proxy.encryptionKeySecretName" -}}
{{- if and .Values.secrets.create .Values.secrets.data.encryptionKey }}
{{- include "llm-proxy.fullname" . }}
{{- else if .Values.secrets.encryptionKey.existingSecret.name }}
{{- .Values.secrets.encryptionKey.existingSecret.name }}
{{- end }}
{{- end }}

{{/*
Get the key within the secret for ENCRYPTION_KEY
*/}}
{{- define "llm-proxy.encryptionKeySecretKey" -}}
{{- if .Values.secrets.create }}
{{- printf "ENCRYPTION_KEY" }}
{{- else }}
{{- .Values.secrets.encryptionKey.existingSecret.key | default "ENCRYPTION_KEY" }}
{{- end }}
{{- end }}

{{/*
Validate SQLite configuration
*/}}
{{- define "llm-proxy.validateSqliteConfig" -}}
{{- $dbDriver := .Values.env.DB_DRIVER | default "sqlite" }}
{{- $maxReplicas := .Values.replicaCount | default 1 }}
{{- if .Values.autoscaling.enabled }}
  {{- $maxReplicas = .Values.autoscaling.maxReplicas | default 1 }}
{{- end }}
{{- if and (eq $dbDriver "sqlite") (gt (int $maxReplicas) 1) }}
  {{- fail (printf "Configuration error: DB_DRIVER is 'sqlite' but the deployment can scale to %v replicas. SQLite is not supported for multi-pod deployments. Set env.DB_DRIVER to 'mysql' or 'postgres' and configure secrets.databaseUrl / mysql.enabled / postgresql.enabled." $maxReplicas) }}
{{- end }}
{{- end }}

{{/*
Validate ENCRYPTION_KEY configuration
*/}}
{{- define "llm-proxy.validateEncryptionKeyConfig" -}}
{{- if .Values.secrets.encryptionKey.required }}
  {{- $hasKey := or (and .Values.secrets.create .Values.secrets.data.encryptionKey) .Values.secrets.encryptionKey.existingSecret.name }}
  {{- if not $hasKey }}
    {{- fail "Configuration error: ENCRYPTION_KEY is required but not configured. Provide secrets.encryptionKey.existingSecret.name (recommended) or set secrets.create=true and secrets.data.encryptionKey (development only)." }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
Get PostgreSQL hostname
*/}}
{{- define "llm-proxy.postgresql.host" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "%s-postgresql" (include "llm-proxy.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Get PostgreSQL port
Returns the PostgreSQL service port from the Bitnami subchart configuration.
Note: The Bitnami PostgreSQL chart sets postgresql.primary.service.ports.postgresql
automatically when deployed. This value is not defined in llm-proxy's values.yaml
but is inherited from the Bitnami subchart's defaults.
*/}}
{{- define "llm-proxy.postgresql.port" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.primary.service.ports.postgresql | default "5432" }}
{{- end }}
{{- end }}

{{/*
Get PostgreSQL database name
*/}}
{{- define "llm-proxy.postgresql.database" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.database | default "llmproxy" }}
{{- end }}
{{- end }}

{{/*
Get PostgreSQL username
*/}}
{{- define "llm-proxy.postgresql.username" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.username | default "llmproxy" }}
{{- end }}
{{- end }}

{{/*
Get PostgreSQL password secret name
*/}}
{{- define "llm-proxy.postgresql.secretName" -}}
{{- if .Values.postgresql.enabled }}
{{- if .Values.postgresql.auth.existingSecret }}
{{- .Values.postgresql.auth.existingSecret }}
{{- else }}
{{- printf "%s-postgresql" (include "llm-proxy.fullname" .) }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Get PostgreSQL password secret key
*/}}
{{- define "llm-proxy.postgresql.secretKey" -}}
{{- if .Values.postgresql.enabled }}
{{- if .Values.postgresql.auth.existingSecret }}
{{- .Values.postgresql.auth.secretKeys.userPasswordKey | default "password" }}
{{- else }}
{{- printf "password" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Construct PostgreSQL connection URL for in-cluster PostgreSQL
*/}}
{{- define "llm-proxy.postgresql.url" -}}
{{- if .Values.postgresql.enabled }}
{{- $host := include "llm-proxy.postgresql.host" . }}
{{- $port := include "llm-proxy.postgresql.port" . }}
{{- $database := include "llm-proxy.postgresql.database" . }}
{{- $username := include "llm-proxy.postgresql.username" . }}
{{- $sslMode := .Values.postgresql.sslMode | default "require" }}
{{- printf "postgres://%s:$(PGPASSWORD)@%s:%s/%s?sslmode=%s" $username $host $port $database $sslMode }}
{{- end }}
{{- end }}

{{/*
Validate PostgreSQL configuration
*/}}
{{- define "llm-proxy.validatePostgresConfig" -}}
{{- $dbDriver := .Values.env.DB_DRIVER | default "sqlite" }}
{{- if eq $dbDriver "postgres" }}
  {{- $hasInCluster := .Values.postgresql.enabled }}
  {{- $hasExternal := or (and .Values.secrets.create .Values.secrets.data.databaseUrl) .Values.secrets.databaseUrl.existingSecret.name }}
  {{- if and $hasInCluster $hasExternal }}
    {{- fail "Configuration error: Cannot use both in-cluster PostgreSQL (postgresql.enabled=true) and external PostgreSQL (secrets.databaseUrl) at the same time. Please choose one approach." }}
  {{- end }}
  {{- if not (or $hasInCluster $hasExternal) }}
    {{- fail (printf "Configuration error: DB_DRIVER is set to 'postgres' but no database configuration found. Please either:\n  1. Enable in-cluster PostgreSQL with postgresql.enabled=true (development/testing only)\n  2. Configure external PostgreSQL with secrets.databaseUrl.existingSecret.name (recommended for production)\n  3. Change DB_DRIVER to 'sqlite' for single-instance deployments") }}
  {{- end }}
  {{- if and $hasInCluster (not .Values.postgresql.auth.password) (not .Values.postgresql.auth.existingSecret) }}
    {{- fail "Configuration error: postgresql.enabled=true but no password configured. Please set postgresql.auth.password or postgresql.auth.existingSecret" }}
  {{- end }}
{{- end }}
{{- if and .Values.postgresql.enabled (ne $dbDriver "postgres") }}
  {{- fail (printf "Configuration error: postgresql.enabled=true but DB_DRIVER is set to '%s'. When using in-cluster PostgreSQL, DB_DRIVER must be set to 'postgres'." $dbDriver) }}
{{- end }}
{{- end }}

{{/*
Validate Redis configuration
*/}}
{{- define "llm-proxy.validateRedisConfig" -}}
{{- $eventBus := .Values.env.LLM_PROXY_EVENT_BUS | default "" }}
{{- if or (eq $eventBus "redis") (eq $eventBus "redis-streams") }}
  {{- if not .Values.redis.external.addr }}
    {{- fail (printf "Configuration error: LLM_PROXY_EVENT_BUS is set to '%s' but redis.external.addr is empty. Please set redis.external.addr to your Redis server address (e.g., 'redis.example.com:6379') or change LLM_PROXY_EVENT_BUS to 'in-memory' for single-instance deployments." $eventBus) }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
Dispatcher fullname
*/}}
{{- define "llm-proxy.dispatcher.fullname" -}}
{{- printf "%s-dispatcher" (include "llm-proxy.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Admin UI fullname
*/}}
{{- define "llm-proxy.admin.fullname" -}}
{{- printf "%s-admin" (include "llm-proxy.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Admin UI selector labels
*/}}
{{- define "llm-proxy.admin.selectorLabels" -}}
app.kubernetes.io/name: {{ include "llm-proxy.name" . }}-admin
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: admin
{{- end }}

{{/*
Admin UI labels
*/}}
{{- define "llm-proxy.admin.labels" -}}
helm.sh/chart: {{ include "llm-proxy.chart" . }}
{{ include "llm-proxy.admin.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Dispatcher selector labels
*/}}
{{- define "llm-proxy.dispatcher.selectorLabels" -}}
app.kubernetes.io/name: {{ include "llm-proxy.name" . }}-dispatcher
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: dispatcher
{{- end }}

{{/*
Dispatcher labels
*/}}
{{- define "llm-proxy.dispatcher.labels" -}}
helm.sh/chart: {{ include "llm-proxy.chart" . }}
{{ include "llm-proxy.dispatcher.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Get the name of the secret containing DISPATCHER_API_KEY
*/}}
{{- define "llm-proxy.dispatcher.apiKeySecretName" -}}
{{- if .Values.dispatcher.apiKey.existingSecret.name }}
{{- .Values.dispatcher.apiKey.existingSecret.name }}
{{- end }}
{{- end }}

{{/*
Get the key within the secret for DISPATCHER_API_KEY
*/}}
{{- define "llm-proxy.dispatcher.apiKeySecretKey" -}}
{{- .Values.dispatcher.apiKey.existingSecret.key | default "DISPATCHER_API_KEY" }}
{{- end }}

{{/*
Get dispatcher service endpoint with defaults
*/}}
{{- define "llm-proxy.dispatcher.endpoint" -}}
{{- if .Values.dispatcher.endpoint }}
{{- .Values.dispatcher.endpoint }}
{{- else if eq .Values.dispatcher.service "file" }}
{{- printf "/app/data/events.jsonl" }}
{{- else if eq .Values.dispatcher.service "lunary" }}
{{- printf "https://api.lunary.ai/v1/runs/ingest" }}
{{- else if eq .Values.dispatcher.service "helicone" }}
{{- printf "https://api.worker.helicone.ai/custom/v1/log" }}
{{- end }}
{{- end }}

{{/*
Validate dispatcher configuration
*/}}
{{- define "llm-proxy.validateDispatcherConfig" -}}
{{- if .Values.dispatcher.enabled }}
  {{- $eventBus := .Values.env.LLM_PROXY_EVENT_BUS | default "in-memory" }}
  {{- if and (ne $eventBus "redis") (ne $eventBus "redis-streams") }}
    {{- fail (printf "Configuration error: Dispatcher requires LLM_PROXY_EVENT_BUS to be 'redis' or 'redis-streams', but it is set to '%s'. Set LLM_PROXY_EVENT_BUS to a supported durable event bus type and configure redis.external.addr." $eventBus) }}
  {{- end }}
  {{- if and (or (eq $eventBus "redis") (eq $eventBus "redis-streams")) (not .Values.redis.external.addr) }}
    {{- fail (printf "Configuration error: Dispatcher is enabled with LLM_PROXY_EVENT_BUS='%s' but redis.external.addr is empty. Please set redis.external.addr to your Redis server address." $eventBus) }}
  {{- end }}
  {{- if and (ne .Values.dispatcher.service "file") (not .Values.dispatcher.apiKey.existingSecret.name) }}
    {{- fail (printf "Configuration error: Dispatcher service '%s' requires an API key via existingSecret. Please set dispatcher.apiKey.existingSecret.name" .Values.dispatcher.service) }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
Validate autoscaling configuration
*/}}
{{- define "llm-proxy.validateAutoscalingConfig" -}}
{{- if .Values.autoscaling.enabled }}
  {{- if and (not .Values.autoscaling.targetCPUUtilizationPercentage) (not .Values.autoscaling.targetMemoryUtilizationPercentage) }}
    {{- fail "Configuration error: autoscaling.enabled is true but no target metrics are configured. Please set at least one of autoscaling.targetCPUUtilizationPercentage or autoscaling.targetMemoryUtilizationPercentage." }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
MySQL helper functions
*/}}

{{/*
Get MySQL hostname
*/}}
{{- define "llm-proxy.mysql.host" -}}
{{- if .Values.mysql.enabled }}
{{- printf "%s-mysql" (include "llm-proxy.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Get MySQL port
*/}}
{{- define "llm-proxy.mysql.port" -}}
{{- if .Values.mysql.enabled }}
{{- printf "3306" }}
{{- end }}
{{- end }}

{{/*
Get MySQL database name
*/}}
{{- define "llm-proxy.mysql.database" -}}
{{- if .Values.mysql.enabled }}
{{- .Values.mysql.auth.database | default "llmproxy" }}
{{- end }}
{{- end }}

{{/*
Get MySQL username
*/}}
{{- define "llm-proxy.mysql.username" -}}
{{- if .Values.mysql.enabled }}
{{- .Values.mysql.auth.username | default "llmproxy" }}
{{- end }}
{{- end }}

{{/*
Get MySQL password secret name
*/}}
{{- define "llm-proxy.mysql.secretName" -}}
{{- if .Values.mysql.enabled }}
{{- if .Values.mysql.auth.existingSecret }}
{{- .Values.mysql.auth.existingSecret }}
{{- else }}
{{- printf "%s-mysql" (include "llm-proxy.fullname" .) }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Get MySQL password secret key
*/}}
{{- define "llm-proxy.mysql.secretKey" -}}
{{- if .Values.mysql.enabled }}
{{- if .Values.mysql.auth.existingSecret }}
{{- .Values.mysql.auth.secretKeys.userPasswordKey | default "mysql-password" }}
{{- else }}
{{- printf "mysql-password" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Get MySQL root password secret key
*/}}
{{- define "llm-proxy.mysql.rootSecretKey" -}}
{{- if .Values.mysql.enabled }}
{{- if .Values.mysql.auth.existingSecret }}
{{- .Values.mysql.auth.secretKeys.rootPasswordKey | default "mysql-root-password" }}
{{- else }}
{{- printf "mysql-root-password" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Construct MySQL connection URL for in-cluster MySQL
*/}}
{{- define "llm-proxy.mysql.url" -}}
{{- if .Values.mysql.enabled }}
{{- $host := include "llm-proxy.mysql.host" . }}
{{- $port := include "llm-proxy.mysql.port" . }}
{{- $database := include "llm-proxy.mysql.database" . }}
{{- $username := include "llm-proxy.mysql.username" . }}
{{- $tls := "" }}
{{- if .Values.mysql.tls.enabled }}
{{- if .Values.mysql.tls.skipVerify }}
{{- $tls = "&tls=skip-verify" }}
{{- else }}
{{- $tls = "&tls=true" }}
{{- end }}
{{- end }}
{{- printf "%s:$(MYSQL_PASSWORD)@tcp(%s:%s)/%s?parseTime=true%s" $username $host $port $database $tls }}
{{- end }}
{{- end }}

{{/*
Validate MySQL configuration
*/}}
{{- define "llm-proxy.validateMysqlConfig" -}}
{{- $dbDriver := .Values.env.DB_DRIVER | default "sqlite" }}
{{- if eq $dbDriver "mysql" }}
  {{- $hasInCluster := .Values.mysql.enabled }}
  {{- $hasExternal := or (and .Values.secrets.create .Values.secrets.data.databaseUrl) .Values.secrets.databaseUrl.existingSecret.name }}
  {{- if and $hasInCluster $hasExternal }}
    {{- fail "Configuration error: Cannot use both in-cluster MySQL (mysql.enabled=true) and external MySQL (secrets.databaseUrl) at the same time. Please choose one approach." }}
  {{- end }}
  {{- if not (or $hasInCluster $hasExternal) }}
    {{- fail (printf "Configuration error: DB_DRIVER is set to 'mysql' but no database configuration found. Please either:\n  1. Enable in-cluster MySQL with mysql.enabled=true (development/testing only)\n  2. Configure external MySQL with secrets.databaseUrl.existingSecret.name (recommended for production)\n  3. Change DB_DRIVER to 'sqlite' for single-instance deployments") }}
  {{- end }}
  {{- if and $hasInCluster (not .Values.mysql.auth.password) (not .Values.mysql.auth.existingSecret) }}
    {{- fail "Configuration error: mysql.enabled=true but no password configured. Please set mysql.auth.password and mysql.auth.rootPassword or mysql.auth.existingSecret" }}
  {{- end }}
  {{- if and $hasInCluster (not .Values.mysql.auth.rootPassword) (not .Values.mysql.auth.existingSecret) }}
    {{- fail "Configuration error: mysql.enabled=true but no root password configured. Please set mysql.auth.rootPassword or mysql.auth.existingSecret" }}
  {{- end }}
{{- end }}
{{- if and .Values.mysql.enabled (ne $dbDriver "mysql") }}
  {{- fail (printf "Configuration error: mysql.enabled=true but DB_DRIVER is set to '%s'. When using in-cluster MySQL, DB_DRIVER must be set to 'mysql'." $dbDriver) }}
{{- end }}
{{- end }}
