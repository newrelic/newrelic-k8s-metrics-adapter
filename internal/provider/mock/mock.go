// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package mock implements the external provider interface.
package mock

import (
	"context"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/metrics/pkg/apis/external_metrics"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
)

// Provider holds the config of the provider.
type Provider struct {
	GetMethod func(ctx context.Context, namespace string, metricSelector labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) //nolint:lll // External interface requirement.
}

// GetExternalMetric implemented from external provider interface.
func (p *Provider) GetExternalMetric(ctx context.Context, namespace string, metricSelector labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) { //nolint:lll // External interface requirement.
	return p.GetMethod(ctx, namespace, metricSelector, info)
}

// ListAllExternalMetrics implemented from external provider interface.
func (p *Provider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	return []provider.ExternalMetricInfo{
		{
			Metric: "MockMetric",
		},
	}
}
