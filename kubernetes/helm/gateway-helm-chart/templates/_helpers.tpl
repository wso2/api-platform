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
{{- else if eq .which "redis" -}}
{{- $r := default (dict) (default (dict) $c.redis).passwordSecretRef -}}
{{- $redis := default (dict) (default (dict) .root.Values.gateway.config.counters).redis -}}
{{- $s = dict "field" "gateway.controller.redis.passwordSecretRef" "nameField" "name" "name" (default "" $r.name) "key" (default "password" $r.key) "source" (default "env" $r.source) "mountPath" (default "/secrets/gateway-controller/redis" $r.mountPath) "envVar" "APIP_GW_COUNTERS_REDIS_PASSWORD" "volume" "redis-password" "inUse" (and $redis.enabled (ne (default "" $r.name) "")) -}}
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
when two of them, or one of them and a mount the chart already makes or an entry of
deployment.extraVolumeMounts, share a mount path: Kubernetes rejects a pod with
duplicate mount paths.

Args: the root context.
*/}}
{{- define "gateway-operator.controllerSecretMounts" -}}
{{- $c := .Values.gateway.controller -}}
{{- $taken := dict -}}
{{- if $c.deployment.volumeMountPath -}}{{- $_ := set $taken (trimSuffix "/" $c.deployment.volumeMountPath) "gateway.controller.deployment.volumeMountPath" -}}{{- end -}}
{{- if and $c.encryptionKeys.enabled $c.encryptionKeys.mountPath -}}{{- $_ := set $taken (trimSuffix "/" $c.encryptionKeys.mountPath) "gateway.controller.encryptionKeys.mountPath" -}}{{- end -}}
{{- /* Certificate mounts, under the same conditions controller/deployment.yaml renders them. */ -}}
{{- if (default (dict) $c.tls).enabled -}}{{- $_ := set $taken "/app/listener-certs" "the controller's listener certificate mount (gateway.controller.tls.enabled)" -}}{{- end -}}
{{- $serverTls := default (dict) .Values.gateway.config.controller.server.tls -}}
{{- if and $serverTls.enabled (ne (default (dict) $c.restApiTls).certificateProvider "none") -}}{{- $_ := set $taken "/app/rest-api-certs" "the controller's REST API certificate mount (gateway.config.controller.server.tls.enabled)" -}}{{- end -}}
{{- $upstreamCerts := default (dict) $c.upstreamCerts -}}
{{- if and $upstreamCerts.enabled (or $upstreamCerts.secretName $upstreamCerts.configMapName) -}}{{- $_ := set $taken "/app/certificates" "the controller's upstream certificate mount (gateway.controller.upstreamCerts)" -}}{{- end -}}
{{- range $i, $m := (default (list) $c.deployment.extraVolumeMounts) -}}
{{- if and (kindIs "map" $m) $m.mountPath -}}{{- $_ := set $taken (trimSuffix "/" (toString $m.mountPath)) (printf "gateway.controller.deployment.extraVolumeMounts[%d]" $i) -}}{{- end -}}
{{- end -}}
{{- $mounts := list -}}
{{- range $which := list "token" "postgres" "sqlserver" "redis" -}}
{{- $s := include "gateway-operator.controllerSecret" (dict "root" $ "which" $which) | fromYaml -}}
{{- if and $s.inUse (eq $s.source "file") -}}
{{- with (get $taken $s.mountPath) -}}
{{- fail (printf "%s.mountPath %q is already used by %s; give each mounted secret its own directory." $s.field $s.mountPath .) -}}
{{- end -}}
{{- $_ := set $taken $s.mountPath (printf "%s.mountPath" $s.field) -}}
{{- $mounts = append $mounts (dict "volume" $s.volume "secretName" $s.name "key" $s.key "mountPath" $s.mountPath) -}}
{{- end -}}
{{- end -}}
{{- range $s := (include "gateway-operator.basicAuthUserSecrets" $ | fromYaml).list -}}
{{- if eq $s.source "file" -}}
{{- with (get $taken $s.mountPath) -}}
{{- fail (printf "%s.mountPath %q is already used by %s; give each mounted secret its own directory." $s.field $s.mountPath .) -}}
{{- end -}}
{{- $_ := set $taken $s.mountPath (printf "%s.mountPath" $s.field) -}}
{{- $mounts = append $mounts (dict "volume" $s.volume "secretName" $s.name "key" $s.key "mountPath" $s.mountPath) -}}
{{- end -}}
{{- end -}}
{{- toYaml (dict "mounts" $mounts) -}}
{{- end -}}

{{/*
The admin credential the chart provides when basic auth is enabled but
gateway.config.controller.auth.basic.users is empty (gateway.controller.adminCredentials).
Resolved once per render and cached under .Values.gateway.controller, so every template
sees the same result and a new password is generated at most once. When it applies, it
also sets gateway.config.controller.auth.basic.users to that one user, so the config,
the Secret mounts and the Route guard render it like a user given in values.

Modes:
  none      basic auth is disabled, users are given in values, or generate is false.
  existing  adminCredentials.existingSecret names the Secret holding the password.
  generated the chart's Secret <fullname>-admin-credentials. Reused when it exists;
            otherwise generated, on a fresh install or a render without cluster access.
  legacy    upgrade of a release made by a chart that predates generation, whose config
            has basic auth enabled with that chart's default plain admin/admin user: it
            keeps that user, so the credential in use does not change. When the earlier
            config shows any other users, or basic auth off, admin/admin was not in use and
            the chart generates a credential instead. A config it cannot parse keeps
            admin/admin, as it cannot tell.
The mode is recorded on the gateway ConfigMap (annotation
gateway.api-platform.wso2.com/admin-credentials), which is how an upgrade tells a
release that generated its credential from one that predates generation. Generated and
existing users read the password from a mounted file, never an environment variable.

Returns YAML: mode, secretName, key, username, password (generated only), new, offline,
checksum (rolls the controller when the password changes). The checksum is on the pod
template, which anyone who can read Deployments, ReplicaSets or Pods sees, so it never
hashes a password a person may have chosen: for a Secret read from the cluster it hashes
the Secret's UID and resourceVersion. Only a password the chart generated in this render
(32 random characters, too long to guess) is hashed itself, because its Secret has no
UID yet.
*/}}
{{/*
How an upgrade treats the admin credential of a release made by a chart that predates
generation (its <fullname>-config ConfigMap has no admin-credentials annotation).
Returns "legacy" when that config ran basic auth with the earlier charts' default plain
admin/admin user, or when its config.toml cannot be parsed (so it cannot tell), and ""
otherwise, which lets the chart generate a credential.
Args: the ConfigMap from lookup.
*/}}
{{- define "gateway-operator.earlierChartAdminMode" -}}
{{- $old := fromToml (toString (dig "data" "config.toml" "" .)) -}}
{{- if hasKey $old "Error" -}}
legacy
{{- else -}}
{{- $basic := dig "controller" "auth" "basic" (dict) $old -}}
{{- if eq (toString (get $basic "enabled")) "true" -}}
{{- $found := false -}}
{{- range (default (list) (get $basic "users")) -}}
{{- if and (eq (toString (get . "username")) "admin") (eq (toString (get . "password")) "admin") (ne (toString (get . "password_hashed")) "true") -}}{{- $found = true -}}{{- end -}}
{{- end -}}
{{- if $found -}}legacy{{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "gateway-operator.adminCredentials" -}}
{{- $c := .Values.gateway.controller -}}
{{- if not (hasKey $c "_resolvedAdminCredentials") -}}
{{- $ac := default (dict) $c.adminCredentials -}}
{{- $auth := default (dict) .Values.gateway.config.controller.auth -}}
{{- $basic := default (dict) $auth.basic -}}
{{- $r := dict "mode" "none" -}}
{{- if and $basic.enabled (not $basic.users) -}}
{{- $username := default "admin" $ac.username -}}
{{- if $ac.existingSecret -}}
{{- $r = dict "mode" "existing" "secretName" $ac.existingSecret "key" (default "password" $ac.existingSecretKey) "username" $username -}}
{{- $existing := lookup "v1" "Secret" .Release.Namespace $ac.existingSecret -}}
{{- with $existing.metadata -}}{{- $_ := set $r "checksum" (sha256sum (printf "%s/%s" .uid .resourceVersion)) -}}{{- end -}}
{{- else if not (and (hasKey $ac "generate") (eq (toString $ac.generate) "false")) -}}
{{- $name := printf "%s-admin-credentials" (include "gateway-operator.fullname" .) -}}
{{- /* kube-root-ca.crt exists in every namespace, so an empty lookup means the chart is
       rendered without cluster access (helm template, Argo CD, --dry-run=client). */ -}}
{{- $online := lookup "v1" "ConfigMap" .Release.Namespace "kube-root-ca.crt" -}}
{{- $r = dict "mode" "generated" "secretName" $name "key" "password" "username" $username "new" false "offline" (not $online) -}}
{{- $existing := lookup "v1" "Secret" .Release.Namespace $name -}}
{{- $password := "" -}}
{{- with $existing.data -}}{{- $password = b64dec (toString (default "" (get . "password"))) -}}{{- end -}}
{{- if $password -}}
{{- $_ := set $r "password" $password -}}
{{- $_ := set $r "checksum" (sha256sum (printf "%s/%s" $existing.metadata.uid $existing.metadata.resourceVersion)) -}}
{{- else -}}
{{- $cm := lookup "v1" "ConfigMap" .Release.Namespace (printf "%s-config" (include "gateway-operator.fullname" .)) -}}
{{- $previous := "" -}}
{{- if $cm -}}
{{- $previous = dig "metadata" "annotations" "gateway.api-platform.wso2.com/admin-credentials" "" $cm -}}
{{- if not $previous -}}{{- $previous = include "gateway-operator.earlierChartAdminMode" $cm -}}{{- end -}}
{{- end -}}
{{- if eq $previous "generated" -}}
{{- fail (printf "gateway.controller.adminCredentials: this release generated its admin credential in Secret %s, which no longer exists. Restore the Secret, or set gateway.controller.adminCredentials.existingSecret or gateway.config.controller.auth.basic.users. The chart does not generate a replacement on upgrade." $name) -}}
{{- else if eq $previous "legacy" -}}
{{- $r = dict "mode" "legacy" -}}
{{- else -}}
{{- $_ := set $r "password" (randAlphaNum 32) -}}
{{- $_ := set $r "new" true -}}
{{- $_ := set $r "checksum" (sha256sum (printf "%s/%s" $name $r.password)) -}}
{{- end -}}
{{- end -}}
{{- else if not (dig "idp" "enabled" false $auth) -}}
{{- fail "gateway.config.controller.auth.basic is enabled with no users, gateway.controller.adminCredentials.generate is false and IDP auth is off, so the management API would accept every request. Add users, set adminCredentials.existingSecret, or let the chart generate the credential." -}}
{{- end -}}
{{- if has $r.mode (list "existing" "generated") -}}
{{- $_ := set $basic "users" (list (dict "username" $r.username "passwordSecretRef" (dict "name" $r.secretName "key" $r.key "source" "file") "password_hashed" false "roles" (list "admin"))) -}}
{{- else if eq $r.mode "legacy" -}}
{{- $_ := set $basic "users" (list (dict "username" "admin" "password" "admin" "password_hashed" false "roles" (list "admin"))) -}}
{{- end -}}
{{- end -}}
{{- $_ := set $c "_resolvedAdminCredentials" $r -}}
{{- end -}}
{{- toYaml (get $c "_resolvedAdminCredentials") -}}
{{- end -}}

{{/*
Basic-auth users whose password comes from a Kubernetes Secret
(gateway.config.controller.auth.basic.users[i].passwordSecretRef), resolved the same
way as gateway-operator.controllerSecret: delivered as an environment variable
(source: env, the default) or as a file mounted from the Secret (source: file) that the
controller reads with a {{ file }} token, so the value never enters the pod environment.
Users with a plain password are not listed and render exactly as before. Only resolved
while basic auth is enabled, so a disabled block never mounts or requires a Secret.

Args: the root context.
Returns YAML: list (each entry: index, field, name, key, source, mountPath, path, envVar,
volume, token) and byIndex (the same entries keyed by the user's index as a string).
*/}}
{{- define "gateway-operator.basicAuthUserSecrets" -}}
{{- $_ := include "gateway-operator.adminCredentials" . -}}
{{- $auth := default (dict) .Values.gateway.config.controller.auth -}}
{{- $basic := default (dict) $auth.basic -}}
{{- $list := list -}}
{{- $byIndex := dict -}}
{{- if $basic.enabled -}}
{{- range $i, $u := (default (list) $basic.users) -}}
{{- if and (kindIs "map" $u) (hasKey $u "passwordSecretRef") -}}
{{- $field := printf "gateway.config.controller.auth.basic.users[%d].passwordSecretRef" $i -}}
{{- $r := $u.passwordSecretRef -}}
{{- if not (kindIs "map" $r) -}}
{{- fail (printf "%s must be a map with name, key and source." $field) -}}
{{- end -}}
{{- if $u.password -}}
{{- fail (printf "gateway.config.controller.auth.basic.users[%d] sets both password and passwordSecretRef; set only one." $i) -}}
{{- end -}}
{{- $s := dict "index" $i "field" $field "name" (default "" $r.name) "key" (default "password" $r.key) "source" (default "env" $r.source) "mountPath" (trimSuffix "/" (toString (default (printf "/secrets/gateway-controller/basic-auth/user-%d" $i) $r.mountPath))) "envVar" (printf "APIP_GW_CONTROLLER_AUTH_BASIC_USER_%d_PASSWORD" $i) "volume" (printf "basic-auth-user-%d" $i) -}}
{{- if not (has $s.source (list "env" "file")) -}}
{{- fail (printf "%s.source must be \"env\" or \"file\", got %q" $field (toString $s.source)) -}}
{{- end -}}
{{- if not $s.name -}}
{{- fail (printf "%s.name is required: it names the Secret that holds the password." $field) -}}
{{- end -}}
{{- if or (not (regexMatch `^[-._a-zA-Z0-9]+$` $s.key)) (eq $s.key ".") (hasPrefix ".." $s.key) (gt (len $s.key) 253) -}}
{{- fail (printf "%s.key must be a Secret key name (letters, digits, '-', '_', '.'), at most 253 characters, not starting with '..', got %q" $field $s.key) -}}
{{- end -}}
{{- if eq $s.source "file" -}}
{{- if contains ".." $s.key -}}
{{- fail (printf "%s.key must not contain '..' when source is file: the gateway-controller rejects such a file path, got %q" $field $s.key) -}}
{{- end -}}
{{- if or (not (regexMatch `^(/[-_a-zA-Z0-9.]+)+$` $s.mountPath)) (regexMatch `(^|/)\.\.?(/|$)` $s.mountPath) -}}
{{- fail (printf "%s.mountPath must be an absolute directory path of plain segments (no '.' or '..'), got %q" $field $s.mountPath) -}}
{{- end -}}
{{- if or (eq $s.mountPath "/etc/gateway-controller") (hasPrefix "/etc/gateway-controller/config.toml" $s.mountPath) (hasPrefix "/etc/gateway-controller/controller-secrets.toml" $s.mountPath) -}}
{{- fail (printf "%s.mountPath must not be /etc/gateway-controller or overlap its config files; use a directory such as /secrets/gateway-controller/<name>." $field) -}}
{{- end -}}
{{- end -}}
{{- $path := printf "%s/%s" $s.mountPath $s.key -}}
{{- $token := printf `'{{ env "%s" "" }}'` $s.envVar -}}
{{- if eq $s.source "file" -}}
{{- $token = printf `'{{ file "%s" }}'` $path -}}
{{- end -}}
{{- $_ := set $s "path" $path -}}
{{- $_ := set $s "token" $token -}}
{{- $list = append $list $s -}}
{{- $_ := set $byIndex (toString $i) $s -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- toYaml (dict "list" $list "byIndex" $byIndex) -}}
{{- end -}}

{{/*
Whether the [controller.auth.basic] block is rendered into the controller-only
controller-secrets.toml instead of config.toml: true when any basic-auth user reads its
password from a mounted file. The second -config file replaces arrays, so the whole users
list moves together, and {{ file }} tokens must stay out of config.toml, which the policy
engine also interpolates without the controller's mounts. Env tokens carry an empty
default and stay in config.toml, like the other env-delivered controller secrets.
Returns "true" or "".
*/}}
{{- define "gateway-operator.basicAuthInSecretsFile" -}}
{{- range (include "gateway-operator.basicAuthUserSecrets" . | fromYaml).list -}}
{{- if eq .source "file" -}}true{{- end -}}
{{- end -}}
{{- end -}}

{{/*
The [controller.auth.basic] TOML block, unindented. A user with passwordSecretRef gets the
env or file token from gateway-operator.basicAuthUserSecrets, or an empty password while
basic auth is disabled (the controller then ignores the users); a user with a plain
password renders it as before.
Args: the root context.
*/}}
{{- define "gateway-operator.basicAuthToml" -}}
{{- $_ := include "gateway-operator.adminCredentials" . -}}
{{- $basic := .Values.gateway.config.controller.auth.basic -}}
{{- $secrets := (include "gateway-operator.basicAuthUserSecrets" . | fromYaml).byIndex -}}
[controller.auth.basic]
enabled = {{ $basic.enabled }}
{{- if $basic.users }}
{{- range $idx, $user := $basic.users }}
{{- $s := get $secrets (toString $idx) }}
{{- if regexMatch `[:\x00-\x1f\x7f]` (toString $user.username) }}
{{- fail (printf "gateway.config.controller.auth.basic.users[%d].username cannot contain ':' or control characters." $idx) }}
{{- end }}
{{- if kindIs "invalid" $user.username }}
{{- fail (printf "gateway.config.controller.auth.basic.users[%d].username is required." $idx) }}
{{- end }}
{{- if and (kindIs "invalid" $user.password) (not (hasKey $user "passwordSecretRef")) }}
{{- fail (printf "gateway.config.controller.auth.basic.users[%d] needs password or passwordSecretRef." $idx) }}
{{- end }}
[[controller.auth.basic.users]]
username = {{ $user.username | quote }}
{{- if $s }}
password = {{ $s.token }}
{{- else if and (kindIs "map" $user) (hasKey $user "passwordSecretRef") (not $user.password) }}
{{- /* Basic auth is disabled, so the Secret is not resolved and the controller does not
       read the users; keep the TOML valid without requiring the Secret. */}}
password = ""
{{- else }}
password = {{ $user.password | quote }}
{{- end }}
password_hashed = {{ $user.password_hashed | default false }}
roles = [{{- range $i, $role := $user.roles }}{{- if gt $i 0 }}, {{ end }}{{ $role | quote }}{{- end }}]
{{- end }}
{{- end }}
{{- end -}}

{{/*
Render a gateway controller probe. An httpGet probe without a port targets the management API
listener the controller opens: "rest-tls" over HTTPS when gateway.config.controller.server.tls is
enabled, otherwise "rest" over HTTP. A probe that names its own port, or is not an httpGet probe,
is rendered as given. An empty or null probe renders nothing.
Usage: include "gateway-operator.controllerProbe" (dict "probe" $probe "tls" $serverTls.enabled)
*/}}
{{- define "gateway-operator.controllerProbe" -}}
{{- if .probe -}}
{{- $tls := .tls -}}
{{- $probe := deepCopy .probe -}}
{{- with $probe.httpGet -}}
{{- if not (hasKey . "port") -}}
{{- $_ := set . "port" (ternary "rest-tls" "rest" (eq (toString $tls) "true")) -}}
{{- $_ := set . "scheme" (ternary "HTTPS" "HTTP" (eq (toString $tls) "true")) -}}
{{- end -}}
{{- end -}}
{{- toYaml $probe -}}
{{- end -}}
{{- end -}}
