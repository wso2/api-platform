{{/*
Common helpers for templates
*/}}

{{- define "open-choreo-operator.name" -}}
{{- default .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- /*
Return the image to use for the manager container. This chooses the debug image when
debug.enabled is true and a debugImage is provided, otherwise falls back to the
configured image.repository:tag.
*/ -}}
{{- define "open-choreo-operator.image" -}}
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
{{- define "open-choreo-operator.podSecurityContext" -}}
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
{{- end -}}
