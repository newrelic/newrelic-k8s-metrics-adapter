// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"context"
	"fmt"
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
	testQuantity      = "123m"
)

//nolint:funlen,gocognit,cyclop // Just a large test suite.
func Test_Getting_external_metric(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	t.Run("returning_no_error", func(t *testing.T) {
		t.Parallel()

		t.Run("and_running_a_query_with", func(t *testing.T) {
			t.Parallel()

			t.Run("same_metric_and_selectors_in_cache_TTL_windows_uses_cache", func(t *testing.T) {
				t.Parallel()

				p, numCalls := testProvider(t)

				_, err := p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				_, err = p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				if *numCalls != 1 {
					t.Errorf("Expected exactly 1 call to backend, got %d", *numCalls)
				}
			})

			t.Run("same_metric_and_selectors_after_cache_TTL_calls_twice_the_backend", func(t *testing.T) {
				t.Parallel()

				mockProvider, numCalls := testMockProvider(t, resource.MustParse(testQuantity), time.Now(), nil)

				p, err := cache.NewCacheProvider(cache.ProviderOptions{ExternalProvider: mockProvider, CacheTTL: 1})
				if err != nil {
					t.Fatalf("Unexpected error creating the provider %v", err)
				}

				_, err = p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				time.Sleep(2 * time.Second)

				_, err = p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				if *numCalls != 2 {
					t.Errorf("Expected exactly 2 calls to backend, got %d", *numCalls)
				}
			})

			t.Run("different_selectors_calls_twice_the_backend", func(t *testing.T) {
				t.Parallel()

				p, numCalls := testProvider(t)

				_, err := p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.Exists, []string{})

				_, err = p.GetExternalMetric(ctx, "", s.Add(*r1), provider.ExternalMetricInfo{Metric: testMetricNameOne})
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				if *numCalls != 2 {
					t.Errorf("Expected exactly 2 calls to backend, got %d", *numCalls)
				}
			})

			t.Run("different_metrics_calls_twice_the_backend", func(t *testing.T) {
				t.Parallel()

				p, numCalls := testProvider(t)

				_, err := p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				_, err = p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameTwo})
				if err != nil {
					t.Fatalf("Unexpected error while getting external metric: %v", err)
				}

				if *numCalls != 2 {
					t.Errorf("Expected exactly 2 calls to backend, got %d", *numCalls)
				}
			})
		})
	})

	t.Run("returning_error", func(t *testing.T) {
		t.Parallel()

		expectedError := fmt.Errorf("randomError")
		mockProvider, numCalls := testMockProvider(t, resource.MustParse(testQuantity), time.Now(), expectedError)

		p, err := cache.NewCacheProvider(cache.ProviderOptions{ExternalProvider: mockProvider})
		if err != nil {
			t.Fatalf("Unexpected error creating the provider %v", err)
		}

		_, err = p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
		if err == nil {
			t.Fatalf("Error expected")
		}

		t.Run("and_running_a_query_with_same_metric_and_selectors_calls_twice_the_backend", func(t *testing.T) {
			t.Parallel()

			_, err = p.GetExternalMetric(ctx, "", nil, provider.ExternalMetricInfo{Metric: testMetricNameOne})
			if err == nil {
				t.Fatalf("Error expected")
			}

			if *numCalls != 2 {
				t.Errorf("Expected exactly 2 calls to backend, got %d", *numCalls)
			}
		})
	})
}

func Test_Listing_available_metrics_returns_all_configured_metrics_by_external_provider(t *testing.T) {
	t.Parallel()

	mockProvider, _ := testMockProvider(t, resource.MustParse(testQuantity), time.Now(), nil)

	p, err := cache.NewCacheProvider(cache.ProviderOptions{ExternalProvider: mockProvider})
	if err != nil {
		t.Fatalf("Unexpected error creating the provider %v", err)
	}

	listCache := p.ListAllExternalMetrics()
	listExternal := mockProvider.ListAllExternalMetrics()

	if cmp.Diff(listCache, listExternal) != "" {
		t.Errorf("Expecting identical lists: %q", cmp.Diff(listCache, listExternal))
	}
}

func Test_Creating_provider_returns_error_when_TTL_is_negative(t *testing.T) {
	t.Parallel()

	options := cache.ProviderOptions{
		ExternalProvider: nil,
		CacheTTL:         -1,
	}

	p, err := cache.NewCacheProvider(options)
	if err == nil {
		t.Errorf("Expected error creating provider")
	}

	if p != nil {
		t.Errorf("Expected provider to be nil when error occurs, got %v", p)
	}
}

func testMockProvider(t *testing.T, q resource.Quantity, ts time.Time, errExpected error) (*mock.Provider, *int) {
	t.Helper()

	numCalls := 0

	mockProvider := mock.Provider{
		GetMethod: func(_ context.Context, _ string, _ labels.Selector, _ provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) { //nolint:lll // External interface requirement.
			numCalls++

			return &external_metrics.ExternalMetricValueList{
				Items: []external_metrics.ExternalMetricValue{
					{
						MetricName: "MockMetric",
						Timestamp:  metav1.NewTime(ts),
						Value:      q,
					},
				},
			}, errExpected
		},
	}

	return &mockProvider, &numCalls
}

func testProvider(t *testing.T) (provider.ExternalMetricsProvider, *int) {
	t.Helper()

	mockProvider, numCalls := testMockProvider(t, resource.MustParse(testQuantity), time.Now(), nil)
	cacheOption := cache.ProviderOptions{
		ExternalProvider: mockProvider,
	}

	p, err := cache.NewCacheProvider(cacheOption)
	if err != nil {
		t.Fatalf("Unexpected error creating the provider %v", err)
	}

	return p, numCalls
}
