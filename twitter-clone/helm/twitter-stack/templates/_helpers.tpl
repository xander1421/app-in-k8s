{{/*
Expand the name of the chart.
*/}}
{{- define "twitter-stack.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "twitter-stack.fullname" -}}
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
{{- define "twitter-stack.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "twitter-stack.labels" -}}
helm.sh/chart: {{ include "twitter-stack.chart" . }}
{{ include "twitter-stack.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "twitter-stack.selectorLabels" -}}
app.kubernetes.io/name: {{ include "twitter-stack.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
PostgreSQL connection host (CloudNativePG uses -rw suffix for read-write service)
*/}}
{{- define "twitter-stack.postgresql.host" -}}
{{- printf "%s-postgresql-rw.%s.svc.cluster.local" .Release.Name .Values.namespace }}
{{- end }}

{{/*
PostgreSQL connection URL
*/}}
{{- define "twitter-stack.postgresql.url" -}}
{{- printf "postgresql://app@%s:5432/%s" (include "twitter-stack.postgresql.host" .) .Values.postgresql.database }}
{{- end }}

{{/*
Redis Sentinel host (Spotahome operator uses rfs- prefix)
*/}}
{{- define "twitter-stack.redis.sentinelHost" -}}
{{- printf "rfs-%s-redis.%s.svc.cluster.local" .Release.Name .Values.namespace }}
{{- end }}

{{/*
Redis master host
*/}}
{{- define "twitter-stack.redis.host" -}}
{{- printf "rfr-%s-redis.%s.svc.cluster.local" .Release.Name .Values.namespace }}
{{- end }}

{{/*
Redis URL (for Sentinel mode)
*/}}
{{- define "twitter-stack.redis.url" -}}
{{- printf "redis://%s:26379/1" (include "twitter-stack.redis.sentinelHost" .) }}
{{- end }}

{{/*
Elasticsearch host (ECK uses -es-http suffix)
*/}}
{{- define "twitter-stack.elasticsearch.host" -}}
{{- printf "%s-elasticsearch-es-http.%s.svc.cluster.local" .Release.Name .Values.namespace }}
{{- end }}

{{/*
Elasticsearch URL
*/}}
{{- define "twitter-stack.elasticsearch.url" -}}
{{- printf "http://%s:9200" (include "twitter-stack.elasticsearch.host" .) }}
{{- end }}

{{/*
RabbitMQ host
*/}}
{{- define "twitter-stack.rabbitmq.host" -}}
{{- printf "%s-rabbitmq.%s.svc.cluster.local" .Release.Name .Values.namespace }}
{{- end }}

{{/*
RabbitMQ URL
*/}}
{{- define "twitter-stack.rabbitmq.url" -}}
{{- printf "amqp://%s:5672" (include "twitter-stack.rabbitmq.host" .) }}
{{- end }}

{{/*
Rails app service name
*/}}
{{- define "twitter-stack.goapp.serviceName" -}}
{{- printf "%s-goapp" .Release.Name }}
{{- end }}
