{{/*
NetCore-Go Helm Chart Helper Templates
Author: NetCore-Go Team
Created: 2024
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "netcore-go.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "netcore-go.fullname" -}}
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
{{- define "netcore-go.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "netcore-go.labels" -}}
helm.sh/chart: {{ include "netcore-go.chart" . }}
{{ include "netcore-go.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: netcore-go
app.kubernetes.io/component: server
{{- end }}

{{/*
Selector labels
*/}}
{{- define "netcore-go.selectorLabels" -}}
app.kubernetes.io/name: {{ include "netcore-go.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "netcore-go.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "netcore-go.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the docker image name
*/}}
{{- define "netcore-go.image" -}}
{{- $registry := .Values.image.registry -}}
{{- $repository := .Values.image.repository -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion -}}
{{- $digest := .Values.image.digest -}}
{{- if .Values.global.imageRegistry }}
{{- $registry = .Values.global.imageRegistry -}}
{{- end }}
{{- if $registry }}
{{- printf "%s/%s" $registry $repository -}}
{{- else }}
{{- $repository -}}
{{- end }}
{{- if $digest }}
{{- printf "@%s" $digest -}}
{{- else }}
{{- printf ":%s" $tag -}}
{{- end }}
{{- end }}

{{/*
Create the name of the config map
*/}}
{{- define "netcore-go.configMapName" -}}
{{- printf "%s-config" (include "netcore-go.fullname" .) }}
{{- end }}

{{/*
Create the name of the secret
*/}}
{{- define "netcore-go.secretName" -}}
{{- printf "%s-secrets" (include "netcore-go.fullname" .) }}
{{- end }}

{{/*
Create the name of the TLS secret
*/}}
{{- define "netcore-go.tlsSecretName" -}}
{{- if .Values.tls.secretName }}
{{- .Values.tls.secretName }}
{{- else }}
{{- printf "%s-tls" (include "netcore-go.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Create the name of the PVC
*/}}
{{- define "netcore-go.pvcName" -}}
{{- printf "%s-data" (include "netcore-go.fullname" .) }}
{{- end }}

{{/*
Create the name of the service monitor
*/}}
{{- define "netcore-go.serviceMonitorName" -}}
{{- printf "%s-metrics" (include "netcore-go.fullname" .) }}
{{- end }}

{{/*
Create the name of the prometheus rule
*/}}
{{- define "netcore-go.prometheusRuleName" -}}
{{- printf "%s-alerts" (include "netcore-go.fullname" .) }}
{{- end }}

{{/*
Create the name of the network policy
*/}}
{{- define "netcore-go.networkPolicyName" -}}
{{- printf "%s-netpol" (include "netcore-go.fullname" .) }}
{{- end }}

{{/*
Create the name of the pod disruption budget
*/}}
{{- define "netcore-go.pdbName" -}}
{{- printf "%s-pdb" (include "netcore-go.fullname" .) }}
{{- end }}

{{/*
Create the name of the horizontal pod autoscaler
*/}}
{{- define "netcore-go.hpaName" -}}
{{- printf "%s-hpa" (include "netcore-go.fullname" .) }}
{{- end }}

{{/*
Create the name of the vertical pod autoscaler
*/}}
{{- define "netcore-go.vpaName" -}}
{{- printf "%s-vpa" (include "netcore-go.fullname" .) }}
{{- end }}

{{/*
Generate certificates for TLS
*/}}
{{- define "netcore-go.gen-certs" -}}
{{- $altNames := list ( printf "%s.%s" (include "netcore-go.fullname" .) .Release.Namespace ) ( printf "%s.%s.svc" (include "netcore-go.fullname" .) .Release.Namespace ) -}}
{{- $ca := genCA "netcore-go-ca" 365 -}}
{{- $cert := genSignedCert ( include "netcore-go.fullname" . ) nil $altNames 365 $ca -}}
tls.crt: {{ $cert.Cert | b64enc }}
tls.key: {{ $cert.Key | b64enc }}
ca.crt: {{ $ca.Cert | b64enc }}
{{- end }}

{{/*
Validate configuration
*/}}
{{- define "netcore-go.validateConfig" -}}
{{- if and .Values.postgresql.enabled .Values.externalDatabase.enabled }}
{{- fail "Cannot enable both postgresql and externalDatabase" }}
{{- end }}
{{- if and .Values.redis.enabled .Values.externalRedis.enabled }}
{{- fail "Cannot enable both redis and externalRedis" }}
{{- end }}
{{- if and (not .Values.postgresql.enabled) (not .Values.externalDatabase.enabled) }}
{{- fail "Either postgresql or externalDatabase must be enabled" }}
{{- end }}
{{- if lt (.Values.replicaCount | int) 1 }}
{{- fail "replicaCount must be at least 1" }}
{{- end }}
{{- if and .Values.autoscaling.enabled (lt (.Values.autoscaling.minReplicas | int) 1) }}
{{- fail "autoscaling.minReplicas must be at least 1" }}
{{- end }}
{{- if and .Values.autoscaling.enabled (gt (.Values.autoscaling.minReplicas | int) (.Values.autoscaling.maxReplicas | int)) }}
{{- fail "autoscaling.minReplicas cannot be greater than autoscaling.maxReplicas" }}
{{- end }}
{{- end }}

{{/*
Get the database host
*/}}
{{- define "netcore-go.databaseHost" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "%s-postgresql" .Release.Name }}
{{- else if .Values.externalDatabase.enabled }}
{{- .Values.externalDatabase.host }}
{{- end }}
{{- end }}

{{/*
Get the database port
*/}}
{{- define "netcore-go.databasePort" -}}
{{- if .Values.postgresql.enabled }}
{{- print "5432" }}
{{- else if .Values.externalDatabase.enabled }}
{{- .Values.externalDatabase.port | toString }}
{{- end }}
{{- end }}

{{/*
Get the database name
*/}}
{{- define "netcore-go.databaseName" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.global.postgresql.auth.database | default .Values.postgresql.auth.database }}
{{- else if .Values.externalDatabase.enabled }}
{{- .Values.externalDatabase.database }}
{{- end }}
{{- end }}

{{/*
Get the database username
*/}}
{{- define "netcore-go.databaseUsername" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.global.postgresql.auth.username | default .Values.postgresql.auth.username }}
{{- else if .Values.externalDatabase.enabled }}
{{- .Values.externalDatabase.username }}
{{- end }}
{{- end }}

{{/*
Get the database password secret name
*/}}
{{- define "netcore-go.databaseSecretName" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "%s-postgresql" .Release.Name }}
{{- else }}
{{- include "netcore-go.secretName" . }}
{{- end }}
{{- end }}

{{/*
Get the database password secret key
*/}}
{{- define "netcore-go.databaseSecretKey" -}}
{{- if .Values.postgresql.enabled }}
{{- print "password" }}
{{- else }}
{{- print "database-password" }}
{{- end }}
{{- end }}

{{/*
Get the Redis host
*/}}
{{- define "netcore-go.redisHost" -}}
{{- if .Values.redis.enabled }}
{{- printf "%s-redis-master" .Release.Name }}
{{- else if .Values.externalRedis.enabled }}
{{- .Values.externalRedis.host }}
{{- end }}
{{- end }}

{{/*
Get the Redis port
*/}}
{{- define "netcore-go.redisPort" -}}
{{- if .Values.redis.enabled }}
{{- print "6379" }}
{{- else if .Values.externalRedis.enabled }}
{{- .Values.externalRedis.port | toString }}
{{- end }}
{{- end }}

{{/*
Get the Redis password secret name
*/}}
{{- define "netcore-go.redisSecretName" -}}
{{- if .Values.redis.enabled }}
{{- printf "%s-redis" .Release.Name }}
{{- else }}
{{- include "netcore-go.secretName" . }}
{{- end }}
{{- end }}

{{/*
Get the Redis password secret key
*/}}
{{- define "netcore-go.redisSecretKey" -}}
{{- if .Values.redis.enabled }}
{{- print "redis-password" }}
{{- else }}
{{- print "redis-password" }}
{{- end }}
{{- end }}

{{/*
Generate environment variables for database connection
*/}}
{{- define "netcore-go.databaseEnv" -}}
- name: DATABASE_HOST
  value: {{ include "netcore-go.databaseHost" . | quote }}
- name: DATABASE_PORT
  value: {{ include "netcore-go.databasePort" . | quote }}
- name: DATABASE_NAME
  value: {{ include "netcore-go.databaseName" . | quote }}
- name: DATABASE_USERNAME
  value: {{ include "netcore-go.databaseUsername" . | quote }}
- name: DATABASE_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "netcore-go.databaseSecretName" . }}
      key: {{ include "netcore-go.databaseSecretKey" . }}
{{- if .Values.externalDatabase.sslMode }}
- name: DATABASE_SSL_MODE
  value: {{ .Values.externalDatabase.sslMode | quote }}
{{- end }}
{{- end }}

{{/*
Generate environment variables for Redis connection
*/}}
{{- define "netcore-go.redisEnv" -}}
{{- if or .Values.redis.enabled .Values.externalRedis.enabled }}
- name: REDIS_HOST
  value: {{ include "netcore-go.redisHost" . | quote }}
- name: REDIS_PORT
  value: {{ include "netcore-go.redisPort" . | quote }}
{{- if or .Values.redis.auth.enabled .Values.externalRedis.password }}
- name: REDIS_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "netcore-go.redisSecretName" . }}
      key: {{ include "netcore-go.redisSecretKey" . }}
{{- end }}
{{- if .Values.externalRedis.database }}
- name: REDIS_DATABASE
  value: {{ .Values.externalRedis.database | toString | quote }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Generate resource limits and requests
*/}}
{{- define "netcore-go.resources" -}}
{{- if .Values.resources }}
resources:
  {{- if .Values.resources.limits }}
  limits:
    {{- range $key, $value := .Values.resources.limits }}
    {{ $key }}: {{ $value | quote }}
    {{- end }}
  {{- end }}
  {{- if .Values.resources.requests }}
  requests:
    {{- range $key, $value := .Values.resources.requests }}
    {{ $key }}: {{ $value | quote }}
    {{- end }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
Generate pod security context
*/}}
{{- define "netcore-go.podSecurityContext" -}}
{{- if .Values.podSecurityContext }}
securityContext:
  {{- toYaml .Values.podSecurityContext | nindent 2 }}
{{- end }}
{{- end }}

{{/*
Generate container security context
*/}}
{{- define "netcore-go.securityContext" -}}
{{- if .Values.securityContext }}
securityContext:
  {{- toYaml .Values.securityContext | nindent 2 }}
{{- end }}
{{- end }}

{{/*
Generate image pull secrets
*/}}
{{- define "netcore-go.imagePullSecrets" -}}
{{- $pullSecrets := list }}
{{- if .Values.global.imagePullSecrets }}
  {{- $pullSecrets = .Values.global.imagePullSecrets }}
{{- else if .Values.image.pullSecrets }}
  {{- $pullSecrets = .Values.image.pullSecrets }}
{{- end }}
{{- if $pullSecrets }}
imagePullSecrets:
{{- range $pullSecrets }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end }}