{{/* 名称截断至 63 字符（DNS 名称限制） */}}
{{- define "prometheus-webhook-feishu.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* 完整名：release-name + chart-name，可被 fullnameOverride 覆盖 */}}
{{- define "prometheus-webhook-feishu.fullname" -}}
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

{{/* chart 名与版本标签 */}}
{{- define "prometheus-webhook-feishu.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* 通用标签 */}}
{{- define "prometheus-webhook-feishu.labels" -}}
helm.sh/chart: {{ include "prometheus-webhook-feishu.chart" . }}
{{ include "prometheus-webhook-feishu.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/* 选择器标签（不可变更，Deployment selector 必须与 Pod labels 一致） */}}
{{- define "prometheus-webhook-feishu.selectorLabels" -}}
app.kubernetes.io/name: {{ include "prometheus-webhook-feishu.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/* ServiceAccount 名称 */}}
{{- define "prometheus-webhook-feishu.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "prometheus-webhook-feishu.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}
