// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cache implements the external provider interface providing a cache for the encapsulated provider.
package cache

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/metrics/pkg/apis/external_metrics"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
)

// defaultCacheTTL if not specified in ProviderOptions.
const defaultCacheTTL = 30

type cacheProvider struct {
	externalProvider provider.ExternalMetricsProvider
	cacheTTL         int64
}

// ProviderOptions holds the configOptions of the provider.
type ProviderOptions struct {
	ExternalProvider provider.ExternalMetricsProvider
	CacheTTL         int64
}

// NewCacheProvider is the constructor for the cache provider.
func NewCacheProvider(options ProviderOptions) (provider.ExternalMetricsProvider, error) {
	if options.CacheTTL < 0 {
		return nil, fmt.Errorf("cacheTTL cannot be less then 0")
	}

	if options.CacheTTL == 0 {
		options.CacheTTL = defaultCacheTTL
	}

	return &cacheProvider{
		externalProvider: options.ExternalProvider,
		cacheTTL:         options.CacheTTL,
	}, nil
}

// GetExternalMetric returns the requested metric.
func (p *cacheProvider) GetExternalMetric(ctx context.Context, _ string, match labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) { //nolint:lll // External interface requirement.
	m, err := p.externalProvider.GetExternalMetric(ctx, "", match, info)
	if err != nil {
		return nil, fmt.Errorf("getting fresh external metric value: %w", err)
	}

	return m, nil
}

// ListAllExternalMetrics returns the list of external metrics supported by this provider.
func (p *cacheProvider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	return p.externalProvider.ListAllExternalMetrics()
}
