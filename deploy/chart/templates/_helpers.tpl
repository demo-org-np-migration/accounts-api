{{- define "accounts-api.name" -}}
accounts-api
{{- end -}}

{{- define "accounts-api.labels" -}}
app.kubernetes.io/name: {{ include "accounts-api.name" . }}
app.kubernetes.io/part-of: cauri
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
cauri.io/team: core-banking
{{- end -}}

{{- define "accounts-api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "accounts-api.name" . }}
{{- end -}}
