{{/*
Define container-image so that we can override which image is used
*/}}
{{- define "container-image" -}}
{{- $ := index . 0 }}
{{- with index . 1 }}
{{- if .image -}}
{{ .image }}
{{- else -}}
{{ .repository }}:{{ .tag | default $.Chart.AppVersion }}
{{- end -}}
{{- end -}}
{{- end -}}
