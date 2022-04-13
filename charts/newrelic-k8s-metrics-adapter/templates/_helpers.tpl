{{/* vim: set filetype=mustache: */}}

{{- /* Allow to change pod defaults dynamically based if we are running in privileged mode or not */ -}}
{{- define "newrelic-k8s-metrics-adapter.securityContext.pod" -}}
{{- if include "newrelic.common.securityContext.pod" . -}}
{{- include "newrelic.common.securityContext.pod" . -}}
{{- else -}}
fsGroup: 1001
runAsUser: 1001
runAsGroup: 1001
{{- end -}}
{{- end -}}



{{/*
Select a value for the region
When this value is empty the New Relic client region will be the default 'US'
*/}}
{{- define "newrelic-k8s-metrics-adapter.region" -}}
{{- if .Values.config.region -}}
  {{- .Values.config.region | upper -}}
{{- else if (include "newrelic.common.nrStaging" .)  -}}
Staging
{{- else if hasPrefix "eu" (include "newrelic.common.license._licenseKey" .) -}}
EU
{{- end -}}
{{- end -}}
