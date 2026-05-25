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

{{/*
Render a component image reference, applying the WSO2 subscription registry rewrite
when wso2.subscription.imagePullSecret is set AND the repository still matches the
default upstream prefix `ghcr.io/wso2/api-platform/`. Explicit overrides pass through.

Args (dict): root, repository, tag
*/}}
{{- define "gateway-operator.componentImage" -}}
{{- $root := .root -}}
{{- $repo := .repository -}}
{{- $tag := .tag -}}
{{- $sub := $root.Values.wso2.subscription.imagePullSecret -}}
{{- $defaultPrefix := "ghcr.io/wso2/api-platform/" -}}
{{- $wso2Prefix := "registry.wso2.com/wso2-api-platform/" -}}
{{- if and (ne $sub "") (hasPrefix $defaultPrefix $repo) -}}
{{- printf "%s%s:%s" $wso2Prefix (trimPrefix $defaultPrefix $repo) $tag -}}
{{- else -}}
{{- printf "%s:%s" $repo $tag -}}
{{- end -}}
{{- end -}}

{{/*
Render an `imagePullSecrets:` YAML block (without indentation) by merging:
  1. wso2.subscription.imagePullSecret (if set)
  2. .Values.imagePullSecrets (global)
  3. component-level imagePullSecrets (passed in)

Returns an empty string when no secrets resolve, so callers can wrap in
`{{- with (include ...) }} {{- . | nindent N }} {{- end }}`.

Args (dict): root, componentPullSecrets
*/}}
{{- define "gateway-operator.componentImagePullSecretsBlock" -}}
{{- $root := .root -}}
{{- $componentPullSecrets := default (list) .componentPullSecrets -}}
{{- $globalPullSecrets := default (list) $root.Values.imagePullSecrets -}}
{{- $sub := $root.Values.wso2.subscription.imagePullSecret -}}
{{- $subList := ternary (list $sub) (list) (ne $sub "") -}}
{{- $all := concat $subList $globalPullSecrets $componentPullSecrets -}}
{{- if $all -}}
imagePullSecrets:
{{- range $all }}
  - name: {{ . }}
{{- end }}
{{- end -}}
{{- end -}}
