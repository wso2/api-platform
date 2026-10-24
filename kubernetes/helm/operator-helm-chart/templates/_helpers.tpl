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
