{{/* vim: set filetype=mustache: */}}

{{- /* Allow to change pod defaults dynamically based if we are running in privileged mode or not */ -}}
{{- define "common.securityContext.podDefaults" -}}
fsGroup: 1001
runAsUser: 1001
runAsGroup: 1001
{{- end -}}

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
