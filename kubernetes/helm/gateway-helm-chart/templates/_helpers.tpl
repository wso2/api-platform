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

{{/*
Resolve how the gateway-controller receives one of its secrets: the control-plane
token or a database password. Each one is delivered either as an environment
variable (source: env, the default) or as a file mounted from its Secret
(source: file) that the controller reads with a {{ file }} interpolation token, so the
value never enters the pod environment. File tokens are rendered into the controller-only
controller-secrets.toml, never config.toml (see gateway-config.yaml). A missing key falls back to the chart
default, so a release upgraded with --reuse-values (or values from the operator
that predate these keys) renders exactly as before.

File-mode settings are validated only when the secret is in use (the token always,
a database password only for its storage.type), so a value that rendered before
these keys existed still renders.

Args (dict): root, which ("token" | "postgres" | "sqlserver")
Returns YAML: field, nameField, name, key, source, mountPath, path, envVar, volume,
inUse, token (the quoted TOML value config.toml assigns: an env or file token).
*/}}
{{- define "gateway-operator.controllerSecret" -}}
{{- $c := .root.Values.gateway.controller -}}
{{- $storageType := .root.Values.gateway.config.controller.storage.type -}}
{{- $s := dict -}}
{{- if eq .which "token" -}}
{{- $t := default (dict) (default (dict) $c.controlPlane).token -}}
{{- $s = dict "field" "gateway.controller.controlPlane.token" "nameField" "secretName" "name" (default "" $t.secretName) "key" (default "token" $t.key) "source" (default "env" $t.source) "mountPath" (default "/secrets/gateway-controller/controlplane" $t.mountPath) "envVar" "APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN" "volume" "controlplane-token" "inUse" true -}}
{{- if and (eq $s.source "file") $t.value -}}
{{- fail "gateway.controller.controlPlane.token.value cannot be used with token.source=file: only a Secret can be mounted as a file. Put the token in a Secret and set token.secretName, or use token.source=env." -}}
{{- end -}}
{{- else if eq .which "postgres" -}}
{{- $r := default (dict) (default (dict) $c.postgres).passwordSecretRef -}}
{{- $s = dict "field" "gateway.controller.postgres.passwordSecretRef" "nameField" "name" "name" (default "" $r.name) "key" (default "password" $r.key) "source" (default "env" $r.source) "mountPath" (default "/secrets/gateway-controller/postgres" $r.mountPath) "envVar" "APIP_GW_CONTROLLER_STORAGE_POSTGRES_PASSWORD" "volume" "postgres-password" "inUse" (eq $storageType "postgres") -}}
{{- else if eq .which "sqlserver" -}}
{{- $r := default (dict) (default (dict) $c.sqlserver).passwordSecretRef -}}
{{- $s = dict "field" "gateway.controller.sqlserver.passwordSecretRef" "nameField" "name" "name" (default "" $r.name) "key" (default "password" $r.key) "source" (default "env" $r.source) "mountPath" (default "/secrets/gateway-controller/sqlserver" $r.mountPath) "envVar" "APIP_GW_CONTROLLER_STORAGE_DATABASE_PASSWORD" "volume" "sqlserver-password" "inUse" (eq $storageType "sqlserver") -}}
{{- else -}}
{{- fail (printf "gateway-operator.controllerSecret: unknown secret %q" .which) -}}
{{- end -}}
{{- if not (has $s.source (list "env" "file")) -}}
{{- fail (printf "%s.source must be \"env\" or \"file\", got %q" $s.field (toString $s.source)) -}}
{{- end -}}
{{- $mountPath := trimSuffix "/" (toString $s.mountPath) -}}
{{- $path := printf "%s/%s" $mountPath $s.key -}}
{{- if and (eq $s.source "file") $s.inUse -}}
{{- if not $s.name -}}
{{- fail (printf "%s.%s is required when %s.source is \"file\": the Secret is what gets mounted." $s.field $s.nameField $s.field) -}}
{{- end -}}
{{- if or (not (regexMatch `^[-._a-zA-Z0-9]+$` $s.key)) (eq $s.key ".") (eq $s.key "..") -}}
{{- fail (printf "%s.key must be a Secret key name (letters, digits, '-', '_', '.'), got %q" $s.field $s.key) -}}
{{- end -}}
{{- if or (not (regexMatch `^(/[-_a-zA-Z0-9.]+)+$` $mountPath)) (regexMatch `(^|/)\.\.?(/|$)` $mountPath) -}}
{{- fail (printf "%s.mountPath must be an absolute directory path of plain segments (no '.' or '..'), got %q" $s.field (toString $s.mountPath)) -}}
{{- end -}}
{{- if or (eq $mountPath "/etc/gateway-controller") (hasPrefix "/etc/gateway-controller/config.toml" $mountPath) (hasPrefix "/etc/gateway-controller/controller-secrets.toml" $mountPath) -}}
{{- fail (printf "%s.mountPath must not be /etc/gateway-controller or overlap its config files; use a directory such as /secrets/gateway-controller/<name>." $s.field) -}}
{{- end -}}
{{- end -}}
{{- $token := printf `'{{ env "%s" "" }}'` $s.envVar -}}
{{- if eq $s.source "file" -}}
{{- $token = printf `'{{ file "%s" }}'` $path -}}
{{- end -}}
{{- $_ := set $s "mountPath" $mountPath -}}
{{- $_ := set $s "path" $path -}}
{{- $_ := set $s "token" $token -}}
{{- toYaml $s -}}
{{- end -}}

{{/*
The gateway-controller secrets delivered as mounted files (source: file) that are in
use, as YAML `mounts: [...]` with volume, secretName, key, mountPath for each. Fails
when two of them, or one of them and a mount the chart already makes, share a
mount path: Kubernetes rejects a pod with duplicate mount paths.

Args: the root context.
*/}}
{{- define "gateway-operator.controllerSecretMounts" -}}
{{- $c := .Values.gateway.controller -}}
{{- $taken := dict -}}
{{- if $c.deployment.volumeMountPath -}}{{- $_ := set $taken (trimSuffix "/" $c.deployment.volumeMountPath) "gateway.controller.deployment.volumeMountPath" -}}{{- end -}}
{{- if and $c.encryptionKeys.enabled $c.encryptionKeys.mountPath -}}{{- $_ := set $taken (trimSuffix "/" $c.encryptionKeys.mountPath) "gateway.controller.encryptionKeys.mountPath" -}}{{- end -}}
{{- $mounts := list -}}
{{- range $which := list "token" "postgres" "sqlserver" -}}
{{- $s := include "gateway-operator.controllerSecret" (dict "root" $ "which" $which) | fromYaml -}}
{{- if and $s.inUse (eq $s.source "file") -}}
{{- with (get $taken $s.mountPath) -}}
{{- fail (printf "%s.mountPath %q is already used by %s; give each mounted secret its own directory." $s.field $s.mountPath .) -}}
{{- end -}}
{{- $_ := set $taken $s.mountPath (printf "%s.mountPath" $s.field) -}}
{{- $mounts = append $mounts (dict "volume" $s.volume "secretName" $s.name "key" $s.key "mountPath" $s.mountPath) -}}
{{- end -}}
{{- end -}}
{{- toYaml (dict "mounts" $mounts) -}}
{{- end -}}
