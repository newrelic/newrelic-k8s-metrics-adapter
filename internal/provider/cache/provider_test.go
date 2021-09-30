// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
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

				p, nCalls := getTestCacheProvider(t)

				_, err := p.GetExternalMetric(ctx, "", td.selectorsFirstCall, td.metricNameFirstCall)
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				time.Sleep(td.secondsToSleep)

				v, err := p.GetExternalMetric(ctx, "", td.selectorsSecondCall, td.metricNameSecondCall)
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				if *nCalls != 2 {
					t.Errorf("Expected exactly 2 calls to backend, got %d", *nCalls)
				}

				if vs := v.Items[0].Value.String(); vs != "2" {
					t.Errorf("Expected '2', got %q", vs)
				}
			})
		}
	})

	t.Run("error_when_fetching_fresh_value_the_external_provider_returns", func(t *testing.T) {
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
					t.Fatalf("Unexpected error creating the provider %v", err)
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

				p, nCalls := getTestCacheProvider(t)

				_, err := p.GetExternalMetric(ctx, "", td.selectorsFirstCall, td.metricNameFirstCall)
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				for i := 0; i < 100; i++ {
					v, err := p.GetExternalMetric(ctx, "", td.selectorsSecondCall, td.metricNameSecondCall)
					if err != nil {
						t.Fatalf("Unexpected error while getting external metric: %v", err)
					}

					if *nCalls != 1 {
						t.Errorf("Expected exactly 1 call to backend, got %d", *nCalls)
					}

					if vs := v.Items[0].Value.String(); vs != "1" {
						t.Errorf("Expected a cache value, got %q", vs)
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
		t.Fatalf("Unexpected error creating the provider %v", err)
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

func Test_Creating_provider_returns_external_provider_when_TTL_is_negative(t *testing.T) {
	t.Parallel()

	td := getWorkingMockOptions()

	mockProvider := &mock.Provider{
		GetExternalMetricFunc: func(_ context.Context, _ string, _ labels.Selector, _ provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) { //nolint:lll // External interface requirement.
			return &external_metrics.ExternalMetricValueList{
				Items: td.sample,
			}, td.err
		},
	}

	p, err := cache.NewCacheProvider(cache.ProviderOptions{ExternalProvider: mockProvider, CacheTTLSeconds: -1})
	if err != nil {
		t.Fatalf("Unexpected error creating the provider %v", err)
	}

	if _, ok := p.(*mock.Provider); !ok {
		t.Errorf("Expected provider type *mock.Provider, got %q", reflect.TypeOf(p))
	}
}

func getTestCacheProvider(t *testing.T) (provider.ExternalMetricsProvider, *int) {
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

	p, err := cache.NewCacheProvider(cache.ProviderOptions{ExternalProvider: mockProvider, CacheTTLSeconds: 2})
	if err != nil {
		t.Fatalf("Unexpected error creating the provider %v", err)
	}

	return p, &numCalls
}

type testDataStruct struct {
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
	}
}

func getWorkingMockOptions() *testMockOptions {
	return &testMockOptions{
		sample: []external_metrics.ExternalMetricValue{
			{Timestamp: metav1.Now(), Value: resource.Quantity{}},
		},
	}
}
