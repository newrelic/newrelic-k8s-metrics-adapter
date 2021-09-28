// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cache implements the external provider interface providing a cache for the encapsulated provider.
package cache

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/external_metrics"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
)

// defaultCacheTTL if not specified in ProviderOptions.
const defaultCacheTTL = 30

type cacheProvider struct {
	externalProvider provider.ExternalMetricsProvider
	cacheTTL         int64
	storage          *sync.Map
}

type cacheEntry struct {
	value          *external_metrics.ExternalMetricValueList
	retrievingTime time.Time
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
		storage:          &sync.Map{},
	}, nil
}

// ListAllExternalMetrics returns the list of external metrics supported by this provider.
func (p *cacheProvider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	return p.externalProvider.ListAllExternalMetrics()
}

// GetExternalMetric returns the requested metric.
func (p *cacheProvider) GetExternalMetric(ctx context.Context, _ string, match labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) { //nolint:lll // External interface requirement.
	id := getID(info.Metric, match)

	if v, ok := p.getCacheEntry(id); ok {
		return v, nil
	}

	v, err := p.fetchAndSave(ctx, match, info, id)
	if err != nil {
		return nil, err
	}

	return v, nil
}

func (p *cacheProvider) fetchAndSave(ctx context.Context, match labels.Selector, info provider.ExternalMetricInfo, id string) (*external_metrics.ExternalMetricValueList, error) { //nolint:lll // External interface requirement.
	v, err := p.externalProvider.GetExternalMetric(ctx, "", match, info)
	if err != nil {
		return nil, fmt.Errorf("getting fresh external metric value: %w", err)
	}

	p.storage.Store(id, &cacheEntry{
		value:          v,
		retrievingTime: time.Now(),
	})

	return v, nil
}

func (p *cacheProvider) getCacheEntry(id string) (*external_metrics.ExternalMetricValueList, bool) {
	value, ok := p.storage.Load(id)
	if !ok {
		return nil, false
	}

	c, ok := value.(*cacheEntry)
	if !ok {
		klog.Infof("unexpected format for cache entry, %s", reflect.TypeOf(value))

		return nil, false
	}

	if p.isDataTooOld(c.retrievingTime) {
		return nil, false
	}

	return c.value, true
}

func getID(metricName string, selector labels.Selector) string {
	id := metricName
	if selector != nil {
		id = fmt.Sprintf("%s/%s", id, selector.String())
	}

	return id
}

func (p *cacheProvider) isDataTooOld(timestamp time.Time) bool {
	validWindow := time.Duration(p.cacheTTL) * time.Second
	oldestSampleAllowed := time.Now().Add(-validWindow)

	return !timestamp.After(oldestSampleAllowed)
}
