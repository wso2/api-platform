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

{{/*
Render a string-keyed metadata map (labels/annotations) as YAML. Values are
coerced to strings so numbers/bools from values.yaml render quoted (Kubernetes
requires string values). Keys with null values are skipped.
*/}}
{{- define "gateway-operator.renderStringMap" -}}
{{- $out := dict -}}
{{- range $k, $v := . -}}
{{- if not (kindIs "invalid" $v) -}}
{{- $_ := set $out $k (toString $v) -}}
{{- end -}}
{{- end -}}
{{- toYaml $out -}}
{{- end -}}

{{- define "gateway-operator.labels" -}}
{{- $std := dict
      "helm.sh/chart" (include "gateway-operator.chart" .)
      "app.kubernetes.io/name" (include "gateway-operator.name" .)
      "app.kubernetes.io/managed-by" .Release.Service
      "app.kubernetes.io/instance" .Release.Name
      "app.kubernetes.io/version" .Chart.AppVersion -}}
{{- include "gateway-operator.renderStringMap" (merge (dict) (default (dict) .Values.commonLabels) $std) -}}
{{- end -}}

{{/*
Standard labels plus extra per-resource labels merged in (extra wins).
Args (list): root, extraLabels (may be nil)
*/}}
{{- define "gateway-operator.resourceLabels" -}}
{{- $root := index . 0 -}}
{{- $extra := default (dict) (index . 1) -}}
{{- $base := fromYaml (include "gateway-operator.labels" $root) -}}
{{- include "gateway-operator.renderStringMap" (merge (dict) $extra $base) -}}
{{- end -}}

{{- define "gateway-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gateway-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Standard labels + component label + extra per-resource labels, merged with
precedence: extra > component > commonLabels > standard.
Args (list): root, component, extraLabels (may be nil)
*/}}
{{- define "gateway-operator.componentLabels" -}}
{{- $root := index . 0 -}}
{{- $component := index . 1 -}}
{{- $extra := default (dict) (index . 2) -}}
{{- $base := fromYaml (include "gateway-operator.labels" $root) -}}
{{- include "gateway-operator.renderStringMap" (merge (dict) $extra (dict "app.kubernetes.io/component" $component) $base) -}}
{{- end -}}

{{/*
Pod-template labels: selector labels merged over podLabels and commonLabels.
Selector keys always win — the Deployment selector is immutable and pods must
keep matching it regardless of user-supplied labels.
Args (list): root, component, podLabels (may be nil)
*/}}
{{- define "gateway-operator.componentPodLabels" -}}
{{- $root := index . 0 -}}
{{- $component := index . 1 -}}
{{- $podLabels := default (dict) (index . 2) -}}
{{- $selector := fromYaml (include "gateway-operator.componentSelectorLabels" (list $root $component)) -}}
{{- include "gateway-operator.renderStringMap" (merge (dict) $selector $podLabels (default (dict) $root.Values.commonLabels)) -}}
{{- end -}}

{{/*
Merge commonAnnotations with per-resource annotations (specific wins). Emits
nothing when both are empty so callers can wrap with:
  {{- with (include "gateway-operator.annotations" (list . $specific)) }}
  annotations:
    {{- . | nindent 4 }}
  {{- end }}
Args (list): root, specificAnnotations (may be nil)
*/}}
{{- define "gateway-operator.annotations" -}}
{{- $root := index . 0 -}}
{{- $specific := default (dict) (index . 1) -}}
{{- $merged := merge (dict) $specific (default (dict) $root.Values.commonAnnotations) -}}
{{- if $merged -}}
{{- include "gateway-operator.renderStringMap" $merged -}}
{{- end -}}
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
only when wso2.subscription.imagePullSecret is set AND the repository value is
exactly the chart-canonical default for this component. Any explicit override —
including overrides that happen to stay under `ghcr.io/wso2/api-platform/` (e.g.
SHA-pinned references, canary tags) — passes through unchanged.

Args (dict): root, repository, defaultRepository, tag
*/}}
{{- define "gateway-operator.componentImage" -}}
{{- $root := .root -}}
{{- $repo := .repository -}}
{{- $defaultRepo := .defaultRepository -}}
{{- $tag := .tag -}}
{{- $sub := $root.Values.wso2.subscription.imagePullSecret -}}
{{- $defaultPrefix := "ghcr.io/wso2/api-platform/" -}}
{{- $wso2Prefix := "registry.wso2.com/wso2-api-platform/" -}}
{{- if and (ne $sub "") (eq $repo $defaultRepo) (hasPrefix $defaultPrefix $repo) -}}
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
