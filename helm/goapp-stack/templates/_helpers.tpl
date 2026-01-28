{{/*
Expand the name of the chart.
*/}}
{{- define "goapp-stack.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "goapp-stack.fullname" -}}
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
{{- define "goapp-stack.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "goapp-stack.labels" -}}
helm.sh/chart: {{ include "goapp-stack.chart" . }}
{{ include "goapp-stack.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "goapp-stack.selectorLabels" -}}
app.kubernetes.io/name: {{ include "goapp-stack.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
PostgreSQL connection host (CloudNativePG uses -rw suffix for read-write service)
*/}}
{{- define "goapp-stack.postgresql.host" -}}
{{- printf "%s-postgresql-rw.%s.svc.cluster.local" .Release.Name .Values.namespace }}
{{- end }}

{{/*
PostgreSQL connection URL
*/}}
{{- define "goapp-stack.postgresql.url" -}}
{{- printf "postgresql://app@%s:5432/%s" (include "goapp-stack.postgresql.host" .) .Values.postgresql.database }}
{{- end }}

{{/*
Redis Sentinel host (Spotahome operator uses rfs- prefix)
*/}}
{{- define "goapp-stack.redis.sentinelHost" -}}
{{- printf "rfs-%s-redis.%s.svc.cluster.local" .Release.Name .Values.namespace }}
{{- end }}

{{/*
Redis master host
*/}}
{{- define "goapp-stack.redis.host" -}}
{{- printf "rfr-%s-redis.%s.svc.cluster.local" .Release.Name .Values.namespace }}
{{- end }}

{{/*
Redis URL (for Sentinel mode)
*/}}
{{- define "goapp-stack.redis.url" -}}
{{- printf "redis://%s:26379/1" (include "goapp-stack.redis.sentinelHost" .) }}
{{- end }}

{{/*
Elasticsearch host (ECK uses -es-http suffix)
*/}}
{{- define "goapp-stack.elasticsearch.host" -}}
{{- printf "%s-elasticsearch-es-http.%s.svc.cluster.local" .Release.Name .Values.namespace }}
{{- end }}

{{/*
Elasticsearch URL
*/}}
{{- define "goapp-stack.elasticsearch.url" -}}
{{- printf "http://%s:9200" (include "goapp-stack.elasticsearch.host" .) }}
{{- end }}

{{/*
RabbitMQ host
*/}}
{{- define "goapp-stack.rabbitmq.host" -}}
{{- printf "%s-rabbitmq.%s.svc.cluster.local" .Release.Name .Values.namespace }}
{{- end }}

{{/*
RabbitMQ URL
*/}}
{{- define "goapp-stack.rabbitmq.url" -}}
{{- printf "amqp://%s:5672" (include "goapp-stack.rabbitmq.host" .) }}
{{- end }}

{{/*
Rails app service name
*/}}
{{- define "goapp-stack.goapp.serviceName" -}}
{{- printf "%s-goapp" .Release.Name }}
{{- end }}
