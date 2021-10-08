// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package newrelic

import (
	"fmt"

	"k8s.io/component-base/metrics"
)

const (
	namespace = "newrelic_adapter"
	subsystem = "external_provider"
)

type providerMetrics struct {
	queriesTotal *metrics.CounterVec
}

func getMetrics() providerMetrics {
	return providerMetrics{
		queriesTotal: metrics.NewCounterVec(
			&metrics.CounterOpts{
				Help:           "Total number of queries to the NewRelic backend.",
				Namespace:      namespace,
				Subsystem:      subsystem,
				Name:           "queries_total",
				StabilityLevel: metrics.ALPHA,
			}, []string{"result"}),
	}
}

func registerMetrics(registerFunc func(metrics.Registerable) error, providerMetrics providerMetrics) error {
	if registerFunc == nil {
		return nil
	}

	if err := registerFunc(providerMetrics.queriesTotal); err != nil {
		return fmt.Errorf("registering queries total metric: %w", err)
	}

	return nil
}
