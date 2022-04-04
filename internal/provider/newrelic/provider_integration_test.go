// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package newrelic_test

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"testing"

	"github.com/elazarl/goproxy"
	nrClient "github.com/newrelic/newrelic-client-go/newrelic"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

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

	t.Run("when_using_cluster_filter", func(t *testing.T) {
		t.Parallel()

		p := newrelicProviderWithMetric(t, newrelic.Metric{
			Query: testIntegrationQuery,
		})

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
		})

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
	t.Setenv("HTTPS_PROXY", fmt.Sprintf("localhost:%d", port))

	t.Run("ends_with_error_when_proxy_fails", func(t *testing.T) {
		proxy := runProxy(t, port, func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			return goproxy.MitmConnect, host
		})

		t.Cleanup(func() {
			if err := proxy.Shutdown(ctx); err != nil {
				t.Logf("Stopping proxy server: %v", err)
			}
		})

		p := newrelicProviderWithMetric(t, newrelic.Metric{
			Query: testIntegrationQuery,
		})

		m := provider.ExternalMetricInfo{Metric: testMetricName}

		if _, err := p.GetExternalMetric(ctx, "", nil, m); err == nil {
			t.Fatal("Error expected")
		}
	})

	t.Run("generates_a_query_successfully", func(t *testing.T) {
		proxy := runProxy(t, port, func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			return goproxy.OkConnect, host
		})

		t.Cleanup(func() {
			if err := proxy.Shutdown(ctx); err != nil {
				t.Logf("Stopping proxy server: %v", err)
			}
		})

		p := newrelicProviderWithMetric(t, newrelic.Metric{
			Query: testIntegrationQuery,
		})

		m := provider.ExternalMetricInfo{Metric: testMetricName}

		if _, err := p.GetExternalMetric(ctx, "", nil, m); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}

func newrelicProviderWithMetric(t *testing.T, metric newrelic.Metric) provider.ExternalMetricsProvider {
	t.Helper()

	testEnv := testutil.TestEnv{}
	testEnv.Generate(t)

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

func runProxy(t *testing.T, port int, f func(string, *goproxy.ProxyCtx) (*goproxy.ConnectAction, string)) *http.Server {
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

	return srv
}
