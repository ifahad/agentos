{{/*
Expand the name of the chart.
*/}}
{{- define "agentos.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{/*
Fully qualified app name (release-name aware, truncated to DNS length).
*/}}
{{- define "agentos.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end }}

{{/*
Chart name and version label.
*/}}
{{- define "agentos.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "agentos.labels" -}}
helm.sh/chart: {{ include "agentos.chart" . }}
{{ include "agentos.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels (shared across all components; each component adds
app.kubernetes.io/component to these).
*/}}
{{- define "agentos.selectorLabels" -}}
app.kubernetes.io/name: {{ include "agentos.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Name of the chart-managed Secret.
*/}}
{{- define "agentos.secretName" -}}
{{ include "agentos.fullname" . }}-secrets
{{- end }}

{{/*
Gateway/checkpoint database URL: bundled Postgres or externalDatabaseUrl.
*/}}
{{- define "agentos.databaseUrl" -}}
{{- if .Values.postgres.enabled -}}
postgres://{{ .Values.postgres.auth.username }}:{{ .Values.postgres.auth.password }}@{{ include "agentos.fullname" . }}-postgres:5432/{{ .Values.postgres.auth.database }}
{{- else -}}
{{- required "externalDatabaseUrl is required when postgres.enabled=false" .Values.externalDatabaseUrl -}}
{{- end -}}
{{- end }}

{{/*
SQL connector (legacy ERP) database URL.
*/}}
{{- define "agentos.connectorDatabaseUrl" -}}
{{- if .Values.sqlConnector.databaseUrl -}}
{{- .Values.sqlConnector.databaseUrl -}}
{{- else if .Values.postgres.enabled -}}
postgres://erp_reader:erp_reader@{{ include "agentos.fullname" . }}-postgres:5432/legacy_erp
{{- else -}}
{{- fail "sqlConnector.databaseUrl is required when sqlConnector.enabled=true and postgres.enabled=false" -}}
{{- end -}}
{{- end }}

{{/*
In-cluster gateway URL.
*/}}
{{- define "agentos.gatewayUrl" -}}
http://{{ include "agentos.fullname" . }}-gateway:{{ .Values.gateway.service.port }}
{{- end }}

{{/*
AGENTOS_MCP_SERVERS for the runtime, composed from the enabled connectors
plus runtime.extraMcpServers.
*/}}
{{- define "agentos.mcpServers" -}}
{{- $servers := list -}}
{{- if .Values.sqlConnector.enabled -}}
{{- $servers = append $servers (printf "http://%s-sql-connector:%v/mcp" (include "agentos.fullname" .) .Values.sqlConnector.service.port) -}}
{{- end -}}
{{- if .Values.restConnector.enabled -}}
{{- $servers = append $servers (printf "http://%s-rest-connector:%v/mcp" (include "agentos.fullname" .) .Values.restConnector.service.port) -}}
{{- end -}}
{{- if .Values.soapConnector.enabled -}}
{{- $servers = append $servers (printf "http://%s-soap-connector:%v/mcp" (include "agentos.fullname" .) .Values.soapConnector.service.port) -}}
{{- end -}}
{{- if .Values.browserConnector.enabled -}}
{{- $servers = append $servers (printf "http://%s-browser-connector:%v/mcp" (include "agentos.fullname" .) .Values.browserConnector.service.port) -}}
{{- end -}}
{{- $servers = concat $servers .Values.runtime.extraMcpServers -}}
{{- join "," $servers -}}
{{- end }}

{{/*
Provider API key env entries for the gateway, sourced from an existing
Secret (both keys optional so either provider works alone).
*/}}
{{- define "agentos.providerKeyEnv" -}}
{{- if .Values.providerKeys.existingSecret }}
- name: AGENTOS_ANTHROPIC_API_KEY
  valueFrom:
    secretKeyRef:
      name: {{ .Values.providerKeys.existingSecret }}
      key: AGENTOS_ANTHROPIC_API_KEY
      optional: true
- name: AGENTOS_OPENAI_API_KEY
  valueFrom:
    secretKeyRef:
      name: {{ .Values.providerKeys.existingSecret }}
      key: AGENTOS_OPENAI_API_KEY
      optional: true
{{- end }}
{{- end }}
