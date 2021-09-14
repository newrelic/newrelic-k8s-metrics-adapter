// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/newrelic/newrelic-client-go/newrelic"
	"gopkg.in/yaml.v3"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/gsanchezgavier/metrics-adapter/internal/adapter"
	"github.com/gsanchezgavier/metrics-adapter/internal/provider"
)

const configFileName = "/etc/newrelic/adapter/config.yaml"

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	klog.Infof("Starting NewRelic metrics adapter")

	config, err := loadConfiguration()
	if err != nil {
		klog.Fatalf("loading configuration: %v", err)
	}

	// The NEWRELIC_API_KEY is read from an envVar populated thanks to a k8s secret.
	c, err := newrelic.New(newrelic.ConfigPersonalAPIKey(os.Getenv("NEWRELIC_API_KEY")))
	if err != nil {
		klog.Fatalf("creating the client: %v", err)
	}

	externalProvider := provider.Provider{
		MetricsSupported: config.Metrics,
		NRDBClient:       &c.Nrdb,
		AccountID:        config.AccountID,
		ClusterName:      os.Getenv("CLUSTER_NAME"),
	}

	options := adapter.Options{
		Args:                    os.Args,
		ExternalMetricsProvider: &externalProvider,
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
		return nil, fmt.Errorf("reading config file %s: %w", configFileName, err)
	}

	config := configOptions{}

	err = yaml.Unmarshal(b, &config)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return &config, nil
}

type configOptions struct {
	//nolint:tagliatelle
	AccountID int64                      `yaml:"accountID"`
	Metrics   map[string]provider.Metric `yaml:"metrics"`
}
