// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package mock implements the external provider interface.
package mock

import (
	"context"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/metrics/pkg/apis/external_metrics"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
)

// Provider holds the config of the provider.
type Provider struct {
	GetExternalMetricFunc      func(ctx context.Context, namespace string, metricSelector labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) //nolint:lll // External interface requirement.
	ListAllExternalMetricsFunc func() []provider.ExternalMetricInfo
}

// GetExternalMetric implemented from external provider interface.
func (p *Provider) GetExternalMetric(ctx context.Context, namespace string, metricSelector labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) { //nolint:lll // External interface requirement.
	if p.GetExternalMetricFunc != nil {
		return p.GetExternalMetricFunc(ctx, namespace, metricSelector, info)
	}

	return &external_metrics.ExternalMetricValueList{
		Items: []external_metrics.ExternalMetricValue{
			{
				MetricName: "MockMetric",
				MetricLabels: map[string]string{
					"foo": "bar",
				},
				Timestamp: metav1.Now(),
				Value:     *resource.NewQuantity(1, resource.DecimalSI),
			},
		},
	}, nil
}

// ListAllExternalMetrics implemented from external provider interface.
func (p *Provider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	if p.ListAllExternalMetricsFunc != nil {
		return p.ListAllExternalMetricsFunc()
	}

	return []provider.ExternalMetricInfo{
		{
			Metric: "MockMetric",
		},
	}
}
