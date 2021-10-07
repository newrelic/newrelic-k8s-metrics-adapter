// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/component-base/metrics"
	metricsTestutil "k8s.io/component-base/metrics/testutil"
	"k8s.io/metrics/pkg/apis/external_metrics"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/provider/cache"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/provider/mock"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/testutil"
)

const (
	testMetricNameOne = "testMetricOne"
	testMetricNameTwo = "testMetricTwo"
)

//nolint:funlen,gocognit,cyclop // Just a large test suite.
func Test_Getting_external_metric_returns(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	t.Run("fresh_value_from_configured_external_provider_when", func(t *testing.T) {
		t.Parallel()

		cases := map[string]func(*testDataStruct){
			"configured_cache_TTL_is_zero": func(td *testDataStruct) {
				td.cacheTTLSeconds = 0
			},
			"configured_cache_TTL_is_negative": func(td *testDataStruct) {
				td.cacheTTLSeconds = -1
			},
			"cache_for_requested_metric_has_expired": func(td *testDataStruct) {
				td.secondsToSleep = 3 * time.Second
			},
			"requested_metric_value_is_not_in_cache": func(td *testDataStruct) {
				td.metricNameSecondCall = provider.ExternalMetricInfo{Metric: testMetricNameTwo}
			},
			"requested_metric_value_is_in_cache_for_different_selectors": func(td *testDataStruct) {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.Exists, []string{})
				sl := s.Add(*r1)
				td.selectorsSecondCall = sl
			},
		}

		for testCaseName, testData := range cases {
			td := getWorkingTestData()
			testData(td)

			t.Run(testCaseName, func(t *testing.T) {
				t.Parallel()

				p, nCalls, _ := getTestCacheProvider(t, td.cacheTTLSeconds)

				_, err := p.GetExternalMetric(ctx, "", td.selectorsFirstCall, td.metricNameFirstCall)
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				time.Sleep(td.secondsToSleep)

				v, err := p.GetExternalMetric(ctx, "", td.selectorsSecondCall, td.metricNameSecondCall)
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric after waiting: %v", err)
				}

				expectedCalls := 2
				if *nCalls != expectedCalls {
					t.Errorf("Expected exactly %d calls to backend, got %d", expectedCalls, *nCalls)
				}

				expectedValue := "2"
				if vs := v.Items[0].Value.String(); vs != expectedValue {
					t.Errorf("Expected %q, got %q", expectedValue, vs)
				}
			})
		}
	})

	t.Run("error_when_fetching_fresh_value_from_the_external_provider_returns", func(t *testing.T) {
		t.Parallel()

		cases := map[string]func(td *testMockOptions){
			"error": func(td *testMockOptions) {
				td.err = fmt.Errorf("random error")
			},
			"zero_metric_values": func(td *testMockOptions) {
				td.sample = []external_metrics.ExternalMetricValue{}
			},
			// Notice that currently the direct provider can return only one metric sample,
			// therefore, we take the timestamp from the first element of the ExternalMetricValue slice.
			// If in the future more samples are returned a different implementation for the timestamp is needed.
			"more_than_one_metric_values": func(td *testMockOptions) {
				td.sample = []external_metrics.ExternalMetricValue{
					{Timestamp: metav1.Now(), Value: resource.Quantity{}},
					{Timestamp: metav1.Now(), Value: resource.Quantity{}},
				}
			},
		}

		for testCaseName, testData := range cases {
			td := getWorkingMockOptions()
			testData(td)

			t.Run(testCaseName, func(t *testing.T) {
				t.Parallel()
				mockProvider := &mock.Provider{
					GetExternalMetricFunc: func(_ context.Context, _ string, _ labels.Selector, _ provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) { //nolint:lll // External interface requirement.
						return &external_metrics.ExternalMetricValueList{
							Items: td.sample,
						}, td.err
					},
				}

				p, err := cache.NewCacheProvider(cache.ProviderOptions{ExternalProvider: mockProvider, CacheTTLSeconds: 5})
				if err != nil {
					t.Fatalf("Unexpected error creating the provider: %v", err)
				}

				_, err = p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
				if err == nil {
					t.Fatalf("Error expected")
				}
			})
		}
	})

	t.Run("cached_value_when", func(t *testing.T) {
		t.Parallel()

		cases := map[string]func(*testDataStruct){
			"same_metric_with_no_selector_is_requested_more_than_once_within_TTL_window": func(td *testDataStruct) {
			},
			"same_metric_with_same_selector_is_requested_more_than_once_within_TTL_window": func(td *testDataStruct) {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.Exists, []string{})
				sl := s.Add(*r1)
				td.selectorsFirstCall = sl
				td.selectorsSecondCall = sl
			},
		}

		for testCaseName, testData := range cases {
			td := getWorkingTestData()
			testData(td)

			t.Run(testCaseName, func(t *testing.T) {
				t.Parallel()

				p, nCalls, _ := getTestCacheProvider(t, td.cacheTTLSeconds)

				_, err := p.GetExternalMetric(ctx, "", td.selectorsFirstCall, td.metricNameFirstCall)
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				for i := 0; i < 100; i++ {
					v, err := p.GetExternalMetric(ctx, "", td.selectorsSecondCall, td.metricNameSecondCall)
					if err != nil {
						t.Fatalf("Unexpected error while getting external metric: %v", err)
					}

					expectedCalls := 1
					if *nCalls != expectedCalls {
						t.Errorf("Expected exactly %d calls to backend, got %d", expectedCalls, *nCalls)
					}

					expectedValue := "1"
					if vs := v.Items[0].Value.String(); vs != "1" {
						t.Errorf("Expected %q, got %q", expectedValue, vs)
					}
				}
			})
		}
	})
}

func Test_Listing_available_external_metrics_always_returns_fresh_list_from_configured_external_provider(t *testing.T) {
	t.Parallel()

	numCalls := 0

	mockProvider := &mock.Provider{
		ListAllExternalMetricsFunc: func() []provider.ExternalMetricInfo {
			numCalls++

			return []provider.ExternalMetricInfo{
				{Metric: "MockMetric"},
			}
		},
	}

	listExternal := mockProvider.ListAllExternalMetrics()

	p, err := cache.NewCacheProvider(cache.ProviderOptions{ExternalProvider: mockProvider, CacheTTLSeconds: 5})
	if err != nil {
		t.Fatalf("Unexpected error creating the provider: %v", err)
	}

	for i := 2; i < 100; i++ {
		listCache := p.ListAllExternalMetrics()

		if diff := cmp.Diff(listCache, listExternal); diff != "" {
			t.Errorf("Expecting identical lists:\n%s", diff)
		}

		if numCalls != i {
			t.Errorf("Expected %d calls to external provider, got %d", i, numCalls)
		}
	}
}

func Test_Creating_provider_returns_external_provider_when_TTL_is(t *testing.T) {
	t.Parallel()

	for name, ttl := range map[string]int64{
		"negative": int64(-1),
		"zero":     int64(0),
	} {
		ttl := ttl

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockProvider, _, _ := getTestCacheProvider(t, ttl)

			p, err := cache.NewCacheProvider(cache.ProviderOptions{ExternalProvider: mockProvider, CacheTTLSeconds: ttl})
			if err != nil {
				t.Fatalf("Unexpected error creating the provider: %v", err)
			}

			if _, ok := p.(*mock.Provider); !ok {
				t.Errorf("Expected provider type *mock.Provider, got %q", reflect.TypeOf(p))
			}
		})
	}
}

func Test_Getting_fresh_external_metric_value_from_configured_external_provider_increments(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	p, _, registry := getTestCacheProvider(t, 1)

	if _, err := p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: "mock_metric"}); err != nil {
		t.Fatalf("Unexpected error while getting external metric: %v", err)
	}

	t.Run("cache_size_metric", func(t *testing.T) {
		t.Parallel()

		expectedMetric := bytes.NewBufferString(`
# HELP newrelic_adapter_external_provider_cache_size [ALPHA] Number of external metrics entries stored in the cache.
# TYPE newrelic_adapter_external_provider_cache_size gauge
newrelic_adapter_external_provider_cache_size 1
`)

		if err := metricsTestutil.GatherAndCompare(registry,
			expectedMetric,
			"newrelic_adapter_external_provider_cache_size",
		); err != nil {
			t.Fatalf("Unexpected error while gathering cache metrics: %v", err)
		}
	})

	t.Run("cache_miss_metric", func(t *testing.T) {
		t.Parallel()

		expectedMetric := bytes.NewBufferString(`
# HELP newrelic_adapter_external_provider_cache_requests_total [ALPHA] Total number of cache request.
# TYPE newrelic_adapter_external_provider_cache_requests_total counter
newrelic_adapter_external_provider_cache_requests_total{result="miss"} 1
`)

		if err := metricsTestutil.GatherAndCompare(
			registry,
			expectedMetric,
			"newrelic_adapter_external_provider_cache_requests_total",
		); err != nil {
			t.Fatalf("Unexpected error while gathering cache metrics: %v", err)
		}
	})
}

func Test_Getting_cached_external_metric_increments_cache_hit_metric(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	p, _, registry := getTestCacheProvider(t, 1)

	if _, err := p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: "mock_metric"}); err != nil {
		t.Fatalf("Unexpected error while getting external metric: %v", err)
	}

	if _, err := p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: "mock_metric"}); err != nil {
		t.Fatalf("Unexpected error while getting external metric: %v", err)
	}

	expectedMetric := bytes.NewBufferString(`
# HELP newrelic_adapter_external_provider_cache_requests_total [ALPHA] Total number of cache request.
# TYPE newrelic_adapter_external_provider_cache_requests_total counter
newrelic_adapter_external_provider_cache_requests_total{result="miss"} 1
newrelic_adapter_external_provider_cache_requests_total{result="hit"} 1
`)

	if err := metricsTestutil.GatherAndCompare(
		registry,
		expectedMetric,
		"newrelic_adapter_external_provider_cache_requests_total",
	); err != nil {
		t.Fatalf("Unexpected error while gathering cache metrics: %v", err)
	}
}

func Test_Creating_provider_returns_error_when_registering_metrics_fails(t *testing.T) {
	t.Parallel()

	expectedError := "foo"

	options := cache.ProviderOptions{
		ExternalProvider: &mock.Provider{},
		CacheTTLSeconds:  1,
		RegisterFunc: func(m metrics.Registerable) error {
			return fmt.Errorf(expectedError)
		},
	}

	_, err := cache.NewCacheProvider(options)
	if err == nil {
		t.Fatalf("Expected error creating the provider")
	}

	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("Expected error to contain %q, got %q", expectedError, err.Error())
	}
}

func getTestCacheProvider(t *testing.T, cacheTTL int64) (provider.ExternalMetricsProvider, *int, metrics.Gatherer) {
	t.Helper()

	numCalls := 0

	mockProvider := &mock.Provider{
		GetExternalMetricFunc: func(_ context.Context, _ string, _ labels.Selector, _ provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) { //nolint:lll // External interface requirement.
			numCalls++

			return &external_metrics.ExternalMetricValueList{
				Items: []external_metrics.ExternalMetricValue{
					{
						MetricName: "MockMetric",
						Timestamp:  metav1.NewTime(time.Now()),
						Value:      resource.MustParse(fmt.Sprintf("%d", numCalls)),
					},
				},
			}, nil
		},
	}

	registry := metrics.NewKubeRegistry()

	options := cache.ProviderOptions{
		ExternalProvider: mockProvider,
		CacheTTLSeconds:  cacheTTL,
		RegisterFunc:     registry.Register,
	}

	p, err := cache.NewCacheProvider(options)
	if err != nil {
		t.Fatalf("Unexpected error creating the provider: %v", err)
	}

	return p, &numCalls, registry
}

type testDataStruct struct {
	cacheTTLSeconds      int64
	secondsToSleep       time.Duration
	selectorsFirstCall   labels.Selector
	selectorsSecondCall  labels.Selector
	metricNameFirstCall  provider.ExternalMetricInfo
	metricNameSecondCall provider.ExternalMetricInfo
}

type testMockOptions struct {
	sample []external_metrics.ExternalMetricValue
	err    error
}

func getWorkingTestData() *testDataStruct {
	return &testDataStruct{
		metricNameFirstCall:  provider.ExternalMetricInfo{Metric: testMetricNameOne},
		metricNameSecondCall: provider.ExternalMetricInfo{Metric: testMetricNameOne},
		cacheTTLSeconds:      2,
	}
}

func getWorkingMockOptions() *testMockOptions {
	return &testMockOptions{
		sample: []external_metrics.ExternalMetricValue{
			{Timestamp: metav1.Now(), Value: resource.Quantity{}},
		},
	}
}
