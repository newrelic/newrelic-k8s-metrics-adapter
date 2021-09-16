// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package newrelic_test

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
)

//nolint:funlen
func Test_query_builder_with(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		selector      func() labels.Selector
		expectedQuery string
	}{
		"no_selectors": {
			selector:      func() labels.Selector { return nil },
			expectedQuery: "select test from testSample limit 1",
		},
		"empty_selector": {
			selector:      labels.NewSelector,
			expectedQuery: "select test from testSample limit 1",
		},
		"equal_selector": {
			selector: func() labels.Selector {
				s := labels.NewSelector()
				r, _ := labels.NewRequirement("key", selection.Equals, []string{"value"})

				return s.Add(*r)
			},
			expectedQuery: "select test from testSample where key = 'value' limit 1",
		},
		"double_selector": {
			selector: func() labels.Selector {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.Equals, []string{"value"})
				r2, _ := labels.NewRequirement("key2", selection.Equals, []string{"value"})

				return s.Add(*r1).Add(*r2)
			},
			expectedQuery: "select test from testSample where key = 'value' and key2 = 'value' limit 1",
		},
		"in_selector": {
			selector: func() labels.Selector {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.In, []string{"value", "15", "18"})
				r2, _ := labels.NewRequirement("key2", selection.NotIn, []string{"value2", "16"})

				return s.Add(*r1).Add(*r2)
			},
			expectedQuery: "select test from testSample where key IN (15, 18, 'value') and key2 NOT IN (16, 'value2') limit 1",
		},
		"exist_selector": {
			selector: func() labels.Selector {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.Exists, []string{})
				r2, _ := labels.NewRequirement("key2", selection.DoesNotExist, []string{})

				return s.Add(*r1).Add(*r2)
			},
			expectedQuery: "select test from testSample where key IS NOT NULL and key2 IS NULL limit 1",
		},
		"multiple_mixed": {
			selector: func() labels.Selector {
				s := labels.NewSelector()
				r1, _ := labels.NewRequirement("key", selection.Exists, []string{})
				r2, _ := labels.NewRequirement("key2", selection.DoesNotExist, []string{})
				r3, _ := labels.NewRequirement("key3", selection.In, []string{"value", "1", "2"})
				r4, _ := labels.NewRequirement("key4", selection.NotIn, []string{"value2", "3"})
				r5, _ := labels.NewRequirement("key5", selection.GreaterThan, []string{"4"})
				r6, _ := labels.NewRequirement("key6", selection.NotEquals, []string{"1234.1234"})

				return s.Add(*r1).Add(*r2).Add(*r3).Add(*r4).Add(*r5).Add(*r6)
			},
			expectedQuery: "select test from testSample where " +
				"key IS NOT NULL and key2 IS NULL and " +
				"key3 IN (1, 2, 'value') and key4 NOT IN (3, 'value2') " +
				"and key5 > 4 and key6 != 1234.1234 limit 1",
		},
	}

	for testCaseName, testData := range cases {
		testData := testData

		t.Run(testCaseName, func(t *testing.T) {
			t.Parallel()

			providerOptions, client := testProviderOptions()

			p := testProvider(t, providerOptions)

			metricInfo := provider.ExternalMetricInfo{Metric: testMetricName}

			if _, err := p.GetExternalMetric(context.Background(), "", testData.selector(), metricInfo); err != nil {
				t.Fatalf("Unexpected error getting external metric: %v", err)
			}

			if client.query != testData.expectedQuery {
				t.Errorf("Expected query %q, got %q", client.query, testData.expectedQuery)
			}
		})
	}
}
