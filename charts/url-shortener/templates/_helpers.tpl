{{/*
Expand the name of the chart.
*/}}
{{- define "url-shortener.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "url-shortener.fullname" -}}
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
Create chart label.
*/}}
{{- define "url-shortener.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "url-shortener.labels" -}}
helm.sh/chart: {{ include "url-shortener.chart" . }}
{{ include "url-shortener.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "url-shortener.selectorLabels" -}}
app.kubernetes.io/name: {{ include "url-shortener.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "url-shortener.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "url-shortener.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image reference.
*/}}
{{- define "url-shortener.image" -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) }}
{{- end }}

{{/*
Name of the Secret for session credentials.
Session existingSecret takes precedence over the rendered secret.
*/}}
{{- define "url-shortener.sessionSecretName" -}}
{{- if .Values.session.existingSecret }}
{{- .Values.session.existingSecret }}
{{- else }}
{{- printf "%s-session" (include "url-shortener.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Name of the Secret for Cassandra credentials.
Priority: existingSecret > valsSecret.name > rendered default.
*/}}
{{- define "url-shortener.cassandraSecretName" -}}
{{- if .Values.cassandra.existingSecret }}
{{- .Values.cassandra.existingSecret }}
{{- else if .Values.cassandra.valsSecret.enabled }}
{{- default (printf "%s-cassandra" (include "url-shortener.fullname" .)) .Values.cassandra.valsSecret.name }}
{{- else }}
{{- printf "%s-cassandra" (include "url-shortener.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Whether a plain Cassandra SSL Secret should be rendered.
True when sslEnabled, inline PEM content provided, and neither existingSSLSecret nor valsSSLSecret is in use.
*/}}
{{- define "url-shortener.cassandraSSLSecretEnabled" -}}
{{- if and .Values.cassandra.sslEnabled (not .Values.cassandra.existingSSLSecret) (not .Values.cassandra.valsSSLSecret.enabled) -}}
{{- if or .Values.cassandra.sslCA .Values.cassandra.sslCert .Values.cassandra.sslKey -}}
true
{{- end -}}
{{- end -}}
{{- end }}

{{/*
Name of the Cassandra SSL Secret.
Priority: existingSSLSecret > valsSSLSecret.name > rendered default.
*/}}
{{- define "url-shortener.cassandraSSLSecretName" -}}
{{- if .Values.cassandra.existingSSLSecret }}
{{- .Values.cassandra.existingSSLSecret }}
{{- else if .Values.cassandra.valsSSLSecret.enabled }}
{{- default (printf "%s-cassandra-ssl" (include "url-shortener.fullname" .)) .Values.cassandra.valsSSLSecret.name }}
{{- else }}
{{- printf "%s-cassandra-ssl" (include "url-shortener.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Name of the Secret for SAML credentials.
*/}}
{{- define "url-shortener.samlSecretName" -}}
{{- if .Values.saml.existingSecret }}
{{- .Values.saml.existingSecret }}
{{- else }}
{{- printf "%s-saml" (include "url-shortener.fullname" .) }}
{{- end }}
{{- end }}
