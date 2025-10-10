// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package newrelic_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"

	"github.com/elazarl/goproxy"
	nrClient "github.com/newrelic/newrelic-client-go/v2/newrelic"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/component-base/metrics/legacyregistry"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/adapter"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/provider/cache"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/provider/newrelic"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/testutil"
)

const (
	testIntegrationQuery = "select 0.123 from NrUsage"
)

//nolint:funlen
func Test_Getting_external_metric_generates_a_query_not_rejected_by_backend(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	testEnv := testutil.TestEnv{}
	testEnv.Generate(t)

	t.Run("when_using_cluster_filter", func(t *testing.T) {
		t.Parallel()

		p := newrelicProviderWithMetric(t, newrelic.Metric{
			Query: testIntegrationQuery,
		}, &testEnv)

		m := provider.ExternalMetricInfo{Metric: testMetricName}

		if _, err := p.GetExternalMetric(ctx, "", nil, m); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})

	t.Run("when_using_label_selector", func(t *testing.T) {
		t.Parallel()

		p := newrelicProviderWithMetric(t, newrelic.Metric{
			Query:               testIntegrationQuery,
			RemoveClusterFilter: true,
		}, &testEnv)

		cases := map[string]func() labels.Selector{
			"with_no_selectors_defined": func() labels.Selector { return nil },
			"with_reserved_word_selector": func() labels.Selector {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("facet", selection.Equals, []string{"value"})

				return s.Add(*r1)
			},
			"with_special_char_selector": func() labels.Selector {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("foo.bar/baz", selection.Equals, []string{"value"})

				return s.Add(*r1)
			},
			"with_EQUAL_selector": func() labels.Selector {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.Equals, []string{"value"})

				return s.Add(*r1)
			},
			"with_IN_selector": func() labels.Selector {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.In, []string{"value", "15", "18"})

				return s.Add(*r1)
			},
			"with_NOT_IN_selector": func() labels.Selector {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.NotIn, []string{"value", "16", "17"})

				return s.Add(*r1)
			},
			"with_DOES_NOT_EXIST_selector": func() labels.Selector {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key1", selection.DoesNotExist, []string{})

				return s.Add(*r1)
			},
			"with_EXISTS_selector": func() labels.Selector {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.Exists, []string{})

				return s.Add(*r1)
			},
			"with_all_supported_selectors": func() labels.Selector {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.Exists, []string{})
				r2, _ := labels.NewRequirement("key2", selection.DoesNotExist, []string{})
				r3, _ := labels.NewRequirement("key3", selection.In, []string{"value", "1", "2"})
				r4, _ := labels.NewRequirement("key4", selection.NotIn, []string{"value2", "3"})
				r5, _ := labels.NewRequirement("key5", selection.Equals, []string{"equalVal"})

				return s.Add(*r1).Add(*r2).Add(*r3).Add(*r4).Add(*r5)
			},
		}

		for testCaseName, selector := range cases {
			selector := selector()

			t.Run(testCaseName, func(t *testing.T) {
				m := provider.ExternalMetricInfo{Metric: testMetricName}

				if _, err := p.GetExternalMetric(ctx, "", selector, m); err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			})
		}
	})
}

// This test uses the `Region: Local` which configures the newrelic go client to use the `localhost:3000` endpoint
// (https://github.com/newrelic/newrelic-client-go/blob/main/pkg/region/region_constants.go).
//
//nolint:funlen,tparallel,paralleltest // no parallel since port 3000 is used by all test.
func Test_does_not_hide_backend_errors(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	testEnv := testutil.TestEnv{
		StartKubernetes: true,
		Region:          "Local",
	}
	testEnv.Generate(t)

	url := fmt.Sprintf("%s/apis/external.metrics.k8s.io/v1beta1/namespaces/*/test_metric", testEnv.BaseURL)

	p := newrelicProviderWithMetric(t, newrelic.Metric{
		Query: testIntegrationQuery,
	}, &testEnv)

	runAdapter(t, &testEnv, p)

	t.Run("when_newrelic_backend_response_error_is", func(t *testing.T) {
		cases := map[string]func(w http.ResponseWriter, r *http.Request){
			"200_with_empty_data": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			"400": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			"401": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			"403": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			"500": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		}

		for testCaseName, handlerFunc := range cases {
			handlerFunc := handlerFunc
			t.Run(testCaseName, func(t *testing.T) {
				runHTTPTestServer(t, 3000, handlerFunc)

				m := provider.ExternalMetricInfo{Metric: testMetricName}

				if _, err := p.GetExternalMetric(ctx, "", nil, m); err == nil {
					t.Errorf("Unexpected error = %v", err)
				}

				testutil.RetryGetRequestAndCheckStatus(ctx, t, testEnv.HTTPClient, url, func(statusCode int) bool {
					return statusCode == http.StatusOK
				})
			})
		}
	})

	t.Run("when_newrelic_backend_response_error_200_for_first_requests_and_then_500", func(t *testing.T) {
		server := runHTTPTestServer(t, 3000, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": {
				"actor": {
				  "account": {
					"nrql": {
					  "results": [{"one": 0.015}]
					}
				  }
				}
			}}`))
		})

		m := provider.ExternalMetricInfo{Metric: testMetricName}

		// Perform 2 requests (totally arbitrary number) returning 200 status code.
		for i := 0; i < 2; i++ {
			if _, err := p.GetExternalMetric(ctx, "", nil, m); err != nil {
				t.Errorf("Unexpected error = %v", err)
			}

			testutil.RetryGetRequestAndCheckStatus(ctx, t, testEnv.HTTPClient, url, func(statusCode int) bool {
				return statusCode != http.StatusOK
			})
		}

		// Update handler to make the following requests to fail.
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
		})

		if _, err := p.GetExternalMetric(ctx, "", nil, m); err == nil {
			t.Errorf("Error expected")
		}

		testutil.RetryGetRequestAndCheckStatus(ctx, t, testEnv.HTTPClient, url, func(statusCode int) bool {
			return statusCode == http.StatusOK
		})
	})
}

//nolint:paralleltest // This test registers environment variables, so it must not be run in parallel.
func Test_Getting_external_metric_through_proxy(t *testing.T) {
	ctx := testutil.ContextWithDeadline(t)
	port := 1337

	testEnv := testutil.TestEnv{}
	testEnv.Generate(t)

	t.Run("ends_with_error_when_proxy_fails", func(t *testing.T) {
		t.Setenv("HTTPS_PROXY", fmt.Sprintf("localhost:%d", port))

		runProxy(ctx, t, port, func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			return goproxy.RejectConnect, host
		})

		p := newrelicProviderWithMetric(t, newrelic.Metric{
			Query: testIntegrationQuery,
		}, &testEnv)

		m := provider.ExternalMetricInfo{Metric: testMetricName}

		if _, err := p.GetExternalMetric(ctx, "", nil, m); err == nil {
			t.Fatal("Error expected")
		}
	})

	t.Run("generates_a_query_successfully", func(t *testing.T) {
		t.Setenv("HTTPS_PROXY", fmt.Sprintf("localhost:%d", port))

		runProxy(ctx, t, port, func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			return goproxy.OkConnect, host
		})

		p := newrelicProviderWithMetric(t, newrelic.Metric{
			Query: testIntegrationQuery,
		}, &testEnv)

		m := provider.ExternalMetricInfo{Metric: testMetricName}

		if _, err := p.GetExternalMetric(ctx, "", nil, m); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}

func runHTTPTestServer(t *testing.T, port int, f func(w http.ResponseWriter, req *http.Request)) *httptest.Server {
	t.Helper()

	l, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatalf("Unexpected error creating listener: %v", err)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(f))
	server.Listener = l
	server.Start()

	t.Cleanup(func() {
		_ = server.Listener.Close()
		server.Close()
	})

	return server
}

func runAdapter(t *testing.T, testEnv *testutil.TestEnv, provider provider.ExternalMetricsProvider) {
	t.Helper()

	cacheOptions := cache.ProviderOptions{
		ExternalProvider: provider,
		CacheTTLSeconds:  1,
		RegisterFunc:     legacyregistry.Register,
	}

	cacheProvider, err := cache.NewCacheProvider(cacheOptions)
	if err != nil {
		t.Fatalf("Unexpected error creating cache provider: %v", err)
	}

	options := adapter.Options{
		ExternalMetricsProvider: cacheProvider,
		Args:                    testEnv.Flags,
	}

	adapter, err := adapter.NewAdapter(options)
	if err != nil {
		t.Fatalf("Creating adapter: %v", err)
	}

	go func() {
		if err := adapter.Run(testEnv.Context); err != nil {
			t.Logf("Running operator: %v\n", err)
			t.Fail()
		}
	}()
}

//nolint:lll
func newrelicProviderWithMetric(t *testing.T, metric newrelic.Metric, testEnv *testutil.TestEnv) provider.ExternalMetricsProvider {
	t.Helper()

	clientOptions := []nrClient.ConfigOption{
		nrClient.ConfigPersonalAPIKey(testEnv.PersonalAPIKey),
		nrClient.ConfigRegion(testEnv.Region),
	}

	c, err := nrClient.New(clientOptions...)
	if err != nil {
		t.Fatalf("Unexpected error creating the client: %v", err)
	}

	accountIDRaw := testEnv.AccountID

	accountID, err := strconv.ParseInt(accountIDRaw, 10, 64)
	if err != nil {
		t.Fatalf("Unexpected error parsing accountID %q: %v", accountIDRaw, err)
	}

	providerOptions := newrelic.ProviderOptions{
		ExternalMetrics: map[string]newrelic.Metric{
			testMetricName: metric,
		},
		NRDBClient:  &c.Nrdb,
		ClusterName: testClusterName,
		AccountID:   accountID,
	}

	p, err := newrelic.NewDirectProvider(providerOptions)
	if err != nil {
		t.Fatalf("Unexpected error creating the provider: %v", err)
	}

	return p
}

//nolint:lll
func runProxy(ctx context.Context, t *testing.T, port int, f func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string)) {
	t.Helper()

	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile(".*"))).HandleConnectFunc(f)

	srv := &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", port),
		Handler: proxy,
	}

	go func() {
		_ = srv.ListenAndServe()
	}()

	t.Cleanup(func() {
		if err := srv.Shutdown(ctx); err != nil {
			t.Logf("Stopping proxy server: %v", err)
		}
	})
}
