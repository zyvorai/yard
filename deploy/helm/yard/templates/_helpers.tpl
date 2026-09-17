{{- define "yard.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "yard.labels" -}}
app.kubernetes.io/name: {{ include "yard.name" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "yard.selectorLabels" -}}
app.kubernetes.io/name: {{ include "yard.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
