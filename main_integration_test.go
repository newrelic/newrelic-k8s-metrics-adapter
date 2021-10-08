// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package main_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	adapter "github.com/newrelic/newrelic-k8s-metrics-adapter"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/testutil"
)

//nolint:paralleltest // This test registers metrics to global registry, so it must not run in parallel.
func Test_Metrics_adapter_exposes_both_API_server_metrics_and_adapter_metrics_under_same_endpoint(t *testing.T) {
	// Setup adapter.
	testEnv := &testutil.TestEnv{
		ContextTimeout:  10 * time.Second,
		StartKubernetes: true,
	}

	testEnv.Generate(t)

	testEnv.Flags = append(testEnv.Flags, "--config-file="+testEnv.ConfigPath)

	go func() {
		if err := adapter.Run(testEnv.Context, testEnv.Flags); err != nil {
			t.Logf("Running operator: %v\n", err)
			t.Fail()
		}
	}()

	// Wait until provider is able to serve metrics.
	url := fmt.Sprintf("%s/apis/external.metrics.k8s.io/v1beta1/namespaces/default/foo", testEnv.BaseURL)

	testutil.CheckStatusCodeOK(testEnv.Context, t, testEnv.HTTPClient, url)

	// Verify metrics content.
	url = fmt.Sprintf("%s/metrics", testEnv.BaseURL)

	metricsRaw := string(testutil.CheckStatusCodeOK(testEnv.Context, t, testEnv.HTTPClient, url))

	expectedMetrics := []string{
		// Generic API server metric.
		"apiserver_request_filter_duration_seconds_bucket",
		// Adapter-specific metric.
		"newrelic_adapter_external_provider_queries_total",
	}

	for _, expectedMetric := range expectedMetrics {
		if !strings.Contains(metricsRaw, expectedMetric) {
			t.Fatalf("Expected metric %q not found in adapter metrics:\n%s", expectedMetric, metricsRaw)
		}
	}
}
