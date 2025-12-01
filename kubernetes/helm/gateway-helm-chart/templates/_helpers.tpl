{{/* vim: set filetype=mustache: */}}
{{- define "gateway-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "gateway-operator.fullname" -}}
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

{{- define "gateway-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "gateway-operator.labels" -}}
helm.sh/chart: {{ include "gateway-operator.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- with .Values.commonLabels }}
{{ toYaml . | indent 0 }}
{{- end }}
{{- end -}}

{{- define "gateway-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gateway-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "gateway-operator.componentLabels" -}}
{{- $root := index . 0 -}}
{{- $component := index . 1 -}}
{{- $extra := default (dict) (index . 2) -}}
{{ include "gateway-operator.labels" $root }}
app.kubernetes.io/component: {{ $component }}
{{- with $extra }}
{{ toYaml . | indent 0 }}
{{- end }}
{{- end -}}

{{- define "gateway-operator.componentSelectorLabels" -}}
{{- $root := index . 0 -}}
{{- $component := index . 1 -}}
{{ include "gateway-operator.selectorLabels" $root }}
app.kubernetes.io/component: {{ $component }}
{{- end -}}

{{- define "gateway-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "gateway-operator.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}
