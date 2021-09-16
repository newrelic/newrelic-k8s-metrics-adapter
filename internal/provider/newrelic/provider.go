// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package newrelic implements the external provider interface retrieving the data directly from the backend.
package newrelic

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

// defaultOldestSampleAllowed is the default value for the oldest sampled allowed (seconds).
const defaultOldestSampleAllowed = 360

// NewRelic reports timestamp in millisecond, whereas the library supports seconds and nanoseconds.
const newrelicTimestampFactor = 1000

type directProvider struct {
	metricsSupported map[string]Metric
	nrdbClient       NRDBClient
	accountID        int64
	clusterName      string
}

// ProviderOptions holds the configOptions of the provider.
type ProviderOptions struct {
	ExternalMetrics map[string]Metric
	NRDBClient      NRDBClient
	AccountID       int64
	ClusterName     string
}

// NewDirectProvider is the constructor for the direct provider.
func NewDirectProvider(options ProviderOptions) (provider.ExternalMetricsProvider, error) {
	if options.AccountID == 0 {
		return nil, fmt.Errorf("building a directProvider the accountID cannot be 0")
	}

	if options.NRDBClient == nil {
		return nil, fmt.Errorf("building a directProvider NRDBClient cannot be nil")
	}

	if options.ClusterName == "" {
		return nil, fmt.Errorf("building a directProvider ClusterName cannot be an empty string")
	}

	klog.Infof("All queries will be executing for account %d", options.AccountID)

	return &directProvider{
		metricsSupported: options.ExternalMetrics,
		nrdbClient:       options.NRDBClient,
		accountID:        options.AccountID,
		clusterName:      options.ClusterName,
	}, nil
}

// Metric holds the config needed to retrieve a supported metric.
type Metric struct {
	Query               Query `json:"query"`
	AddClusterFilter    bool  `json:"addClusterFilter"`
	OldestSampleAllowed int64 `json:"oldestSampleAllowed"`
}

// NRDBClient is the interface a client should respect to be used in the provider to retrieve metrics.
type NRDBClient interface {
	QueryWithContext(ctx context.Context, accountID int, query nrdb.NRQL) (*nrdb.NRDBResultContainer, error)
}

// GetExternalMetric returns the requested metric.
func (p *directProvider) GetExternalMetric(ctx context.Context, _ string, match labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) { //nolint:lll // External interface requirement.
	value, timestamp, err := p.getValueDirectly(ctx, info.Metric, match)
	if err != nil {
		return nil, fmt.Errorf("getting metric value: %w", err)
	}

	valueToBeParsed := fmt.Sprintf("%f", value)

	quantity, err := resource.ParseQuantity(valueToBeParsed)
	if err != nil {
		return nil, fmt.Errorf("parsing quantity: %w", err)
	}

	t := metav1.Now()

	if timestamp != nil {
		t = metav1.NewTime(*timestamp)
	}

	return &external_metrics.ExternalMetricValueList{
		Items: []external_metrics.ExternalMetricValue{
			{
				MetricName:   info.Metric,
				MetricLabels: map[string]string{},
				Timestamp:    t,
				Value:        quantity,
			},
		},
	}, nil
}

// ListAllExternalMetrics returns the list of external metrics supported by this provider.
func (p *directProvider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	em := []provider.ExternalMetricInfo{}

	for k := range p.metricsSupported {
		em = append(em, provider.ExternalMetricInfo{
			Metric: k,
		})
	}

	return em
}

// getValueDirectly fetches a value of a metric calling QueryWithContext of NRDBClient.
func (p *directProvider) getValueDirectly(ctx context.Context, metricName string, sl labels.Selector) (float64, *time.Time, error) {
	metric, ok := p.metricsSupported[metricName]
	if !ok {
		return 0, nil, fmt.Errorf("metric %q not configured", metricName)
	}

	q := metric.Query
	query := q.addClusterFilter(p.clusterName, metric.AddClusterFilter).
		addMatchFilter(sl).
		addLimit()

	klog.Infof("Executing %q", query)

	// Define inline so it can be used only from a single place in code for consistency,
	// to avoid possibly adding query to error message twice.
	errWithQuery := func(format string, a ...interface{}) error {
		return fmt.Errorf("query %q: %w", query, fmt.Errorf(format, a...))
	}

	answer, err := p.nrdbClient.QueryWithContext(ctx, int(p.accountID), nrdb.NRQL(query))
	if err != nil {
		return 0, nil, errWithQuery("executing query: %w", err)
	}

	timestamp, err := p.validateAnswer(answer, metric.OldestSampleAllowed, query)
	if err != nil {
		return 0, nil, errWithQuery("validating result: %w", err)
	}

	f, err := p.extractReturnValue(answer)
	if err != nil {
		return 0, nil, errWithQuery("extracting return value: %w", err)
	}

	return f, timestamp, nil
}

func (p *directProvider) extractReturnValue(answer *nrdb.NRDBResultContainer) (float64, error) {
	// Depending on the function used in the NRQL query the map key has different values, es latest.cpu.used,
	// average.cpu.usage, therefore we need to range to get the single element in that map.
	var returnValue interface{}
	for _, v := range answer.Results[0] {
		returnValue = v
	}

	f, ok := returnValue.(float64)
	if !ok {
		return 0, fmt.Errorf("expected first value to be of type %q, got %q", "float64", reflect.TypeOf(returnValue))
	}

	return f, nil
}

func (p *directProvider) validateAnswer(answer *nrdb.NRDBResultContainer, oldestValidSample int64, query Query) (*time.Time, error) {
	if answer == nil {
		return nil, fmt.Errorf("no error present, but the answer is nil")
	}

	if len(answer.Results) != 1 {
		return nil, fmt.Errorf("expected exactly 1 sample, got %d", len(answer.Results))
	}

	nrdbResult := answer.Results[0]

	timestamp, err := p.validateTimestamp(nrdbResult, oldestValidSample, query)
	if err != nil {
		return nil, fmt.Errorf("validating timestamp: %w", err)
	}

	// We expect 1 samples since in case there was a timestamp field we removed it.
	delete(nrdbResult, "timestamp")

	if len(nrdbResult) != 1 {
		return nil, fmt.Errorf("expected sample to contain exactly 1 field, got %d", len(nrdbResult))
	}

	return timestamp, nil
}

// If we are not able to parse the timestamp, or if it is not present we do not trigger an error.
func (p *directProvider) validateTimestamp(nrdbResult nrdb.NRDBResult, oldestSampleAllowed int64, query Query) (*time.Time, error) {
	t, ok := nrdbResult["timestamp"]
	if !ok {
		klog.Infof("The query %q returns samples without the timestamp "+
			"useful to validate the sample, possibly is due to latest function", query)

		return nil, nil
	}

	tf, okCast := t.(float64)
	if !okCast {
		klog.Infof("The query %q returns samples with a 'no float64' timestamp", query)

		return nil, nil
	}

	if oldestSampleAllowed == 0 {
		oldestSampleAllowed = defaultOldestSampleAllowed
	}

	timestamp := time.Unix(int64(tf/newrelicTimestampFactor), 0)
	validWindow := time.Duration(oldestSampleAllowed) * time.Second
	oldestSample := time.Now().Add(-validWindow)

	if !timestamp.After(oldestSample) {
		return nil, fmt.Errorf("timestamp too old: %s<%s", timestamp.String(), oldestSample.String())
	}

	return &timestamp, nil
}
