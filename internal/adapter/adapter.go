// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package adapter exports top-level adapter logic.
package adapter

import (
	"flag"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/apiserver"
	basecmd "sigs.k8s.io/custom-metrics-apiserver/pkg/cmd"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	generatedopenapi "github.com/newrelic/newrelic-k8s-metrics-adapter/internal/generated/openapi"
)

// Name of the adapter.
const Name = "newrelic-k8s-metrics-adapter"

var version = "dev" //nolint:gochecknoglobal // Version is set at building time.

// Options holds the configuration for the adapter.
type Options struct {
	Args                    []string
	ExternalMetricsProvider provider.ExternalMetricsProvider
}

type adapter struct {
	basecmd.AdapterBase
}

// Adapter represents adapter functionality.
type Adapter interface {
	Run(<-chan struct{}) error
}

// NewAdapter validates given adapter options and creates new runnable adapter instance.
func NewAdapter(options Options) (Adapter, error) {
	adapter := &adapter{}
	// Used as identifier in logs with -v=6, defaults to "custom-metrics-adapter", so we want to override that.
	adapter.Name = Name

	adapter.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(
		generatedopenapi.GetOpenAPIDefinitions,
		openapinamer.NewDefinitionNamer(apiserver.Scheme),
	)
	adapter.OpenAPIConfig.Info.Title = adapter.Name
	adapter.OpenAPIConfig.Info.Version = version

	if err := adapter.initFlags(options.Args); err != nil {
		return nil, fmt.Errorf("initiating flags: %w", err)
	}

	if options.ExternalMetricsProvider == nil {
		return nil, fmt.Errorf("external metrics provider must be configured")
	}

	adapter.WithExternalMetrics(options.ExternalMetricsProvider)

	return adapter, nil
}

func (a *adapter) initFlags(args []string) error {
	a.FlagSet = pflag.NewFlagSet(Name, pflag.ContinueOnError)

	// Add flags from klog to be able to control log level etc.
	klogFlagSet := &flag.FlagSet{}
	klog.InitFlags(klogFlagSet)
	a.FlagSet.AddGoFlagSet(klogFlagSet)

	if err := a.Flags().Parse(args); err != nil {
		return fmt.Errorf("parsing flags %q: %w", strings.Join(args, ","), err)
	}

	return nil
}
