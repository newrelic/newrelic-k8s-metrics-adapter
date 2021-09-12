// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package provider implements the external provider interface.
package provider_test

import (
	"context"
	"testing"
	"time"

	"github.com/newrelic/newrelic-client-go/pkg/nrdb"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	nrprovider "github.com/gsanchezgavier/metrics-adapter/internal/provider"
)

// nolint:funlen
func Test_query_builder_with(t *testing.T) {
	t.Parallel()

	cases := map[string]func() (match labels.Selector, queryResult string){
		"no_selectors": func() (labels.Selector, string) {
			return nil, "select test from testSample limit 1"
		},
		"empty_selector": func() (labels.Selector, string) {
			return labels.NewSelector(), "select test from testSample limit 1"
		},
		"equal_selector": func() (labels.Selector, string) {
			s := labels.NewSelector()
			r, _ := labels.NewRequirement("key", selection.Equals, []string{"value"})

			return s.Add(*r), "select test from testSample where key = 'value' limit 1"
		},
		"double_selector": func() (labels.Selector, string) {
			s := labels.NewSelector()
			r1, _ := labels.NewRequirement("key", selection.Equals, []string{"value"})
			r2, _ := labels.NewRequirement("key2", selection.Equals, []string{"value"})

			return s.Add(*r1).Add(*r2), "select test from testSample where key = 'value' and key2 = 'value' limit 1"
		},
		"in_selector": func() (labels.Selector, string) {
			s := labels.NewSelector()
			r1, _ := labels.NewRequirement("key", selection.In, []string{"value", "15", "18"})
			r2, _ := labels.NewRequirement("key2", selection.NotIn, []string{"value2", "16"})

			return s.Add(*r1).Add(*r2),
				"select test from testSample where key IN (15, 18, 'value') and key2 NOT IN (16, 'value2') limit 1"
		},
		"exist_selector": func() (labels.Selector, string) {
			s := labels.NewSelector()
			r1, _ := labels.NewRequirement("key", selection.Exists, []string{})
			r2, _ := labels.NewRequirement("key2", selection.DoesNotExist, []string{})

			return s.Add(*r1).Add(*r2), "select test from testSample where key IS NOT NULL and key2 IS NULL limit 1"
		},
		"multiple_mixed": func() (labels.Selector, string) {
			s := labels.NewSelector()
			r1, _ := labels.NewRequirement("key", selection.Exists, []string{})
			r2, _ := labels.NewRequirement("key2", selection.DoesNotExist, []string{})
			r3, _ := labels.NewRequirement("key3", selection.In, []string{"value", "1", "2"})
			r4, _ := labels.NewRequirement("key4", selection.NotIn, []string{"value2", "3"})
			r5, _ := labels.NewRequirement("key5", selection.GreaterThan, []string{"4"})
			r6, _ := labels.NewRequirement("key6", selection.NotEquals, []string{"1234.1234"})

			return s.Add(*r1).Add(*r2).Add(*r3).Add(*r4).Add(*r5).Add(*r6),
				"select test from testSample where " +
					"key IS NOT NULL and key2 IS NULL and " +
					"key3 IN (1, 2, 'value') and key4 NOT IN (3, 'value2') " +
					"and key5 > 4 and key6 != 1234.1234 limit 1"
		},
	}

	for testCaseName, labelsF := range cases {
		labelsF := labelsF

		t.Run(testCaseName, func(t *testing.T) {
			t.Parallel()

			sl, result := labelsF()
			client := fakeQuery{
				result: &nrdb.NRDBResultContainer{
					Results: []nrdb.NRDBResult{
						{
							"timestamp": time.Now(),
							"value":     float64(1),
						},
					},
				},
			}

			a := nrprovider.Provider{
				MetricsSupported: map[string]nrprovider.Metric{"test": {Query: "select test from testSample"}},
				NRDBClient:       &client,
				ClusterName:      "testCluster",
			}

			_, err := a.GetExternalMetric(context.Background(), "", sl, provider.ExternalMetricInfo{Metric: "test"})
			if err != nil {
				t.Fatal(err)
			}
			if client.query != result {
				t.Errorf("the query expected is different from the one returned '%s'!='%s'", client.query, result)
			}
		})
	}
}

func Test_query_adding_cluster_name_clause(t *testing.T) {
	t.Parallel()

	sl := labels.NewSelector()
	r1, _ := labels.NewRequirement("key", selection.Exists, []string{})
	sl = sl.Add(*r1)

	client := fakeQuery{
		result: &nrdb.NRDBResultContainer{
			Results: []nrdb.NRDBResult{
				{
					"timestamp": time.Now(),
					"value":     float64(1),
				},
			},
		},
	}

	a := nrprovider.Provider{
		MetricsSupported: map[string]nrprovider.Metric{"test": {
			Query:            "select test from testSample",
			AddClusterFilter: true,
		}},
		NRDBClient:  &client,
		ClusterName: "testCluster",
	}

	_, err := a.GetExternalMetric(context.Background(), "", sl, provider.ExternalMetricInfo{Metric: "test"})
	if err != nil {
		t.Fatal(err)
	}

	result := "select test from testSample where clusterName='testCluster' where key IS NOT NULL limit 1"
	if client.query != result {
		t.Errorf("the query expected is different from the one returned '%s'!='%s'", client.query, result)
	}
}

type fakeQuery struct {
	query  string
	result *nrdb.NRDBResultContainer
	err    error
}

func (r *fakeQuery) QueryWithContext(_ context.Context, _ int, query nrdb.NRQL) (*nrdb.NRDBResultContainer, error) {
	r.query = string(query)

	return r.result, r.err
}
