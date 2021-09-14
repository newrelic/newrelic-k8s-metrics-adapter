// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io/ioutil"
	"os"

	nrClient "github.com/newrelic/newrelic-client-go/newrelic"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/yaml"

	"github.com/gsanchezgavier/metrics-adapter/internal/adapter"
	"github.com/gsanchezgavier/metrics-adapter/internal/provider/newrelic"
)

const configFileName = "/etc/newrelic/adapter/config.yaml"

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	klog.Infof("Starting NewRelic metrics adapter")

	config, err := loadConfiguration()
	if err != nil {
		klog.Fatalf("Loading configuration: %v", err)
	}

	// The NEWRELIC_API_KEY is read from an envVar populated thanks to a k8s secret.
	c, err := nrClient.New(nrClient.ConfigPersonalAPIKey(os.Getenv("NEWRELIC_API_KEY")))
	if err != nil {
		klog.Fatalf("Creating NewRelic client: %v", err)
	}

	providerOptions := newrelic.ProviderOptions{
		MetricsSupported: config.Metrics,
		NRDBClient:       &c.Nrdb,
		AccountID:        config.AccountID,
		ClusterName:      os.Getenv("CLUSTER_NAME"),
	}

	directProvider, err := newrelic.NewDirectProvider(providerOptions)
	if err != nil {
		klog.Fatalf("Creating direct provider: %v", err)
	}

	options := adapter.Options{
		Args:                    os.Args,
		ExternalMetricsProvider: directProvider,
	}

	a, err := adapter.NewAdapter(options)
	if err != nil {
		klog.Fatalf("Initializing adapter: %v", err)
	}

	if err := a.Run(signals.SetupSignalHandler().Done()); err != nil {
		klog.Fatalf("Running adapter: %v", err)
	}
}

func loadConfiguration() (*configOptions, error) {
	b, err := ioutil.ReadFile(configFileName)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", configFileName, err)
	}

	config := &configOptions{}
	if err = yaml.Unmarshal(b, config); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return config, nil
}

type configOptions struct {
	AccountID int64                      `json:"accountID"`
	Metrics   map[string]newrelic.Metric `json:"metrics"`
}
