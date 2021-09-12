module github.com/gsanchezgavier/metrics-adapter

go 1.16

require (
	github.com/newrelic/newrelic-client-go v0.62.1
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/apimachinery v0.22.0
	k8s.io/apiserver v0.22.0
	k8s.io/client-go v0.22.0
	k8s.io/component-base v0.22.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e
	k8s.io/metrics v0.22.0
	sigs.k8s.io/controller-runtime v0.9.6
	sigs.k8s.io/custom-metrics-apiserver v1.22.0
)
