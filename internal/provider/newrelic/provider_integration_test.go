// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package newrelic_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/elazarl/goproxy"
	nrClient "github.com/newrelic/newrelic-client-go/newrelic"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/adapter"
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

//nolint:paralleltest // This test registers environment variables, so it must not be run in parallel.
func Test_Getting_external_metric_through_proxy(t *testing.T) {
	ctx := testutil.ContextWithDeadline(t)
	port := 1337

	t.Run("ends_with_error_when_proxy_fails", func(t *testing.T) {
		t.Setenv("HTTPS_PROXY", fmt.Sprintf("localhost:%d", port))

		runProxy(ctx, t, port, func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			return goproxy.RejectConnect, host
		})

		p := newrelicProviderWithMetric(t, newrelic.Metric{
			Query: testIntegrationQuery,
		}, &testutil.TestEnv{})

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
		}, &testutil.TestEnv{})

		m := provider.ExternalMetricInfo{Metric: testMetricName}

		if _, err := p.GetExternalMetric(ctx, "", nil, m); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}

// As The server must be run on port 3000 to be run locally as per the new relic client requirements, subtests
// cannot be run in parallel.
//
// For more details check https://github.com/newrelic/newrelic-client-go/blob/main/pkg/region/region_constants.go
//
//nolint:funlen,tparallel
func Test_does_not_hide_backend_errors(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	testEnv := testutil.TestEnv{
		StartKubernetes: true,
		Region:          "Local",
	}
	testEnv.Generate(t)

	p := newrelicProviderWithMetric(t, newrelic.Metric{
		Query: testIntegrationQuery,
	}, &testEnv)

	runAdapter(t, &testEnv, p)

	//nolint:paralleltest
	t.Run("when_response_error", func(t *testing.T) {
		cases := map[string]func(w http.ResponseWriter, r *http.Request){
			"is_200_with_empty_data": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			"is_400": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			"is_401": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			"is_403": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			"is_500": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		}

		for testCaseName, handlerFunc := range cases {
			handlerFunc := handlerFunc
			//nolint:paralleltest
			t.Run(testCaseName, func(t *testing.T) {
				runHTTPTestServer(t, 3000, handlerFunc)

				m := provider.ExternalMetricInfo{Metric: testMetricName}

				if _, err := p.GetExternalMetric(ctx, "", nil, m); err == nil {
					t.Error("Error expected")
				}

				// url reflecting the external metrics endpoint for the test_metrics so that later we can simulate the request a hpa
				// would perform to fetch metrics.
				url := fmt.Sprintf("%s/apis/external.metrics.k8s.io/v1beta1/namespaces/*/test_metric", testEnv.BaseURL)
				checkStatusCodeNotOK(ctx, t, &testEnv, url)
			})
		}
	})

	//nolint:paralleltest
	t.Run("when_returns_200_for_first_requests_and_then_500", func(t *testing.T) {
		server := runHTTPTestServer(t, 3000, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
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

		// Make the 3rd request to fail.
		for i := 0; i < 3; i++ {
			if i == 2 {
				server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
				})
				if _, err := p.GetExternalMetric(ctx, "", nil, m); err == nil {
					t.Error("Error expected")
				}

				return
			}

			if _, err := p.GetExternalMetric(ctx, "", nil, m); err != nil {
				t.Errorf("Unexpected error = %v", err)
			}
		}

		url := fmt.Sprintf("%s/apis/external.metrics.k8s.io/v1beta1/namespaces/*/test_metric", testEnv.BaseURL)
		checkStatusCodeNotOK(ctx, t, &testEnv, url)
	})
}

// checkStatusCodeNotOK sends a GET request to the given URL.
//
// If 401 or 403 return code is received, function will retry.
//
// If a status code = 2xx is received, function will fail the given test.
func checkStatusCodeNotOK(ctx context.Context, t *testing.T, testEnv *testutil.TestEnv, url string) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		t.Fatalf("Creating request: %v", err)
	}

	if err := wait.PollImmediateUntilWithContext(testEnv.Context, 1*time.Second, func(context.Context) (bool, error) {
		resp, err := testEnv.HTTPClient.Do(req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				t.Fatalf("Test timed out: %v", err)
			}

			t.Logf("Fetching %s: %v", url, err)

			time.Sleep(1 * time.Second)

			return false, nil
		}

		defer func() {
			_ = resp.Body.Close()
		}()

		switch resp.StatusCode {
		// Generic API server does not wait for RequestHeaderAuthRequestController informers cache to be synchronized,
		// so until this is done, metrics adapter will be responding with HTTP 403, so we want to retry on that.
		case http.StatusForbidden, http.StatusUnauthorized:
			t.Logf("Got %d response code, expected %d: %v. Retrying.", resp.StatusCode, http.StatusOK, resp)

			return false, nil
		case http.StatusOK:
			t.Errorf("Got %d response code, expected != 2xx", resp.StatusCode)
		default:
		}

		return true, nil
	}); err != nil {
		t.Fatalf("Timed out waiting for k8s apiserver to respond: %v", err)
	}
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

	options := adapter.Options{
		ExternalMetricsProvider: provider,
		Args:                    testEnv.Flags,
	}

	adapter, err := adapter.NewAdapter(options)
	if err != nil {
		t.Fatalf("Creating adapter: %v", err)
	}

	go func() {
		if err := adapter.Run(testEnv.Context.Done()); err != nil {
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
