// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"fmt"

	"k8s.io/component-base/metrics"
)

const (
	// MetricsSubsystem groups metrics coming from cache provider.
	MetricsSubsystem = "external_provider_cache"
	namespace        = "newrelic_adapter"
)

type cacheMetrics struct {
	size         *metrics.Gauge
	requestTotal *metrics.CounterVec
}

func getMetrics() cacheMetrics {
	return cacheMetrics{
		size: metrics.NewGauge(
			&metrics.GaugeOpts{
				Help:           "Number of external metrics entries stored in the cache.",
				Namespace:      namespace,
				Subsystem:      MetricsSubsystem,
				Name:           "size",
				StabilityLevel: metrics.ALPHA,
			}),
		requestTotal: metrics.NewCounterVec(
			&metrics.CounterOpts{
				Help:           "Total number of cache request.",
				Namespace:      namespace,
				Subsystem:      MetricsSubsystem,
				Name:           "requests_total",
				StabilityLevel: metrics.ALPHA,
			}, []string{"result"}),
	}
}

func registerMetrics(registerFunc func(metrics.Registerable) error, cacheMetrics cacheMetrics) error {
	if registerFunc == nil {
		return nil
	}

	for i, metric := range []metrics.Registerable{
		cacheMetrics.size,
		cacheMetrics.requestTotal,
	} {
		if err := registerFunc(metric); err != nil {
			return fmt.Errorf("registering metric %d: %w", i, err)
		}
	}

	return nil
}
