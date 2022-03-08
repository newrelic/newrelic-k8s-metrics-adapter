{{/* vim: set filetype=mustache: */}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "newrelic-k8s-metrics-adapter.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Return the licenseKey
*/}}
{{- define "newrelic-k8s-metrics-adapter.licenseKey" -}}
{{- if .Values.global}}
  {{- if .Values.global.licenseKey }}
      {{- .Values.global.licenseKey -}}
  {{- else -}}
      {{- .Values.licenseKey | default "" -}}
  {{- end -}}
{{- else -}}
    {{- .Values.licenseKey | default "" -}}
{{- end -}}
{{- end -}}

{{/*
Return the cluster
*/}}
{{- define "newrelic-k8s-metrics-adapter.cluster" -}}
{{- if .Values.global -}}
  {{- if .Values.global.cluster -}}
      {{- .Values.global.cluster -}}
  {{- else -}}
      {{- .Values.cluster | default "" -}}
  {{- end -}}
{{- else -}}
  {{- .Values.cluster | default "" -}}
{{- end -}}
{{- end -}}

{{/*
Select a value for the region
*/}}
{{- define "newrelic-k8s-metrics-adapter.region" -}}
{{- if .Values.config.region -}}
  {{- .Values.config.region | upper -}}
{{- else if .Values.global -}}
  {{- if .Values.global.nrStaging -}}
    Staging
  {{- else if eq (include "newrelic-k8s-metrics-adapter.licenseKey" . | substr 0 2) "eu" -}}
    EU
    {{- end }}
{{- else if eq (include "newrelic-k8s-metrics-adapter.licenseKey" . | substr 0 2) "eu" -}}
EU
{{- end -}}
{{- end -}}
