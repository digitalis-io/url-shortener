{{/*
Expand the name of the chart.
*/}}
{{- define "url-shortener.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this.
*/}}
{{- define "url-shortener.fullname" -}}
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
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "url-shortener.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "url-shortener.labels" -}}
helm.sh/chart: {{ include "url-shortener.chart" . }}
{{ include "url-shortener.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "url-shortener.selectorLabels" -}}
app.kubernetes.io/name: {{ include "url-shortener.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Create the name of the service account to use.
*/}}
{{- define "url-shortener.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "url-shortener.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Secret name resolvers. When a group sets existingSecret, the deployment references
that external Secret; otherwise it references the Secret rendered by this chart.
*/}}
{{- define "url-shortener.sessionSecretName" -}}
{{- .Values.session.existingSecret | default (include "url-shortener.fullname" .) -}}
{{- end -}}

{{- define "url-shortener.cassandraSecretName" -}}
{{- .Values.cassandra.existingSecret | default (include "url-shortener.fullname" .) -}}
{{- end -}}

{{- define "url-shortener.samlSecretName" -}}
{{- .Values.saml.existingSecret | default (include "url-shortener.fullname" .) -}}
{{- end -}}

{{/*
Non-sensitive environment variables rendered into the ConfigMap. Defined here so
both configmap.yaml and the deployment's checksum annotation share one source.
*/}}
{{- define "url-shortener.configmapData" -}}
HTTP_ADDR: {{ .Values.app.httpAddr | quote }}
PUBLIC_BASE_URL: {{ .Values.app.publicBaseURL | quote }}
ADMIN_BASE_URL: {{ .Values.app.adminBaseURL | quote }}
CODE_LENGTH: {{ .Values.app.codeLength | quote }}
CASSANDRA_HOSTS: {{ .Values.cassandra.hosts | quote }}
CASSANDRA_KEYSPACE: {{ .Values.cassandra.keyspace | quote }}
CASSANDRA_SSL_ENABLED: {{ .Values.cassandra.sslEnabled | quote }}
CASSANDRA_CONSISTENCY: {{ .Values.cassandra.consistency | quote }}
CASSANDRA_LOCAL_DC: {{ .Values.cassandra.localDC | quote }}
AUTH_DEV_BYPASS: {{ .Values.auth.devBypass | quote }}
{{- if .Values.cassandra.username }}
CASSANDRA_USERNAME: {{ .Values.cassandra.username | quote }}
{{- end }}
{{- if .Values.saml.entityID }}
SAML_ENTITY_ID: {{ .Values.saml.entityID | quote }}
{{- end }}
{{- if .Values.saml.acsURL }}
SAML_ACS_URL: {{ .Values.saml.acsURL | quote }}
{{- end }}
{{- if .Values.saml.idpMetadataURL }}
SAML_IDP_METADATA_URL: {{ .Values.saml.idpMetadataURL | quote }}
{{- end }}
{{- end -}}
