{{/* vim: set filetype=mustache: */}}

{{/*
Select a value for the region
*/}}
{{- define "newrelic-k8s-metrics-adapter.region" -}}
{{- if .Values.config.region -}}
  {{- .Values.config.region | upper -}}
{{- else if (include "common.nrStaging" .)  -}}
Staging
{{- else if eq (include "common.license._licenseKey" . | substr 0 2) "eu" -}}
EU
{{- end -}}
{{- end -}}
