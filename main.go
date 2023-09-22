// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	nrClient "github.com/newrelic/newrelic-client-go/newrelic"
	"github.com/newrelic/newrelic-client-go/pkg/nrdb"
	"github.com/newrelic/newrelic-client-go/pkg/region"
	"github.com/spf13/pflag"
	"k8s.io/component-base/logs"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
	"sigs.k8s.io/yaml"

	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/adapter"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/provider/cache"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/provider/newrelic"
)

const (
	// DefaultConfigPath is a path from where configuration will be read.
	DefaultConfigPath = "/etc/newrelic/adapter/config.yaml"

	// NewRelicAPIKeyEnv is an environment variable name which must be set with a valid NewRelic license key for
	// adapter to run.
	NewRelicAPIKeyEnv = "NEWRELIC_API_KEY"

	// ClusterNameEnv is an environment variable name which will be read for filtering cluster-scoped metrics.
	ClusterNameEnv = "CLUSTER_NAME"

	// NrdbClientMaxTimeoutSeconds is the maximum timeout that could be set to the nrdb client.
	NrdbClientMaxTimeoutSeconds = 120
)

// ConfigOptions represents supported configuration options for metric-adapter.
type ConfigOptions struct {
	AccountID                int64                      `json:"accountID"`
	ExternalMetrics          map[string]newrelic.Metric `json:"externalMetrics"`
	Region                   string                     `json:"region"`
	CacheTTLSeconds          int64                      `json:"cacheTTLSeconds"`
	NrdbClientTimeoutSeconds int                        `json:"nrdbClientTimeoutSeconds"`
}

// Run reads configuration file and environment variables to configure and run the adapter.
func Run(ctx context.Context, args []string) error {
	flagSet := pflag.NewFlagSet(adapter.Name, pflag.ContinueOnError)

	configPath := flagSet.String("config-file", DefaultConfigPath, "Path to read config file from")

	err := adapter.ParseFlags(args, flagSet, nil)
	if err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}

		return fmt.Errorf("parsing given flags: %w", err)
	}

	config, err := loadConfiguration(*configPath)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	if config.Region != "" {
		if _, err := region.Parse(config.Region); err != nil {
			return fmt.Errorf("parsing region %q: %w", config.Region, err)
		}
	}

	if config.NrdbClientTimeoutSeconds > NrdbClientMaxTimeoutSeconds {
		config.NrdbClientTimeoutSeconds = NrdbClientMaxTimeoutSeconds
	}
	clientOptions := []nrClient.ConfigOption{
		nrClient.ConfigPersonalAPIKey(os.Getenv(NewRelicAPIKeyEnv)),
		nrClient.ConfigRegion(config.Region),
		nrClient.ConfigHTTPTimeout(time.Duration(config.NrdbClientTimeoutSeconds) * time.Second),
	}

	// The NEWRELIC_API_KEY is read from an envVar populated thanks to a k8s secret.
	c, err := nrClient.New(clientOptions...)
	if err != nil {
		return fmt.Errorf("creating NewRelic client: %w", err)
	}

	externalMetricsProvider, err := externalMetricsProvider(config, &c.Nrdb)
	if err != nil {
		return fmt.Errorf("creating external metrics provider: %w", err)
	}

	options := adapter.Options{
		Args:                    args,
		ExtraFlags:              flagSet,
		ExternalMetricsProvider: externalMetricsProvider,
	}

	a, err := adapter.NewAdapter(options)
	if err != nil {
		return fmt.Errorf("initializing adapter: %w", err)
	}

	return a.Run(ctx.Done()) //nolint:wrapcheck // Don't wrap as otherwise error annotations will be duplicated.
}

func externalMetricsProvider(config *ConfigOptions, nrdb *nrdb.Nrdb) (provider.ExternalMetricsProvider, error) {
	providerOptions := newrelic.ProviderOptions{
		ExternalMetrics: config.ExternalMetrics,
		NRDBClient:      nrdb,
		AccountID:       config.AccountID,
		ClusterName:     os.Getenv(ClusterNameEnv),
		RegisterFunc:    legacyregistry.Register,
	}

	directProvider, err := newrelic.NewDirectProvider(providerOptions)
	if err != nil {
		return nil, fmt.Errorf("creating direct provider: %w", err)
	}

	cacheOptions := cache.ProviderOptions{
		ExternalProvider: directProvider,
		CacheTTLSeconds:  config.CacheTTLSeconds,
		RegisterFunc:     legacyregistry.Register,
	}

	cacheProvider, err := cache.NewCacheProvider(cacheOptions)
	if err != nil {
		return nil, fmt.Errorf("creating cache provider: %w", err)
	}

	return cacheProvider, nil
}

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	klog.Infof("Starting NewRelic metrics adapter")

	if err := Run(signals.SetupSignalHandler(), os.Args); err != nil {
		klog.Fatalf("Running adapter failed: %v", err)
	}
}

func loadConfiguration(configPath string) (*ConfigOptions, error) {
	b, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	config := &ConfigOptions{}
	if err = yaml.UnmarshalStrict(b, config); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return config, nil
}
