{{/*
Common helpers for templates
*/}}

{{- define "gateway-operator.name" -}}
{{- default .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
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

{{- /*
Return the image to use for the manager container. This chooses the debug image when
debug.enabled is true and a debugImage is provided, otherwise falls back to the
configured image.repository:tag.
*/ -}}
{{- define "gateway-operator.image" -}}
{{- if .Values.debug.enabled -}}
	{{- if .Values.debug.debugImage -}}
		{{- printf "%s:%s" .Values.debug.debugImage .Values.image.tag -}}
	{{- else -}}
		{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
	{{- end -}}
{{- else -}}
	{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- end -}}
{{- end -}}

{{- /*
Renders "true" when the Gateway API controllers (GatewayClass, Gateway, HTTPRoute) are
on: gatewayApi.enabled, true when unset.
*/ -}}
{{- define "gateway-operator.gatewayApiEnabled" -}}
{{- if dig "enabled" true (default (dict) .Values.gatewayApi) -}}
true
{{- end -}}
{{- end -}}

{{- /*
RBAC for GatewayClass (cluster-scoped). Namespace Roles cannot grant these
verbs at cluster scope; when watchNamespaces is set, chart also installs
clusterrole-gatewayclass.yaml + binding.
*/ -}}
{{- define "gateway-operator.rbacRulesGatewayClass" -}}
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - gatewayclasses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - gatewayclasses/status
  verbs:
  - get
  - patch
  - update
{{- end }}

{{/*
Renders "true" when deploying to OpenShift (openshift.enabled true, or auto and the
cluster serves route.openshift.io/v1), else empty.
*/}}
{{- define "gateway-operator.isOpenShift" -}}
{{- $mode := toString (dig "enabled" false (default (dict) .Values.openshift)) -}}
{{- if not (has $mode (list "true" "false" "auto")) -}}
{{- fail (printf "openshift.enabled must be true, false or auto, got %q" $mode) -}}
{{- end -}}
{{- if or (eq $mode "true") (and (eq $mode "auto") (.Capabilities.APIVersions.Has "route.openshift.io/v1")) -}}
true
{{- end -}}
{{- end -}}

{{- /*
Common RBAC rules shared between ClusterRole (global) and Role (scoped)
*/ -}}
{{- define "gateway-operator.rbacRules" -}}
- apiGroups:
  - ""
  resources:
  - configmaps
  - persistentvolumeclaims
  - serviceaccounts
  - services
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - namespaces
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - gateway.api-platform.wso2.com
  resources:
  - restapis
  - llmproviders
  - llmprovidertemplates
  - llmproxies
  - mcps
  - agents
  - managedsecrets
  - certificates
  - apikeys
  - subscriptionplans
  - subscriptions
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - gateway.api-platform.wso2.com
  resources:
  - restapis/finalizers
  - llmproviders/finalizers
  - llmprovidertemplates/finalizers
  - llmproxies/finalizers
  - mcps/finalizers
  - agents/finalizers
  - managedsecrets/finalizers
  - certificates/finalizers
  - apikeys/finalizers
  - subscriptionplans/finalizers
  - subscriptions/finalizers
  verbs:
  - update
- apiGroups:
  - gateway.api-platform.wso2.com
  resources:
  - restapis/status
  - llmproviders/status
  - llmprovidertemplates/status
  - llmproxies/status
  - mcps/status
  - agents/status
  - managedsecrets/status
  - certificates/status
  - apikeys/status
  - subscriptionplans/status
  - subscriptions/status
  verbs:
  - get
  - patch
  - update
- apiGroups:
  - gateway.api-platform.wso2.com
  resources:
  - apigateways
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - gateway.api-platform.wso2.com
  resources:
  - apigateways/finalizers
  verbs:
  - update
- apiGroups:
  - gateway.api-platform.wso2.com
  resources:
  - apigateways/status
  verbs:
  - get
  - patch
  - update
{{- if include "gateway-operator.gatewayApiEnabled" . }}
- apiGroups:
  - gateway.api-platform.wso2.com
  resources:
  - apipolicies
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - gateway.api-platform.wso2.com
  resources:
  - apipolicies/status
  verbs:
  - get
  - patch
  - update
{{- end }}
- apiGroups:
  - apps
  resources:
  - deployments
  - replicasets
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - cert-manager.io
  resources:
  - certificates
  - issuers
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
{{- if include "gateway-operator.gatewayApiEnabled" . }}
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - gateways
  verbs:
  - get
  - list
  - watch
  - update
  - patch
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - gateways/status
  verbs:
  - get
  - patch
  - update
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - gateways/finalizers
  verbs:
  - update
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - httproutes
  verbs:
  - get
  - list
  - watch
  - update
  - patch
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - httproutes/status
  verbs:
  - get
  - patch
  - update
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - httproutes/finalizers
  verbs:
  - update
{{ include "gateway-operator.rbacRulesGatewayClass" . }}
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - referencegrants
  verbs:
  - get
  - list
  - watch
{{- end }}
{{- if include "gateway-operator.isOpenShift" . }}
- apiGroups:
  - route.openshift.io
  resources:
  - routes
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - route.openshift.io
  resources:
  - routes/custom-host
  verbs:
  - create
  - patch
  - update
{{- end }}
{{- end -}}

{{/*
Names of the APIGateway conversion webhook Service and certificate Secret.
*/}}
{{- define "gateway-operator.conversionWebhookServiceName" -}}
{{- printf "%s-webhook" (include "gateway-operator.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "gateway-operator.conversionWebhookSecretName" -}}
{{- printf "%s-webhook-cert" (include "gateway-operator.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Pod label that marks operator pods serving the APIGateway conversion webhook. The
webhook Service selects it, so it only reaches pods of a release that serves the
webhook. It is not part of the Deployment's selector, which is immutable.
*/}}
{{- define "gateway-operator.conversionWebhookPodLabel" -}}
gateway.api-platform.wso2.com/conversion-webhook-server: "true"
{{- end -}}

{{/*
Conversion webhook settings with their defaults applied. An upgrade with
--reuse-values keeps the previous release's values, which predate
conversionWebhook, so every key falls back to the same default as values.yaml.
*/}}
{{- define "gateway-operator.conversionWebhookPort" -}}
{{- $cw := .Values.conversionWebhook | default dict -}}
{{- $cw.port | default 9443 -}}
{{- end -}}

{{/*
Operator flags for the APIGateway conversion webhook.
*/}}
{{- define "gateway-operator.conversionWebhookArgs" -}}
{{- $cw := .Values.conversionWebhook | default dict -}}
- --conversion-webhook-port={{ include "gateway-operator.conversionWebhookPort" . }}
- --conversion-webhook-service-name={{ include "gateway-operator.conversionWebhookServiceName" . }}
- --conversion-webhook-cert-secret={{ include "gateway-operator.conversionWebhookSecretName" . }}
- --conversion-webhook-max-request-bytes={{ int64 ($cw.maxRequestBytes | default 67108864) }}
- --conversion-webhook-pqc-key-exchange={{ ternary $cw.pqcKeyExchange true (hasKey $cw "pqcKeyExchange") }}
{{- end -}}
