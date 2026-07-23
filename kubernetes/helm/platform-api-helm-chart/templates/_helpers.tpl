{{/* vim: set filetype=mustache: */}}
{{/*
Shared helpers for the api-platform-portals suite.

Cross-cutting configuration (developmentMode, labels/annotations, image pull
secrets, service account, subscription registry, and the shared Platform API
service coordinates) is read from `.Values.global.*` so every component subchart
resolves it identically. Component-specific config is read from the subchart's
own `.Values` by the calling templates, not here.

Component resource names are derived from the release name with a fixed suffix
(NOT from .Chart.Name), so a portal subchart can compute the Platform API's
in-cluster Service name even though it lives in a different subchart.
*/}}

{{/* Base name: release name, or global.fullnameOverride when set. */}}
{{- define "apip.fullname" -}}
{{- $g := default (dict) .Values.global -}}
{{- if $g.fullnameOverride -}}
{{- $g.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/* Component full names — fixed suffixes, release-derived, cross-subchart stable. */}}
{{- define "apip.platformApi.fullname" -}}
{{- printf "%s-platform-api" (include "apip.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- define "apip.aiWorkspace.fullname" -}}
{{- printf "%s-ai-workspace" (include "apip.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- define "apip.developerPortal.fullname" -}}
{{- printf "%s-developer-portal" (include "apip.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "apip.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Render a string-keyed metadata map (labels/annotations) as YAML. Values are
coerced to strings so numbers/bools render quoted. Null values are skipped.
*/}}
{{- define "apip.renderStringMap" -}}
{{- $out := dict -}}
{{- range $k, $v := . -}}
{{- if not (kindIs "invalid" $v) -}}
{{- $_ := set $out $k (toString $v) -}}
{{- end -}}
{{- end -}}
{{- toYaml $out -}}
{{- end -}}

{{/* Standard labels. `name` = subchart chart name; all components share part-of. */}}
{{- define "apip.labels" -}}
{{- $g := default (dict) .Values.global -}}
{{- $std := dict
      "helm.sh/chart" (include "apip.chart" .)
      "app.kubernetes.io/name" .Chart.Name
      "app.kubernetes.io/managed-by" .Release.Service
      "app.kubernetes.io/instance" .Release.Name
      "app.kubernetes.io/part-of" "api-platform-portals"
      "app.kubernetes.io/version" .Chart.AppVersion -}}
{{- include "apip.renderStringMap" (merge (dict) (default (dict) $g.commonLabels) $std) -}}
{{- end -}}

{{/* Standard labels + extra (extra wins). Args (list): root, extraLabels|nil */}}
{{- define "apip.resourceLabels" -}}
{{- $root := index . 0 -}}
{{- $extra := default (dict) (index . 1) -}}
{{- $base := fromYaml (include "apip.labels" $root) -}}
{{- include "apip.renderStringMap" (merge (dict) $extra $base) -}}
{{- end -}}

{{- define "apip.selectorLabels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/* Standard + component label + extra (extra > component > commonLabels > std). Args: root, component, extra|nil */}}
{{- define "apip.componentLabels" -}}
{{- $root := index . 0 -}}
{{- $component := index . 1 -}}
{{- $extra := default (dict) (index . 2) -}}
{{- $base := fromYaml (include "apip.labels" $root) -}}
{{- include "apip.renderStringMap" (merge (dict) $extra (dict "app.kubernetes.io/component" $component) $base) -}}
{{- end -}}

{{/* Pod-template labels: selector keys always win. Args: root, component, podLabels|nil */}}
{{- define "apip.componentPodLabels" -}}
{{- $root := index . 0 -}}
{{- $component := index . 1 -}}
{{- $podLabels := default (dict) (index . 2) -}}
{{- $g := default (dict) $root.Values.global -}}
{{- $selector := fromYaml (include "apip.componentSelectorLabels" (list $root $component)) -}}
{{- include "apip.renderStringMap" (merge (dict) $selector $podLabels (default (dict) $g.commonLabels)) -}}
{{- end -}}

{{/*
Merge global.commonAnnotations with per-resource annotations (specific wins).
Emits nothing when both empty. Args (list): root, specificAnnotations|nil
*/}}
{{- define "apip.annotations" -}}
{{- $root := index . 0 -}}
{{- $specific := default (dict) (index . 1) -}}
{{- $g := default (dict) $root.Values.global -}}
{{- $merged := merge (dict) $specific (default (dict) $g.commonAnnotations) -}}
{{- if $merged -}}
{{- include "apip.renderStringMap" $merged -}}
{{- end -}}
{{- end -}}

{{- define "apip.componentSelectorLabels" -}}
{{- $root := index . 0 -}}
{{- $component := index . 1 -}}
{{ include "apip.selectorLabels" $root }}
app.kubernetes.io/component: {{ $component }}
{{- end -}}

{{/* Shared service account name (one SA per release, created at umbrella level). */}}
{{- define "apip.serviceAccountName" -}}
{{- $g := default (dict) .Values.global -}}
{{- $sa := default (dict) $g.serviceAccount -}}
{{- if $sa.create -}}
{{- default (include "apip.fullname" .) $sa.name -}}
{{- else -}}
{{- default "default" $sa.name -}}
{{- end -}}
{{- end -}}

{{/*
Default in-cluster URL portals use to reach the shared Platform API. Scheme and
port come from global.platformApi so portals need not read the platform-api
subchart's own values.
*/}}
{{- define "apip.platformApi.internalURL" -}}
{{- $g := default (dict) .Values.global -}}
{{- $pa := default (dict) $g.platformApi -}}
{{- $scheme := ternary "https" "http" (default true $pa.tlsEnabled) -}}
{{- printf "%s://%s:%d" $scheme (include "apip.platformApi.fullname" .) (int (default 9243 $pa.port)) -}}
{{- end -}}

{{/*
Render a component image reference, applying the WSO2 subscription registry
rewrite only when global.wso2.subscription.imagePullSecret is set AND the
repository is exactly the chart-canonical default. Explicit overrides pass
through unchanged. Args (dict): root, repository, defaultRepository, tag
*/}}
{{- define "apip.componentImage" -}}
{{- $root := .root -}}
{{- $repo := .repository -}}
{{- $defaultRepo := .defaultRepository -}}
{{- $tag := .tag -}}
{{- $g := default (dict) $root.Values.global -}}
{{- $wso2sub := default (dict) (default (dict) $g.wso2).subscription -}}
{{- $sub := default "" $wso2sub.imagePullSecret -}}
{{- $defaultPrefix := "ghcr.io/wso2/api-platform/" -}}
{{- $wso2Prefix := "registry.wso2.com/wso2-api-platform/" -}}
{{- if and (ne $sub "") (eq $repo $defaultRepo) (hasPrefix $defaultPrefix $repo) -}}
{{- printf "%s%s:%s" $wso2Prefix (trimPrefix $defaultPrefix $repo) $tag -}}
{{- else -}}
{{- printf "%s:%s" $repo $tag -}}
{{- end -}}
{{- end -}}

{{/*
Render an `imagePullSecrets:` block by merging global.wso2.subscription secret,
global.imagePullSecrets, and component-level pull secrets. Empty string when none.
Args (dict): root, componentPullSecrets
*/}}
{{- define "apip.componentImagePullSecretsBlock" -}}
{{- $root := .root -}}
{{- $componentPullSecrets := default (list) .componentPullSecrets -}}
{{- $g := default (dict) $root.Values.global -}}
{{- $globalPullSecrets := default (list) $g.imagePullSecrets -}}
{{- $wso2sub := default (dict) (default (dict) $g.wso2).subscription -}}
{{- $sub := default "" $wso2sub.imagePullSecret -}}
{{- $subList := ternary (list $sub) (list) (ne $sub "") -}}
{{- $all := concat $subList $globalPullSecrets $componentPullSecrets -}}
{{- if $all -}}
imagePullSecrets:
{{- range $all }}
  - name: {{ . }}
{{- end }}
{{- end -}}
{{- end -}}

{{/*
External-secrets model ("setup generates, startup only checks"). Each helper runs
in its OWN subchart's context, so `.Values.secrets.existingSecret` resolves to
that component's secret. Fails the render when a required secret is unset.
*/}}
{{- define "apip.platformApi.secretName" -}}
{{- $name := .Values.secrets.existingSecret -}}
{{- if not $name -}}
{{- fail "platformApi.secrets.existingSecret is required. Run ./generate-secrets.sh <namespace> to create the Secrets and write values-secrets.yaml, then install with -f values-secrets.yaml." -}}
{{- end -}}
{{- $name -}}
{{- end -}}

{{- define "apip.aiWorkspace.secretName" -}}
{{- $name := .Values.secrets.existingSecret -}}
{{- if not $name -}}
{{- fail "aiWorkspace.secrets.existingSecret is required when aiWorkspace.config.authMode is \"oidc\". Run ./generate-secrets.sh <namespace> with AIW_OIDC_CLIENT_SECRET set, then install with -f values-secrets.yaml." -}}
{{- end -}}
{{- $name -}}
{{- end -}}

{{- define "apip.developerPortal.secretName" -}}
{{- $name := .Values.secrets.existingSecret -}}
{{- if not $name -}}
{{- fail "developerPortal.secrets.existingSecret is required. Run ./generate-secrets.sh <namespace> to create the Secrets and write values-secrets.yaml, then install with -f values-secrets.yaml." -}}
{{- end -}}
{{- $name -}}
{{- end -}}
