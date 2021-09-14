// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package newrelic_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/newrelic/newrelic-client-go/pkg/nrdb"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	"github.com/gsanchezgavier/metrics-adapter/internal/provider/newrelic"
)

// nolint: funlen, unparam
func Test_query_fails_when_is_returned(t *testing.T) {
	t.Parallel()

	cases := map[string]func() (result *nrdb.NRDBResultContainer, err error){
		"everything_nil": func() (result *nrdb.NRDBResultContainer, err error) {
			return nil, nil
		},
		"error": func() (result *nrdb.NRDBResultContainer, err error) {
			return nil, fmt.Errorf("random error")
		},
		"no_sample": func() (result *nrdb.NRDBResultContainer, err error) {
			return &nrdb.NRDBResultContainer{
				Results: []nrdb.NRDBResult{},
			}, nil
		},
		"a_sample_with_no_data": func() (result *nrdb.NRDBResultContainer, err error) {
			return &nrdb.NRDBResultContainer{
				Results: []nrdb.NRDBResult{
					{},
				},
			}, nil
		},
		"a_sample_with_too_much_data": func() (result *nrdb.NRDBResultContainer, err error) {
			return &nrdb.NRDBResultContainer{
				Results: []nrdb.NRDBResult{
					{
						"one":   float64(1),
						"two":   float64(2),
						"three": float64(3),
					},
				},
			}, nil
		},
		"a_sample_with_data_too_old": func() (result *nrdb.NRDBResultContainer, err error) {
			return &nrdb.NRDBResultContainer{
				Results: []nrdb.NRDBResult{
					{
						"one":       float64(1),
						"timestamp": float64(time.Now().Add(time.Duration(-15)*time.Hour).UnixNano() / 1000000),
					},
				},
			}, nil
		},
		"a_sample_with_no_float_data": func() (result *nrdb.NRDBResultContainer, err error) {
			return &nrdb.NRDBResultContainer{
				Results: []nrdb.NRDBResult{
					{"one": 1},
				},
			}, nil
		},
	}

	for testCaseName, valuesF := range cases {
		valuesF := valuesF

		t.Run(testCaseName, func(t *testing.T) {
			t.Parallel()

			result, errToReturn := valuesF()
			client := fakeQuery{
				result: result,
				err:    errToReturn,
			}

			providerOptions := newrelic.ProviderOptions{
				MetricsSupported: map[string]newrelic.Metric{"test": {Query: "select test from testSample"}},
				NRDBClient:       &client,
				ClusterName:      "test",
				AccountID:        1,
			}

			p, err := newrelic.NewDirectProvider(providerOptions)
			if err != nil {
				t.Fatalf("We were not expecting an error creating the provider %v", err)
			}

			metricInfo := provider.ExternalMetricInfo{Metric: "test"}

			r, err := p.GetExternalMetric(context.Background(), "", nil, metricInfo)
			if err == nil {
				t.Errorf("We were expecting an error")
			}

			if r != nil {
				t.Errorf("We were not expecting a result: %v", r)
			}
		})
	}
}

func Test_list_available_metrics(t *testing.T) {
	t.Parallel()

	m := map[string]newrelic.Metric{
		"test":  {Query: "select test from testSample"},
		"test2": {Query: "select test from testSample"},
	}

	providerOptions := newrelic.ProviderOptions{
		MetricsSupported: m,
		NRDBClient:       &fakeQuery{},
		ClusterName:      "test",
		AccountID:        1,
	}

	p, err := newrelic.NewDirectProvider(providerOptions)
	if err != nil {
		t.Fatalf("We were not expecting an error creating the provider %v", err)
	}

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

func Test_query_fail_when_metric_is_not_supported(t *testing.T) {
	t.Parallel()

	providerOptions := newrelic.ProviderOptions{
		MetricsSupported: map[string]newrelic.Metric{"test": {Query: "select test from testSample"}},
		NRDBClient: &fakeQuery{
			result: &nrdb.NRDBResultContainer{
				Results: []nrdb.NRDBResult{
					{
						"one": float64(1),
					},
				},
			},
		},
		ClusterName: "test",
		AccountID:   1,
	}

	p, err := newrelic.NewDirectProvider(providerOptions)
	if err != nil {
		t.Fatalf("We were not expecting an error creating the provider %v", err)
	}

	metricInfo := provider.ExternalMetricInfo{Metric: "not_existing_metric"}

	r, err := p.GetExternalMetric(context.Background(), "", nil, metricInfo)
	if err == nil {
		t.Errorf("We were expecting an error")
	}

	if r != nil {
		t.Errorf("We were not expecting a result %v", r)
	}
}

func Test_query_succeeds_when(t *testing.T) {
	t.Parallel()

	cases := map[string]func() (result *nrdb.NRDBResultContainer){
		"a_sample_with_valid_timestamp_is_returned": func() (result *nrdb.NRDBResultContainer) {
			return &nrdb.NRDBResultContainer{
				Results: []nrdb.NRDBResult{
					{
						"one":       float64(0.015),
						"timestamp": float64(time.Now().UnixNano() / 1000000),
					},
				},
			}
		},
		"a_sample_with_no_timestamp_is_returned": func() (result *nrdb.NRDBResultContainer) {
			return &nrdb.NRDBResultContainer{
				Results: []nrdb.NRDBResult{
					{
						"one": float64(0.015),
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
				MetricsSupported: map[string]newrelic.Metric{"test": {Query: "select test from testSample"}},
				NRDBClient: &fakeQuery{
					result: valuesF(),
				},
				ClusterName: "test",
				AccountID:   1,
			}

			p, err := newrelic.NewDirectProvider(providerOptions)
			if err != nil {
				t.Fatalf("We were not expecting an error creating the provider %v", err)
			}

			metricInfo := provider.ExternalMetricInfo{Metric: "test"}

			r, err := p.GetExternalMetric(context.Background(), "", nil, metricInfo)
			if err != nil {
				t.Errorf("We were not expecting an error: %v", err)
			}

			if len(r.Items) != 1 {
				t.Errorf("we were expecting exactly one item, %d", len(r.Items))
			}

			if r.Items[0].Value.String() != "15m" {
				t.Errorf("we were expecting a different value: '%s'!=15m", r.Items[0].Value.String())
			}
		})
	}
}

func Test_contrastror_fails_when(t *testing.T) {
	t.Parallel()

	cases := map[string]newrelic.ProviderOptions{
		"cluster_name_is_empty": {
			NRDBClient: &fakeQuery{},
			AccountID:  1,
		},
		"account_id_is_zero": {
			NRDBClient:  &fakeQuery{},
			ClusterName: "test",
		},
		"client_is_null": {
			AccountID:   1,
			ClusterName: "test",
		},
	}

	for testCaseName, options := range cases {
		options := options

		t.Run(testCaseName, func(t *testing.T) {
			t.Parallel()

			p, err := newrelic.NewDirectProvider(options)
			if err != nil {
				t.Errorf("We were expecting an error")
			}

			if p != nil {
				t.Errorf("We were not expecting an provider %v", p)
			}
		})
	}
}
