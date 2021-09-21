// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

// Package e2e_test implements e2e tests for metrics adapter, which are not related to any specific package.
//
// They also test Helm chart manifests to verify metrics reachability over Kubernetes API server.
package e2e_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"
	eclient "k8s.io/metrics/pkg/client/external_metrics"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	// This metric must be configured when deploying the adapter.
	testMetric = "e2e"

	// All external metrics are namespaced but we do not support namespace filtering at the moment, so this
	// value must be just an existing namespace in the cluster.
	testNamespace = "default"
)

func Test_Metrics_adapter_makes_sample_external_metric_available(t *testing.T) {
	t.Parallel()

	externalMetricsClient := testExternalMetricsClient(t)

	if _, err := externalMetricsClient.NamespacedMetrics(testNamespace).List(testMetric, labels.Everything()); err != nil {
		t.Fatalf("Unexpected error getting metric %q: %v", testMetric, err)
	}
}

func testExternalMetricsClient(t *testing.T) eclient.ExternalMetricsClient {
	t.Helper()

	testEnv := &envtest.Environment{
		// For e2e tests, we use envtest.Environment for consistency with integration tests,
		// but we force them to use existing cluster instead of creating local controlplane,
		// as cluster we test on must have created resources defined in the operator Helm chart.
		//
		// This also allows us to test if the Helm chart configuration is correct (e.g. RBAC rules).
		//
		// With that approach, e2e tests can also be executed against cluster with 'make tilt-up' running.
		//
		// In the future, we may support also optionally creating Helm chart release on behalf of the user.
		UseExistingCluster: pointer.BoolPtr(true),
	}

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("Starting test environment: %v", err)
	}

	t.Cleanup(func() {
		if err := testEnv.Stop(); err != nil {
			t.Logf("Stopping test environment: %v", err)
		}
	})

	externalMetricsClient, err := eclient.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("Creating metrics clientset: %v", err)
	}

	return externalMetricsClient
}
