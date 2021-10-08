// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cache implements the external provider interface providing a cache for the encapsulated provider.
package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/component-base/metrics"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/external_metrics"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
)

// ProviderOptions holds the configOptions of the provider.
type ProviderOptions struct {
	ExternalProvider provider.ExternalMetricsProvider
	CacheTTLSeconds  int64
	RegisterFunc     func(metrics.Registerable) error
}

type cacheProvider struct {
	externalProvider provider.ExternalMetricsProvider
	ttlWindow        time.Duration
	storage          *sync.Map
	cacheMetrics     cacheMetrics
}

type cacheEntry struct {
	value     *external_metrics.ExternalMetricValueList
	timestamp metav1.Time
}

// NewCacheProvider is the constructor for the cache provider.
func NewCacheProvider(options ProviderOptions) (provider.ExternalMetricsProvider, error) {
	if options.CacheTTLSeconds <= 0 {
		klog.Infof("Cache TTL is <= 0. Cache disabled.")

		return options.ExternalProvider, nil
	}

	cacheMetrics := getMetrics()

	if err := registerMetrics(options.RegisterFunc, cacheMetrics); err != nil {
		return nil, fmt.Errorf("registering metrics: %w", err)
	}

	return &cacheProvider{
		externalProvider: options.ExternalProvider,
		ttlWindow:        time.Duration(options.CacheTTLSeconds) * time.Second,
		storage:          &sync.Map{},
		cacheMetrics:     cacheMetrics,
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
		p.cacheMetrics.requestTotal.WithLabelValues("hit").Inc()

		return v, nil
	}

	p.cacheMetrics.requestTotal.WithLabelValues("miss").Inc()

	v, err := p.externalProvider.GetExternalMetric(ctx, "", match, info)
	if err != nil {
		return nil, fmt.Errorf("getting fresh external metric value: %w", err)
	}

	if l := len(v.Items); l != 1 {
		return nil, fmt.Errorf("expected exactly 1 metric from external provider for metric %q, got %d", id, l)
	}

	p.storage.Store(id, &cacheEntry{
		value:     v,
		timestamp: v.Items[0].Timestamp,
	})

	p.cacheMetrics.size.Add(1)

	return v, nil
}

func (p *cacheProvider) getCacheEntry(id string) (*external_metrics.ExternalMetricValueList, bool) {
	value, ok := p.storage.Load(id)
	if !ok {
		return nil, false
	}

	c := value.(*cacheEntry) //nolint:forcetypeassert // Cache should always be of this type.
	if p.isDataTooOld(c.timestamp) {
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

func (p *cacheProvider) isDataTooOld(timestamp metav1.Time) bool {
	oldestSampleAllowed := time.Now().Add(-p.ttlWindow)

	return !timestamp.After(oldestSampleAllowed)
}
