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

//nolint:funlen // Just a large test suite.
func Test_Getting_external_metric_returns(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	t.Run("fresh_value_from_configured_external_provider_when", func(t *testing.T) {
		t.Parallel()

		t.Run("cache_for_requested_metric_has_expired", func(t *testing.T) {
			t.Parallel()

			mockProvider, numCalls := getTestMockProvider(t, time.Now(), nil)

			p, err := cache.NewCacheProvider(cache.ProviderOptions{ExternalProvider: mockProvider, CacheTTLSeconds: 1})
			if err != nil {
				t.Fatalf("Unexpected error creating the provider %v", err)
			}

			_, err = p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
			if err != nil {
				t.Fatalf("Unexpected error while getting external metric: %v", err)
			}

			time.Sleep(2 * time.Second)

			testFreshValue(t, p, numCalls, nil, testMetricNameOne)
		})

		t.Run("requested_metric_value_is_not_in_cache", func(t *testing.T) {
			t.Parallel()

			p, numCalls := getTestCacheProvider(t)

			_, err := p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
			if err != nil {
				t.Fatalf("Unexpected error while getting external metric: %v", err)
			}

			testFreshValue(t, p, numCalls, nil, testMetricNameTwo)
		})

		t.Run("requested_metric_value_is_in_cache_for_different_selectors", func(t *testing.T) {
			t.Parallel()

			p, numCalls := getTestCacheProvider(t)

			_, err := p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
			if err != nil {
				t.Fatalf("Unexpected error while getting external metric: %v", err)
			}

			s := labels.NewSelector()
			r1, _ := labels.NewRequirement("key", selection.Exists, []string{})

			testFreshValue(t, p, numCalls, s.Add(*r1), testMetricNameOne)
		})
	})

	t.Run("error_when_fetching_fresh_value_fails", func(t *testing.T) {
		t.Parallel()

		expectedError := fmt.Errorf("randomError")
		mockProvider, _ := getTestMockProvider(t, time.Now(), expectedError)

		p, err := cache.NewCacheProvider(cache.ProviderOptions{ExternalProvider: mockProvider, CacheTTLSeconds: 5})
		if err != nil {
			t.Fatalf("Unexpected error creating the provider %v", err)
		}

		_, err = p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
		if err == nil {
			t.Fatalf("Error expected")
		}
	})

	t.Run("cached_value_when", func(t *testing.T) {
		t.Parallel()

		t.Run("same_metric_with_no_selector_is_requested_more_than_once_within_TTL_window", func(t *testing.T) {
			t.Parallel()
			p, nCalls := getTestCacheProvider(t)

			_, err := p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
			if err != nil {
				t.Fatalf("Unexpected error while getting external metric: %v", err)
			}

			testCache(t, p, nCalls, nil, testMetricNameOne)
		})

		t.Run("same_metric_with_same_selector_is_requested_more_than_once_within_TTL_window", func(t *testing.T) {
			t.Parallel()
			p, nCalls := getTestCacheProvider(t)
			s := labels.NewSelector()
			r1, _ := labels.NewRequirement("key", selection.Exists, []string{})
			sl := s.Add(*r1)

			_, err := p.GetExternalMetric(ctx, "", sl, provider.ExternalMetricInfo{Metric: testMetricNameOne})
			if err != nil {
				t.Fatalf("Unexpected error while getting external metric: %v", err)
			}

			testCache(t, p, nCalls, sl, testMetricNameOne)
		})
	})
}

func Test_Listing_available_external_metrics_always_gets_fresh_list_from_configured_external_provider(t *testing.T) {
	t.Parallel()

	mockProvider, numCalls := getTestMockProvider(t, time.Now(), nil)
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

		if *numCalls != i {
			t.Errorf("Expected %d calls to external provider, got %d", i, *numCalls)
		}
	}
}

func Test_Creating_provider_returns_external_provider_when_TTL_is_negative(t *testing.T) {
	t.Parallel()

	m, _ := getTestMockProvider(t, time.Now(), nil)

	p, err := cache.NewCacheProvider(cache.ProviderOptions{ExternalProvider: m, CacheTTLSeconds: -1})
	if err != nil {
		t.Fatalf("Unexpected error creating the provider %v", err)
	}

	if _, ok := p.(*mock.Provider); !ok {
		t.Errorf("Expected provider type *mock.Provider, got %q", reflect.TypeOf(p))
	}
}

func getTestMockProvider(t *testing.T, ts time.Time, errExpected error) (*mock.Provider, *int) {
	t.Helper()

	numCalls := 0

	mockProvider := mock.Provider{
		GetExternalMetricFunc: func(_ context.Context, _ string, _ labels.Selector, _ provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) { //nolint:lll // External interface requirement.
			numCalls++

			return &external_metrics.ExternalMetricValueList{
				Items: []external_metrics.ExternalMetricValue{
					{
						MetricName: "MockMetric",
						Timestamp:  metav1.NewTime(ts),
						Value:      resource.MustParse(fmt.Sprintf("%d", numCalls)),
					},
				},
			}, errExpected
		},
		ListAllExternalMetricsFunc: func() []provider.ExternalMetricInfo {
			numCalls++

			return []provider.ExternalMetricInfo{
				{
					Metric: "MockMetric",
				},
			}
		},
	}

	return &mockProvider, &numCalls
}

func getTestCacheProvider(t *testing.T) (provider.ExternalMetricsProvider, *int) {
	t.Helper()

	mockProvider, numCalls := getTestMockProvider(t, time.Now(), nil)

	p, err := cache.NewCacheProvider(cache.ProviderOptions{ExternalProvider: mockProvider, CacheTTLSeconds: 10})
	if err != nil {
		t.Fatalf("Unexpected error creating the provider %v", err)
	}

	return p, numCalls
}

func testFreshValue(t *testing.T, p provider.ExternalMetricsProvider, nCalls *int, sl labels.Selector, metric string) {
	t.Helper()

	ctx := testutil.ContextWithDeadline(t)

	v, err := p.GetExternalMetric(ctx, "", sl, provider.ExternalMetricInfo{Metric: metric})
	if err != nil {
		t.Fatalf("Unexpected error while getting external metric: %v", err)
	}

	if *nCalls != 2 {
		t.Errorf("Expected exactly 2 calls to backend, got %d", *nCalls)
	}

	if vs := v.Items[0].Value.String(); vs != "2" {
		t.Errorf("Expected '2', got %q", vs)
	}
}

func testCache(t *testing.T, p provider.ExternalMetricsProvider, nCalls *int, sl labels.Selector, metric string) {
	t.Helper()

	ctx := testutil.ContextWithDeadline(t)

	for i := 0; i < 100; i++ {
		v, err := p.GetExternalMetric(ctx, "", sl, provider.ExternalMetricInfo{Metric: metric})
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
}
