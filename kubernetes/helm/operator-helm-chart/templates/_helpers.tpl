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
Return pod-level securityContext YAML block according to values and debug flag.
This helper prints the inner keys only; caller should indent appropriately.
*/ -}}
{{- define "gateway-operator.podSecurityContext" -}}
{{- if .Values.debug.enabled }}
runAsUser: 0
{{- else }}
{{- if .Values.securityContext.runAsUser }}
runAsUser: {{ .Values.securityContext.runAsUser }}
{{- end }}
{{- if .Values.securityContext.runAsNonRoot }}
runAsNonRoot: true
{{- end }}
{{- end }}
{{- end }}

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
  - gateway.api-platform.wso2.com
  resources:
  - restapis
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
  verbs:
  - update
- apiGroups:
  - gateway.api-platform.wso2.com
  resources:
  - restapis/status
  verbs:
  - get
  - patch
  - update
- apiGroups:
  - gateway.api-platform.wso2.com
  resources:
  - gateways
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
  - gateways/finalizers
  verbs:
  - update
- apiGroups:
  - gateway.api-platform.wso2.com
  resources:
  - gateways/status
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
{{- end -}}
