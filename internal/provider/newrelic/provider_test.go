// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package newrelic_test

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/newrelic/newrelic-client-go/pkg/nrdb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/component-base/metrics"
	metricsTestutil "k8s.io/component-base/metrics/testutil"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/provider/newrelic"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/testutil"
)

const (
	testClusterName = "testCluster"
	testMetricName  = "test_metric"
	testQuery       = "select test from testSample limit 1"
)

//nolint:funlen,gocognit,cyclop // Just a large test suite.
func Test_Getting_external_metric(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	t.Run("without_error", func(t *testing.T) {
		testTimestampMilli := time.Now().UnixNano() / 1000000

		// NewRelic only supports millisecond precision, so we need to drop all nanoseconds, otherwise time comparison
		// will not work.
		testTimestamp := time.Unix(testTimestampMilli/1000, 0)

		providerOptions, client := testProviderOptions()
		client.response = &nrdb.NRDBResultContainer{
			Results: []nrdb.NRDBResult{
				{
					"one":       float64(0.015),
					"timestamp": float64(testTimestampMilli),
				},
			},
		}

		registry := metrics.NewKubeRegistry()
		providerOptions.RegisterFunc = registry.Register

		p := testProvider(t, providerOptions)

		r, err := p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricName})
		if err != nil {
			t.Fatalf("Unexpected error while getting external metric: %v", err)
		}

		t.Run("returns_exactly_one_metric", func(t *testing.T) {
			t.Parallel()

			if len(r.Items) != 1 {
				t.Fatalf("Expected exactly one item, got %d", len(r.Items))
			}

			metric := r.Items[0]

			t.Run("with_first_value_from_query_result", func(t *testing.T) {
				t.Parallel()

				expectedValue := "15m"

				if metric.Value.String() != expectedValue {
					t.Errorf("Expected value %q, got %q", expectedValue, metric.Value.String())
				}
			})

			t.Run("with_name_of_requested_metric", func(t *testing.T) {
				t.Parallel()

				if metric.MetricName != testMetricName {
					t.Fatalf("Expected metric name %q, got %q", testMetricName, metric.MetricName)
				}
			})

			t.Run("with_timestamp_from_query_result", func(t *testing.T) {
				t.Parallel()

				time := metav1.NewTime(testTimestamp)
				if !metric.Timestamp.Equal(&time) {
					t.Fatalf("Expected timestamp %v, got %v", testTimestamp, metric.Timestamp)
				}
			})
		})

		t.Run("increments_successful_queries_metric", func(t *testing.T) {
			t.Parallel()

			expectedMetric := bytes.NewBufferString(`
# HELP newrelic_adapter_external_provider_queries_total [ALPHA] Total number of queries to the NewRelic backend.
# TYPE newrelic_adapter_external_provider_queries_total counter
newrelic_adapter_external_provider_queries_total{result="ok"} 1
`)
			if err := metricsTestutil.GatherAndCompare(
				registry,
				expectedMetric,
				"newrelic_adapter_external_provider_queries_total",
			); err != nil {
				t.Fatalf("Unexpected error while gathering cache metrics: %v", err)
			}
		})
	})

	t.Run("filters_metrics_by_configured_cluster_name_when_remove_cluster_filter_is_false", func(t *testing.T) {
		t.Parallel()

		sl := labels.NewSelector()
		r1, _ := labels.NewRequirement("key", selection.Exists, []string{})
		sl = sl.Add(*r1)

		providerOptions, client := testProviderOptions()
		providerOptions.ExternalMetrics = map[string]newrelic.Metric{
			testMetricName: {
				Query: testQuery,
			},
		}

		p := testProvider(t, providerOptions)

		metricInfo := provider.ExternalMetricInfo{Metric: testMetricName}

		if _, err := p.GetExternalMetric(ctx, "", sl, metricInfo); err != nil {
			t.Fatalf("Unexpected error getting external metric: %v", err)
		}

		expectedQuery := "select test from testSample limit 1 where clusterName='testCluster' where `key` IS NOT NULL"

		if client.query != expectedQuery {
			t.Errorf("Expected query %q, got %q", expectedQuery, client.query)
		}
	})

	t.Run("with_label_selector", func(t *testing.T) {
		t.Parallel()

		cases := map[string]struct {
			selector      func() labels.Selector
			expectedQuery string
		}{
			"does_not_modify_metric_query_when_no_selector_is_received": {
				selector:      func() labels.Selector { return nil },
				expectedQuery: "select test from testSample limit 1",
			},
			"does_not_modify_metric_query_when_empty_selector_is_received": {
				selector:      labels.NewSelector,
				expectedQuery: "select test from testSample limit 1",
			},
			"adds_IN_selector_to_query_when_defined": {
				selector: func() labels.Selector {
					s := labels.NewSelector()
					r1, _ := labels.NewRequirement("key", selection.In, []string{"value", "15", "18"})

					return s.Add(*r1)
				},
				expectedQuery: "select test from testSample limit 1 where `key` IN (15, 18, 'value')",
			},
			"adds_NOT_IN_selector_to_query_when_defined": {
				selector: func() labels.Selector {
					s := labels.NewSelector()
					r1, _ := labels.NewRequirement("key", selection.NotIn, []string{"value", "16", "17"})

					return s.Add(*r1)
				},
				expectedQuery: "select test from testSample limit 1 where `key` NOT IN (16, 17, 'value')",
			},
			"adds_IS_NOT_NULL_to_query_when_defined": {
				selector: func() labels.Selector {
					s := labels.NewSelector()
					r1, _ := labels.NewRequirement("key1", selection.DoesNotExist, []string{})

					return s.Add(*r1)
				},
				expectedQuery: "select test from testSample limit 1 where `key1` IS NULL",
			},
			"adds_IS_NULL_to_query_when_defined": {
				selector: func() labels.Selector {
					s := labels.NewSelector()
					r1, _ := labels.NewRequirement("key", selection.Exists, []string{})

					return s.Add(*r1)
				},
				expectedQuery: "select test from testSample limit 1 where `key` IS NOT NULL",
			},
			"adds_EQUALS_string_value_to_query_when_defined": {
				selector: func() labels.Selector {
					s := labels.NewSelector()
					r1, _ := labels.NewRequirement("key", selection.Equals, []string{"value"})

					return s.Add(*r1)
				},
				expectedQuery: "select test from testSample limit 1 where `key` = 'value'",
			},
			"adds_EQUALS_number_value_to_query_when_defined": {
				selector: func() labels.Selector {
					s := labels.NewSelector()
					r1, _ := labels.NewRequirement("key", selection.Equals, []string{"1.5"})

					return s.Add(*r1)
				},
				expectedQuery: "select test from testSample limit 1 where `key` = 1.5",
			},
			"adds_all_defined_selectors_to_query": {
				selector: func() labels.Selector {
					s := labels.NewSelector()
					r1, _ := labels.NewRequirement("key", selection.Exists, []string{})
					r2, _ := labels.NewRequirement("key2", selection.DoesNotExist, []string{})
					r3, _ := labels.NewRequirement("key3", selection.In, []string{"value", "1", "2"})
					r4, _ := labels.NewRequirement("key4", selection.NotIn, []string{"value2", "3"})

					return s.Add(*r1).Add(*r2).Add(*r3).Add(*r4)
				},
				expectedQuery: "select test from testSample limit 1 where " +
					"`key` IS NOT NULL and `key2` IS NULL and " +
					"`key3` IN (1, 2, 'value') and `key4` NOT IN (3, 'value2')",
			},
		}

		for testCaseName, testData := range cases {
			testData := testData

			t.Run(testCaseName, func(t *testing.T) {
				t.Parallel()

				providerOptions, client := testProviderOptions()

				p := testProvider(t, providerOptions)

				metricInfo := provider.ExternalMetricInfo{Metric: testMetricName}

				if _, err := p.GetExternalMetric(ctx, "", testData.selector(), metricInfo); err != nil {
					t.Fatalf("Unexpected error getting external metric: %v", err)
				}

				if client.query != testData.expectedQuery {
					t.Errorf("Expected query %q, got %q", testData.expectedQuery, client.query)
				}
			})
		}
	})

	t.Run("returns_metric_without_error_when_query_returns", func(t *testing.T) {
		t.Parallel()

		cases := map[string]func() (result *nrdb.NRDBResultContainer){
			"a_single_sample_with_valid_timestamp": func() (result *nrdb.NRDBResultContainer) {
				return &nrdb.NRDBResultContainer{
					Results: []nrdb.NRDBResult{
						{
							"one":       float64(0.015),
							"timestamp": float64(time.Now().UnixNano() / 1000000),
						},
					},
				}
			},
			"a_single_sample_with_no_timestamp": func() (result *nrdb.NRDBResultContainer) {
				return &nrdb.NRDBResultContainer{
					Results: []nrdb.NRDBResult{
						{
							"one": float64(0.015),
						},
					},
				}
			},
			"a_single_sample_with_malformed_timestamp": func() (result *nrdb.NRDBResultContainer) {
				return &nrdb.NRDBResultContainer{
					Results: []nrdb.NRDBResult{
						{
							"one":       float64(0.015),
							"timestamp": 1,
						},
					},
				}
			},
		}

		for testCaseName, valuesF := range cases {
			valuesF := valuesF

			t.Run(testCaseName, func(t *testing.T) {
				t.Parallel()

				providerOptions := newrelic.ProviderOptions{
					ExternalMetrics: map[string]newrelic.Metric{testMetricName: {Query: testQuery}},
					NRDBClient: &testClient{
						response: valuesF(),
					},
					ClusterName: testClusterName,
					AccountID:   1,
				}

				p := testProvider(t, providerOptions)

				metricInfo := provider.ExternalMetricInfo{Metric: testMetricName}

				r, err := p.GetExternalMetric(ctx, "", nil, metricInfo)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if len(r.Items) != 1 {
					t.Errorf("Expected exactly one item, got %d", len(r.Items))
				}

				expectedValue := "15m"

				if r.Items[0].Value.String() != expectedValue {
					t.Errorf("Expected value %q, got %q", expectedValue, r.Items[0].Value.String())
				}
			})
		}
	})

	t.Run("returns_metric_with_current_timestamp_when_query_result_has_no_timestamp", func(t *testing.T) {
		t.Parallel()

		providerOptions, client := testProviderOptions()
		client.response = &nrdb.NRDBResultContainer{
			Results: []nrdb.NRDBResult{
				{
					"one": float64(0.015),
				},
			},
		}

		p := testProvider(t, providerOptions)

		r, err := p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricName})
		if err != nil {
			t.Fatalf("Unexpected error while getting external metric: %v", err)
		}

		if reflect.DeepEqual(r.Items[0].Timestamp, metav1.Time{}) {
			t.Fatalf("Expected metric timestamp to not be empty")
		}
	})

	t.Run("increments_failed_queries_metric_when_query_fails", func(t *testing.T) {
		t.Parallel()

		providerOptions, client := testProviderOptions()
		client.err = fmt.Errorf("new error")
		client.response = nil

		registry := metrics.NewKubeRegistry()
		providerOptions.RegisterFunc = registry.Register

		p := testProvider(t, providerOptions)

		if _, err := p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricName}); err == nil {
			t.Errorf("Expected error getting external metric")
		}

		expectedMetric := bytes.NewBufferString(`
# HELP newrelic_adapter_external_provider_queries_total [ALPHA] Total number of queries to the NewRelic backend.
# TYPE newrelic_adapter_external_provider_queries_total counter
newrelic_adapter_external_provider_queries_total{result="err"} 1
`)

		if err := metricsTestutil.GatherAndCompare(
			registry,
			expectedMetric,
			"newrelic_adapter_external_provider_queries_total",
		); err != nil {
			t.Fatalf("Unexpected error while gathering cache metrics: %v", err)
		}
	})

	t.Run("fails_when", func(t *testing.T) {
		t.Parallel()

		expectGetFails := func(
			t *testing.T,
			providerOptions newrelic.ProviderOptions,
			selector labels.Selector,
			info provider.ExternalMetricInfo,
		) {
			t.Helper()

			p := testProvider(t, providerOptions)

			r, err := p.GetExternalMetric(ctx, "", selector, info)
			if err == nil {
				t.Errorf("Expected error getting external metric")
			}

			if r != nil {
				t.Errorf("Expected result to be nil, got %v", r)
			}
		}

		t.Run("query_fails", func(t *testing.T) {
			t.Parallel()

			providerOptions, client := testProviderOptions()
			client.err = fmt.Errorf("new error")
			client.response = nil

			expectGetFails(t, providerOptions, nil, provider.ExternalMetricInfo{Metric: testMetricName})
		})

		t.Run("query_result", func(t *testing.T) {
			cases := map[string]*nrdb.NRDBResultContainer{
				"is_nil": nil,
				"has_no_samples": {
					Results: []nrdb.NRDBResult{},
				},
				"has_a_sample_with_no_data": {
					Results: []nrdb.NRDBResult{
						{},
					},
				},
				"has_more_than_one_result": {
					Results: []nrdb.NRDBResult{
						{
							"one": float64(1),
						},
						{
							"two": float64(2),
						},
					},
				},
				"has_a_sample_with_too_much_data": {
					Results: []nrdb.NRDBResult{
						{
							"one":   float64(1),
							"two":   float64(2),
							"three": float64(3),
						},
					},
				},
				"has_a_sample_with_data_too_old": {
					Results: []nrdb.NRDBResult{
						{
							"one": float64(1),
							// NewRelic returns timestamp in milliseconds, so get nanosecond precision, then raise to
							// milliseconds to avoid losing precision.
							"timestamp": float64(time.Now().Add(-time.Hour).UnixNano() / 1000000),
						},
					},
				},
				"has_a_sample_with_no_float_data": {
					Results: []nrdb.NRDBResult{
						{"one": 1},
					},
				},
			}

			for testCaseName, response := range cases {
				response := response

				t.Run(testCaseName, func(t *testing.T) {
					t.Parallel()

					providerOptions, client := testProviderOptions()
					client.response = response

					expectGetFails(t, providerOptions, nil, provider.ExternalMetricInfo{Metric: testMetricName})
				})
			}
		})

		t.Run("requested_metric_is_not_configured", func(t *testing.T) {
			t.Parallel()

			providerOptions, _ := testProviderOptions()

			expectGetFails(t, providerOptions, nil, provider.ExternalMetricInfo{Metric: "not_existing_metric"})
		})

		t.Run("metric_request_use_unsupported_operator_in_selector", func(t *testing.T) {
			t.Parallel()

			providerOptions, _ := testProviderOptions()

			s := labels.NewSelector()

			r1, err := labels.NewRequirement("key", selection.GreaterThan, []string{"1"})
			if err != nil {
				t.Fatalf("Unexpected error building requirement: %v", err)
			}

			expectGetFails(t, providerOptions, s.Add(*r1), provider.ExternalMetricInfo{Metric: testMetricName})
		})

		t.Run("requested_metric_has_uppercase_characters_in_name", func(t *testing.T) {
			t.Parallel()

			providerOptions, _ := testProviderOptions()

			expectGetFails(t, providerOptions, nil, provider.ExternalMetricInfo{Metric: "Test"})
		})
	})
}

func Test_Listing_available_metrics_returns_all_configured_metrics(t *testing.T) {
	t.Parallel()

	m := map[string]newrelic.Metric{
		"test":  {Query: testQuery},
		"test2": {Query: testQuery},
	}

	providerOptions, _ := testProviderOptions()
	providerOptions.ExternalMetrics = m

	p := testProvider(t, providerOptions)

	list := p.ListAllExternalMetrics()

	if len(list) != 2 {
		t.Errorf("Two elements in the list expected, got %d", len(list))
	}

	for _, l := range list {
		if _, ok := m[l.Metric]; !ok {
			t.Errorf("The metric %q was not intended to be supported", l.Metric)
		}
	}
}

func Test_Creating_provider_returns_error_when(t *testing.T) {
	t.Parallel()

	cases := map[string]func(o *newrelic.ProviderOptions){
		"account_id_is_zero": func(o *newrelic.ProviderOptions) { o.AccountID = 0 },
		"client_is_not_set":  func(o *newrelic.ProviderOptions) { o.NRDBClient = nil },
		"any_of_configured_external_metric_names_is_not_a_valid_path_segment": func(o *newrelic.ProviderOptions) {
			o.ExternalMetrics["test/"] = newrelic.Metric{}
		},
		"any_of_configured_external_metrics_has_uppercase_character_in_name": func(o *newrelic.ProviderOptions) {
			o.ExternalMetrics["Test"] = newrelic.Metric{}
		},
	}

	for testCaseName, mutateF := range cases {
		mutateF := mutateF

		t.Run(testCaseName, func(t *testing.T) {
			t.Parallel()

			providerOptions, _ := testProviderOptions()
			mutateF(&providerOptions)

			p, err := newrelic.NewDirectProvider(providerOptions)
			if err == nil {
				t.Errorf("Expected error creating provider")
			}

			if p != nil {
				t.Errorf("Expected provider to be nil when error occurs, got %v", p)
			}
		})
	}
}

type testClient struct {
	query    string
	response *nrdb.NRDBResultContainer
	err      error
}

func (r *testClient) QueryWithContext(_ context.Context, _ int, query nrdb.NRQL) (*nrdb.NRDBResultContainer, error) {
	r.query = string(query)

	return r.response, r.err
}

func testProvider(t *testing.T, options newrelic.ProviderOptions) provider.ExternalMetricsProvider {
	t.Helper()

	p, err := newrelic.NewDirectProvider(options)
	if err != nil {
		t.Fatalf("Unexpected error creating the provider: %v", err)
	}

	return p
}

func testProviderOptions() (newrelic.ProviderOptions, *testClient) {
	client := &testClient{
		response: &nrdb.NRDBResultContainer{
			Results: []nrdb.NRDBResult{
				{
					"value":     float64(1),
					"timestamp": float64(time.Now().UnixNano() / 1000000),
				},
			},
		},
	}

	return newrelic.ProviderOptions{
		ExternalMetrics: map[string]newrelic.Metric{testMetricName: {Query: testQuery, RemoveClusterFilter: true}},
		NRDBClient:      client,
		AccountID:       1,
		ClusterName:     testClusterName,
	}, client
}
