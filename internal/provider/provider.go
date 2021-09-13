// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package provider implements the external provider interface.
package provider

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/newrelic/newrelic-client-go/pkg/nrdb"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/external_metrics"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
)

// This constant represent the default value for the oldest sampled allowed expressed in seconds.
const defaultOldestSampleAllowed = 360

// NewRelic express timestamp in millisecond, whereas the library supports seconds and nanoseconds.
const newrelicTimestampFactor = 1000

// Provider holds the config of the provider.
type Provider struct {
	MetricsSupported map[string]Metric
	NRDBClient       NRDBClient
	AccountID        int64
	ClusterName      string
}

// Metric is the representation of the data needed to retrieve a metric.
type Metric struct {
	Query               string `yaml:"query"`
	AddClusterFilter    bool   `yaml:"addClusterFilter"`
	OldestSampleAllowed int64  `yaml:"oldestSampleAllowed"`
}

// NRDBClient is the interface a client should respect to be used in the provider.
type NRDBClient interface {
	QueryWithContext(ctx context.Context, accountID int, query nrdb.NRQL) (*nrdb.NRDBResultContainer, error)
}

// GetExternalMetric implemented from external provider interface.
func (p *Provider) GetExternalMetric(ctx context.Context, _ string, match labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) { //nolint:lll // External interface requirement.
	value, err := p.GetValueDirectly(ctx, info.Metric, match)
	if err != nil {
		return nil, fmt.Errorf("error getting external metrics: '%w'", err)
	}

	quantity, err := resource.ParseQuantity(fmt.Sprintf("%f", value))
	if err != nil {
		return nil, fmt.Errorf("parsing quantity: '%w'", err)
	}

	return &external_metrics.ExternalMetricValueList{
		Items: []external_metrics.ExternalMetricValue{
			{
				MetricName:   "info.Metric",
				MetricLabels: map[string]string{},
				Timestamp:    metav1.Now(),
				Value:        quantity,
			},
		},
	}, nil
}

// ListAllExternalMetrics implemented from external provider interface.
func (p *Provider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	em := []provider.ExternalMetricInfo{}

	for k := range p.MetricsSupported {
		em = append(em, provider.ExternalMetricInfo{
			Metric: k,
		})
	}

	return em
}

// GetValueDirectly is a function allowing to fetch a value directly.
func (p *Provider) GetValueDirectly(ctx context.Context, metricName string, match labels.Selector) (float64, error) {
	metric, ok := p.MetricsSupported[metricName]
	if !ok {
		return 0, fmt.Errorf("metric not supported")
	}

	query := metric.Query
	if metric.AddClusterFilter {
		query = addClusterFilter(p.ClusterName, query)
	}

	if match != nil {
		query = addMatchFilter(match, query)
	}

	query = addLimit(query)

	answer, err := p.NRDBClient.QueryWithContext(ctx, int(p.AccountID), nrdb.NRQL(query))
	if err != nil {
		return 0, fmt.Errorf("executing query '%s' in account '%d', %w", query, p.AccountID, err)
	}

	err = p.validateAnswer(answer, metric.OldestSampleAllowed, query)
	if err != nil {
		return 0, fmt.Errorf("validating answer, '%w'", err)
	}

	f, err := p.extractReturnValue(answer, query)
	if err != nil {
		return 0, fmt.Errorf("extracting return value, '%w'", err)
	}

	return f, nil
}

func (p *Provider) extractReturnValue(answer *nrdb.NRDBResultContainer, query string) (float64, error) {
	var returnValue interface{}

	// depending on the function used in the NRQL query the map key has different values, es latest.cpu.used,
	// average.cpu.usage, therefore we need to range to get the single element in that map.
	for _, v := range answer.Results[0] {
		returnValue = v
	}

	f, ok := returnValue.(float64)
	if !ok {
		return 0, fmt.Errorf("query result '%v' is not a float64 '%s', %s", query, returnValue, reflect.TypeOf(returnValue))
	}

	return f, nil
}

func (p *Provider) validateAnswer(answer *nrdb.NRDBResultContainer, oldestSampleAllowed int64, query string) error {
	if answer == nil {
		return fmt.Errorf("no error present, but the answer is nil for query: '%s'", query)
	}

	if len(answer.Results) != 1 {
		return fmt.Errorf("the query '%s' did not return exactly 1 sample: %d", query, len(answer.Results))
	}

	err := p.validateTimestamp(answer.Results[0], oldestSampleAllowed, query)
	if err != nil {
		return fmt.Errorf("validating timestamp: %w", err)
	}

	// We expect 1 samples since in case there was a timestamp field we removed it.
	delete(answer.Results[0], "timestamp")

	if len(answer.Results[0]) != 1 {
		return fmt.Errorf("the sample returned by the query '%s'"+
			" does not contain exactly 1 field: %d", query, len(answer.Results[0]))
	}

	return nil
}

// If we are not able to parse the timestamp, or if it is not present we do not trigger an error.
func (p *Provider) validateTimestamp(result nrdb.NRDBResult, oldestSampleAllowed int64, query string) error {
	t, ok := result["timestamp"]
	if !ok {
		klog.Infof("The query '%s' returns samples without the timestamp "+
			"useful to validate the sample, possibly is due to latest function", query)

		return nil
	}

	tf, okCast := t.(float64)
	if !okCast {
		klog.Infof("The query '%s' returns samples without float64 timestamp "+
			"that would be useful to validate the sample", query)

		return nil
	}

	if oldestSampleAllowed == 0 {
		oldestSampleAllowed = defaultOldestSampleAllowed
	}

	timestamp := time.Unix(int64(tf/newrelicTimestampFactor), 0)
	validWindow := time.Duration(oldestSampleAllowed) * time.Second
	oldestSample := time.Now().Add(-validWindow)

	if !timestamp.After(oldestSample) {
		return fmt.Errorf("the query returned a timestamp too old: '%s' %s<%s",
			query, timestamp.String(), oldestSample.String())
	}

	return nil
}
