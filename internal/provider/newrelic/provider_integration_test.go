// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package newrelic_test

import (
	"context"
	"os"
	"strconv"
	"testing"

	nrClient "github.com/newrelic/newrelic-client-go/newrelic"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/provider/newrelic"
)

//nolint:funlen
func Test_Get_External_metrics(t *testing.T) {
	t.Parallel()

	p := setupNewrelicProvider(t)

	t.Run("fails_when_a_query_returns", func(t *testing.T) {
		t.Parallel()

		cases := []string{
			"error",
			"not_found",
			"string",
			"multiple_floats",
		}

		for _, queryName := range cases {
			queryName := queryName

			t.Run(queryName, func(t *testing.T) {
				t.Parallel()

				m := provider.ExternalMetricInfo{Metric: queryName}

				r, err := p.GetExternalMetric(context.Background(), "", nil, m)
				if err == nil {
					t.Errorf("Error expected")
				}

				if r != nil {
					t.Errorf("Unexpected result: %v", r)
				}
			})
		}
	})

	t.Run("succeeds_when_query", func(t *testing.T) {
		t.Parallel()

		cases := map[string]func() testData{
			"with_no_selectors_returns_a_float": func() testData {
				return testData{
					metric:    "float",
					selectors: nil,
				}
			},
			"with_selectors_returns_a_float": func() testData {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.Exists, []string{})
				r2, _ := labels.NewRequirement("key2", selection.DoesNotExist, []string{})
				r3, _ := labels.NewRequirement("key3", selection.In, []string{"value", "1", "2"})
				r4, _ := labels.NewRequirement("key4", selection.NotIn, []string{"value2", "3"})

				return testData{
					metric:    "float",
					selectors: s.Add(*r1).Add(*r2).Add(*r3).Add(*r4),
				}
			},
		}

		for testCaseName, testDataf := range cases {
			testData := testDataf()

			t.Run(testCaseName, func(t *testing.T) {
				t.Parallel()

				m := provider.ExternalMetricInfo{Metric: testData.metric}

				r, err := p.GetExternalMetric(context.Background(), "", testData.selectors, m)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if len(r.Items) != 1 {
					t.Errorf("Expected exactly one item, got %d", len(r.Items))
				}

				expectedValue := "123m"
				if r.Items[0].Value.String() != expectedValue {
					t.Errorf("Expected value %q, got %q", expectedValue, r.Items[0].Value.String())
				}
			})
		}
	})
}

func setupNewrelicProvider(t *testing.T) provider.ExternalMetricsProvider {
	t.Helper()

	c, err := nrClient.New(nrClient.ConfigPersonalAPIKey(os.Getenv("NEWRELIC_API_KEY")))
	if err != nil {
		t.Fatalf("Unexpected error creating the client: %v", err)
	}

	accountID, err := strconv.ParseInt(os.Getenv("NEWRELIC_ACCOUNT_ID"), 10, 64)
	if err != nil {
		t.Fatalf("Unexpected error parsing accountID: %v", err)
	}

	providerOptions := newrelic.ProviderOptions{
		ExternalMetrics: map[string]newrelic.Metric{
			"float": {
				Query:            "select 0.123 from K8sClusterSample",
				AddClusterFilter: true,
			},
			"multiple_floats": {Query: "SELECT average(1),average(2) from K8sClusterSample"},
			"string":          {Query: "SELECT latest('casa') from K8sClusterSample"},
			"not_found":       {Query: "select notExisting from K8sClusterSample"},
			"error":           {Query: "@!#$%^%#&%"},
		},
		NRDBClient:  &c.Nrdb,
		ClusterName: "test",
		AccountID:   accountID,
	}

	p, err := newrelic.NewDirectProvider(providerOptions)
	if err != nil {
		t.Fatalf("Unexpected error creating the provider: %v", err)
	}

	return p
}

type testData struct {
	metric    string
	selectors labels.Selector
}
