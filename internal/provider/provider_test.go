// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package provider implements the external provider interface.
package provider_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/newrelic/newrelic-client-go/pkg/nrdb"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	nrprovider "github.com/gsanchezgavier/metrics-adapter/internal/provider"
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

			p := nrprovider.Provider{
				MetricsSupported: map[string]nrprovider.Metric{"test": {Query: "select test from testSample"}},
				NRDBClient:       &client,
			}

			r, err := p.GetValueDirectly(context.Background(), "test", nil)
			if err == nil {
				t.Errorf("we were expecting an error")
			}

			if r != 0 {
				t.Errorf("we were not expecting a result: %v", r)
			}
		})
	}
}

func Test_list_available_metrics(t *testing.T) {
	t.Parallel()

	m := map[string]nrprovider.Metric{
		"test":  {Query: "select test from testSample"},
		"test2": {Query: "select test from testSample"},
	}
	p := nrprovider.Provider{
		MetricsSupported: m,
		NRDBClient:       &fakeQuery{},
	}
	list := p.ListAllExternalMetrics()

	if len(list) != 2 {
		t.Errorf("two elements in the list expected")
	}

	for _, l := range list {
		if _, ok := m[l.Metric]; !ok {
			t.Errorf("the metric was not intended to be supported")
		}
	}
}

func Test_query_fail_when_metric_is_not_supported(t *testing.T) {
	t.Parallel()

	p := nrprovider.Provider{
		MetricsSupported: map[string]nrprovider.Metric{"test": {Query: "select test from testSample"}},
		NRDBClient: &fakeQuery{
			result: &nrdb.NRDBResultContainer{
				Results: []nrdb.NRDBResult{
					{
						"one": float64(1),
					},
				},
			},
		},
	}

	r, err := p.GetExternalMetric(context.Background(), "", nil,
		provider.ExternalMetricInfo{Metric: "not_existing_metric"})
	if err == nil {
		t.Errorf("we were expecting an error")
	}

	if r != nil {
		t.Errorf("we were not expecting a result %v", r)
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
						"timestamp": float64(time.Now().Add(time.Duration(-15)*time.Second).UnixNano() / 1000000),
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

			result := valuesF()
			client := fakeQuery{
				result: result,
				err:    nil,
			}

			p := nrprovider.Provider{
				MetricsSupported: map[string]nrprovider.Metric{"test": {Query: "select test from testSample"}},
				NRDBClient:       &client,
			}

			r, err := p.GetValueDirectly(context.Background(), "test", nil)
			if err != nil {
				t.Errorf("we were not expecting an error: %v", err)
			}

			if r == 0 {
				t.Fatal("we were expecting a result != 0")
			}
		})
	}
}
